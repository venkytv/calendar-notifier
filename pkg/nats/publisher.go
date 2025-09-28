package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/venkytv/calendar-notifier/internal/models"
)

// Publisher handles publishing calendar notifications to NATS
type Publisher struct {
	conn    *nats.Conn
	subject string
	logger  *slog.Logger
}

// Config holds NATS publisher configuration
type Config struct {
	URL             string        `yaml:"url"`
	Subject         string        `yaml:"subject"`
	ConnectTimeout  time.Duration `yaml:"connect_timeout"`
	ReconnectWait   time.Duration `yaml:"reconnect_wait"`
	MaxReconnects   int           `yaml:"max_reconnects"`
	PingInterval    time.Duration `yaml:"ping_interval"`
	MaxPingsOut     int           `yaml:"max_pings_out"`
	ReconnectBuffer int           `yaml:"reconnect_buffer"`
}

// DefaultConfig returns a default NATS configuration
func DefaultConfig() *Config {
	return &Config{
		URL:             "nats://localhost:4222",
		Subject:         "calendar.notifications",
		ConnectTimeout:  5 * time.Second,
		ReconnectWait:   2 * time.Second,
		MaxReconnects:   10,
		PingInterval:    2 * time.Minute,
		MaxPingsOut:     2,
		ReconnectBuffer: 5 * 1024 * 1024, // 5MB
	}
}

// NewPublisher creates a new NATS publisher with the given configuration
func NewPublisher(config *Config, logger *slog.Logger) (*Publisher, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if logger == nil {
		logger = slog.Default()
	}

	// Configure NATS connection options
	options := []nats.Option{
		nats.Timeout(config.ConnectTimeout),
		nats.ReconnectWait(config.ReconnectWait),
		nats.MaxReconnects(config.MaxReconnects),
		nats.PingInterval(config.PingInterval),
		nats.MaxPingsOutstanding(config.MaxPingsOut),
		nats.ReconnectBufSize(config.ReconnectBuffer),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			logger.Warn("NATS disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("NATS reconnected", "url", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			logger.Info("NATS connection closed")
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			logger.Error("NATS error", "error", err, "subject", sub.Subject)
		}),
	}

	// Connect to NATS
	conn, err := nats.Connect(config.URL, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS at %s: %v", config.URL, err)
	}

	publisher := &Publisher{
		conn:    conn,
		subject: config.Subject,
		logger:  logger,
	}

	logger.Info("NATS publisher initialized",
		"url", config.URL,
		"subject", config.Subject,
		"connected_url", conn.ConnectedUrl())

	return publisher, nil
}

// PublishNotification publishes a single calendar notification to NATS
func (p *Publisher) PublishNotification(ctx context.Context, notification *models.Notification) error {
	if p.conn == nil || p.conn.IsClosed() {
		return fmt.Errorf("NATS connection is not available")
	}

	// Marshal notification to JSON
	data, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %v", err)
	}

	// Publish to NATS with context timeout
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		err = p.conn.Publish(p.subject, data)
		if err != nil {
			return fmt.Errorf("failed to publish notification: %v", err)
		}
	}

	p.logger.Debug("Published notification",
		"subject", p.subject,
		"title", notification.Title,
		"when", notification.When.Format(time.RFC3339),
		"lead", notification.Lead)

	return nil
}

// PublishNotifications publishes multiple calendar notifications to NATS
func (p *Publisher) PublishNotifications(ctx context.Context, notifications []*models.Notification) error {
	if len(notifications) == 0 {
		return nil
	}

	if p.conn == nil || p.conn.IsClosed() {
		return fmt.Errorf("NATS connection is not available")
	}

	publishedCount := 0
	errorCount := 0

	for _, notification := range notifications {
		select {
		case <-ctx.Done():
			p.logger.Warn("Context cancelled while publishing notifications",
				"published", publishedCount,
				"errors", errorCount,
				"remaining", len(notifications)-publishedCount)
			return ctx.Err()
		default:
			err := p.PublishNotification(ctx, notification)
			if err != nil {
				p.logger.Error("Failed to publish notification",
					"error", err,
					"title", notification.Title)
				errorCount++
			} else {
				publishedCount++
			}
		}
	}

	p.logger.Info("Finished publishing notifications",
		"published", publishedCount,
		"errors", errorCount,
		"total", len(notifications))

	if errorCount > 0 {
		return fmt.Errorf("failed to publish %d out of %d notifications", errorCount, len(notifications))
	}

	return nil
}

// PublishEventNotifications converts events to notifications and publishes them
func (p *Publisher) PublishEventNotifications(ctx context.Context, events []*models.Event, defaultAlarms []models.Alarm) error {
	var notifications []*models.Notification

	for _, event := range events {
		// Use event alarms if available, otherwise use default alarms
		alarms := event.Alarms
		if len(alarms) == 0 {
			alarms = defaultAlarms
		}

		// Skip events with no alarms
		if len(alarms) == 0 {
			p.logger.Debug("Skipping event with no alarms", "title", event.Title)
			continue
		}

		// Create notifications for each alarm
		for _, alarm := range alarms {
			notification := models.NewNotification(event, &alarm)
			notifications = append(notifications, notification)
		}
	}

	return p.PublishNotifications(ctx, notifications)
}

// Flush ensures all published messages have been sent
func (p *Publisher) Flush(timeout time.Duration) error {
	if p.conn == nil || p.conn.IsClosed() {
		return fmt.Errorf("NATS connection is not available")
	}

	err := p.conn.FlushTimeout(timeout)
	if err != nil {
		return fmt.Errorf("failed to flush NATS messages: %v", err)
	}

	return nil
}

// IsHealthy checks if the NATS connection is healthy
func (p *Publisher) IsHealthy() error {
	if p.conn == nil {
		return fmt.Errorf("NATS connection is nil")
	}

	if p.conn.IsClosed() {
		return fmt.Errorf("NATS connection is closed")
	}

	if !p.conn.IsConnected() {
		return fmt.Errorf("NATS is not connected")
	}

	return nil
}

// Stats returns connection statistics
func (p *Publisher) Stats() nats.Statistics {
	if p.conn == nil {
		return nats.Statistics{}
	}
	return p.conn.Stats()
}

// Close gracefully closes the NATS connection
func (p *Publisher) Close() error {
	if p.conn != nil && !p.conn.IsClosed() {
		// Flush any pending messages with a timeout
		err := p.Flush(5 * time.Second)
		if err != nil {
			p.logger.Warn("Failed to flush messages on close", "error", err)
		}

		p.conn.Close()
		p.logger.Info("NATS publisher closed")
	}
	return nil
}
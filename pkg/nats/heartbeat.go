package nats

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/venkytv/nats-heartbeat/pkg/heartbeat"

	"github.com/venkytv/calendar-notifier/pkg/config"
)

// HeartbeatPublisher manages heartbeat publishing to NATS
type HeartbeatPublisher struct {
	publisher *heartbeat.Publisher
	config    *config.HeartbeatConfig
	logger    *slog.Logger
	stopChan  chan struct{}
	doneChan  chan struct{}
}

// NewHeartbeatPublisher creates a new heartbeat publisher
func NewHeartbeatPublisher(conn *nats.Conn, config *config.HeartbeatConfig, logger *slog.Logger) (*HeartbeatPublisher, error) {
	if config == nil {
		return nil, fmt.Errorf("heartbeat config is required")
	}

	if !config.Enabled {
		return nil, fmt.Errorf("heartbeat is not enabled")
	}

	if logger == nil {
		logger = slog.Default()
	}

	// Create the nats-heartbeat publisher
	pub := heartbeat.NewPublisher(conn, config.SubjectPrefix)

	hbPublisher := &HeartbeatPublisher{
		publisher: pub,
		config:    config,
		logger:    logger,
		stopChan:  make(chan struct{}),
		doneChan:  make(chan struct{}),
	}

	logger.Info("Heartbeat publisher initialized",
		"subject_prefix", config.SubjectPrefix,
		"service", config.Service,
		"description", config.Description,
		"interval", config.Interval,
		"grace_period", config.GracePeriod)

	return hbPublisher, nil
}

// Start begins publishing heartbeats at the configured interval
func (h *HeartbeatPublisher) Start(ctx context.Context) {
	go h.run(ctx)
}

// run is the main heartbeat publishing loop
func (h *HeartbeatPublisher) run(ctx context.Context) {
	defer close(h.doneChan)

	ticker := time.NewTicker(h.config.Interval)
	defer ticker.Stop()

	h.logger.Info("Starting heartbeat publisher", "interval", h.config.Interval)

	// Publish initial heartbeat immediately
	if err := h.publishHeartbeat(ctx); err != nil {
		h.logger.Error("Failed to publish initial heartbeat", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			h.logger.Info("Heartbeat publisher stopping due to context cancellation")
			return
		case <-h.stopChan:
			h.logger.Info("Heartbeat publisher stopping due to stop signal")
			return
		case <-ticker.C:
			if err := h.publishHeartbeat(ctx); err != nil {
				h.logger.Error("Failed to publish heartbeat", "error", err)
			}
		}
	}
}

// publishHeartbeat publishes a single heartbeat message
func (h *HeartbeatPublisher) publishHeartbeat(ctx context.Context) error {
	gracePeriod := h.config.GracePeriod
	msg := heartbeat.Message{
		Subject:     h.config.Service,
		GeneratedAt: time.Now(),
		Interval:    h.config.Interval,
		Description: h.config.Description,
		GracePeriod: &gracePeriod,
	}

	// Set skippable if configured
	if h.config.Skippable != nil {
		msg.Skippable = h.config.Skippable
	}

	if err := h.publisher.Publish(ctx, msg); err != nil {
		h.logger.Warn("Failed to publish heartbeat", "error", err)
		return err
	}

	h.logger.Debug("Published heartbeat",
		"subject", msg.Subject,
		"interval", msg.Interval,
		"grace_period", msg.GracePeriod)

	return nil
}

// Stop gracefully stops the heartbeat publisher
func (h *HeartbeatPublisher) Stop() error {
	h.logger.Info("Stopping heartbeat publisher")
	close(h.stopChan)

	// Wait for the publisher to finish with a timeout
	select {
	case <-h.doneChan:
		h.logger.Info("Heartbeat publisher stopped successfully")
		return nil
	case <-time.After(5 * time.Second):
		h.logger.Warn("Heartbeat publisher stop timed out")
		return fmt.Errorf("heartbeat publisher stop timed out")
	}
}

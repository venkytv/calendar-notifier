package caldav

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/venkytv/calendar-notifier/internal/models"
	calendarPkg "github.com/venkytv/calendar-notifier/pkg/calendar"
	"github.com/venkytv/calendar-notifier/pkg/calendar/ical"
	"github.com/venkytv/calendar-notifier/pkg/retry"
)

// Config holds CalDAV provider configuration
type Config struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// SimpleProvider is a basic CalDAV provider that fetches iCal data via HTTP
type SimpleProvider struct {
	name     string
	url      string
	username string
	password string
	client   *http.Client
	logger   *slog.Logger
	retryer  *retry.Retryer
}

// NewSimpleProvider creates a new simple CalDAV provider
func NewSimpleProvider() *SimpleProvider {
	logger := slog.Default()

	// Configure retry with sensible defaults for CalDAV
	retryConfig := &retry.Config{
		MaxAttempts:   3,
		InitialDelay:  2 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
		RetriableStatuses: []int{
			http.StatusRequestTimeout,      // 408
			http.StatusTooManyRequests,     // 429
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout,      // 504
		},
		RetriableErrors: []string{
			"connection refused",
			"timeout",
			"temporary failure",
			"network unreachable",
			"no such host",
			"connection reset",
		},
	}

	return &SimpleProvider{
		name: "CalDAV",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:  logger,
		retryer: retry.NewRetryer(retryConfig, logger),
	}
}

// Name returns the provider name
func (p *SimpleProvider) Name() string {
	return p.name
}

// Type returns the provider type identifier
func (p *SimpleProvider) Type() string {
	return "caldav"
}

// SetLogger sets the logger for this provider
func (p *SimpleProvider) SetLogger(logger *slog.Logger) {
	if logger != nil {
		p.logger = logger
		// Update retryer with new logger
		retryConfig := &retry.Config{
			MaxAttempts:   3,
			InitialDelay:  2 * time.Second,
			MaxDelay:      30 * time.Second,
			BackoffFactor: 2.0,
			Jitter:        true,
			RetriableStatuses: []int{
				http.StatusRequestTimeout,      // 408
				http.StatusTooManyRequests,     // 429
				http.StatusInternalServerError, // 500
				http.StatusBadGateway,          // 502
				http.StatusServiceUnavailable,  // 503
				http.StatusGatewayTimeout,      // 504
			},
			RetriableErrors: []string{
				"connection refused",
				"timeout",
				"temporary failure",
				"network unreachable",
				"no such host",
				"connection reset",
			},
		}
		p.retryer = retry.NewRetryer(retryConfig, logger)
	}
}

// Initialize sets up the CalDAV provider with credentials file (not used for CalDAV)
func (p *SimpleProvider) Initialize(ctx context.Context, configPath string) error {
	return fmt.Errorf("CalDAV provider requires direct configuration, not file-based credentials")
}

// InitializeWithConfig sets up the CalDAV provider with direct configuration
func (p *SimpleProvider) InitializeWithConfig(config *Config) error {
	if config.URL == "" {
		return fmt.Errorf("CalDAV URL is required")
	}
	if config.Username == "" {
		return fmt.Errorf("CalDAV username is required")
	}
	if config.Password == "" {
		return fmt.Errorf("CalDAV password is required")
	}

	p.url = config.URL
	p.username = config.Username
	p.password = config.Password

	return nil
}

// GetEvents retrieves events from CalDAV server by fetching iCal data
func (p *SimpleProvider) GetEvents(ctx context.Context, calendarIDs []string, from, to time.Time) ([]*models.Event, error) {
	if p.url == "" {
		return nil, fmt.Errorf("CalDAV provider not initialized")
	}

	// For simplicity, we'll fetch the main calendar URL
	// In a real implementation, we'd iterate through calendar IDs
	icalData, err := p.fetchICalData(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch iCal data: %v", err)
	}

	// Parse iCal data using shared parser
	// Pass username as userEmail to identify the authenticated user in attendee lists
	events, err := ical.ParseICalData(icalData, p.url, "CalDAV Calendar", from, to, p.username, p.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to parse iCal data: %v", err)
	}

	return events, nil
}

// fetchICalData retrieves iCal data from the CalDAV server with retry logic
func (p *SimpleProvider) fetchICalData(ctx context.Context) (string, error) {
	operation := func() (interface{}, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", p.url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %v", err)
		}

		// Add basic authentication
		req.SetBasicAuth(p.username, p.password)
		req.Header.Set("Accept", "text/calendar")
		req.Header.Set("User-Agent", "calendar-notifier/1.0")

		p.logger.Debug("Fetching CalDAV data", "url", p.url, "username", p.username)

		resp, err := p.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("HTTP request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			// Create HTTP error for proper retry classification
			httpErr := retry.NewHTTPError(resp.StatusCode, resp.Status, p.url)
			p.logger.Warn("HTTP error when fetching CalDAV data",
				"url", p.url,
				"username", p.username,
				"status_code", resp.StatusCode,
				"status", resp.Status)
			return nil, httpErr
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %v", err)
		}

		p.logger.Debug("Successfully fetched CalDAV data",
			"url", p.url,
			"username", p.username,
			"content_length", len(body))

		return string(body), nil
	}

	result, err := p.retryer.DoWithResult(ctx, operation)
	if err != nil {
		p.logger.Error("Failed to fetch CalDAV data after retries",
			"url", p.url,
			"username", p.username,
			"error", err)
		return "", err
	}

	return result.(string), nil
}

// GetCalendars returns a basic calendar list (simplified for this provider)
func (p *SimpleProvider) GetCalendars(ctx context.Context) ([]*calendarPkg.Calendar, error) {
	if p.url == "" {
		return nil, fmt.Errorf("CalDAV provider not initialized")
	}

	// Return a single calendar representing this CalDAV endpoint
	calendar := &calendarPkg.Calendar{
		ID:          p.url,
		Name:        "CalDAV Calendar",
		Description: fmt.Sprintf("Calendar from %s", p.url),
		Primary:     true,
		AccessRole:  "owner",
	}

	return []*calendarPkg.Calendar{calendar}, nil
}

// IsHealthy performs a health check by attempting to fetch calendar data
func (p *SimpleProvider) IsHealthy(ctx context.Context) error {
	if p.url == "" {
		return fmt.Errorf("CalDAV provider not initialized")
	}

	// Try to fetch data as a health check
	_, err := p.fetchICalData(ctx)
	if err != nil {
		return fmt.Errorf("CalDAV health check failed: %v", err)
	}

	return nil
}

// Close cleans up resources
func (p *SimpleProvider) Close() error {
	// HTTP client doesn't require explicit cleanup
	return nil
}

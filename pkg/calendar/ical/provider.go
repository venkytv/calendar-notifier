package ical

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/venkytv/calendar-notifier/internal/models"
	calendarPkg "github.com/venkytv/calendar-notifier/pkg/calendar"
	"github.com/venkytv/calendar-notifier/pkg/retry"
)

// Provider is an iCal provider using the arran4/golang-ical library
type Provider struct {
	name    string
	url     string
	client  *http.Client
	logger  *slog.Logger
	retryer *retry.Retryer
}

// NewProvider creates a new iCal provider using arran4/golang-ical
func NewProvider() *Provider {
	logger := slog.Default()

	// Configure retry with sensible defaults for calendar fetching
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

	return &Provider{
		name: "iCal",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:  logger,
		retryer: retry.NewRetryer(retryConfig, logger),
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return p.name
}

// Type returns the provider type identifier
func (p *Provider) Type() string {
	return "ical"
}

// SetLogger sets the logger for this provider
func (p *Provider) SetLogger(logger *slog.Logger) {
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

// Initialize sets up the iCal provider with the URL
func (p *Provider) Initialize(ctx context.Context, url string) error {
	if url == "" {
		return fmt.Errorf("iCal URL is required")
	}

	p.url = url
	p.logger.Info("Initialized iCal provider", "url", url)
	return nil
}

// GetEvents retrieves events from iCal URL
func (p *Provider) GetEvents(ctx context.Context, calendarIDs []string, from, to time.Time) ([]*models.Event, error) {
	if p.url == "" {
		return nil, fmt.Errorf("iCal provider not initialized")
	}

	// Fetch iCal data
	icalData, err := p.fetchICalData(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch iCal data: %v", err)
	}

	// Parse iCal data using shared parser
	events, err := ParseICalData(icalData, p.url, "iCal Calendar", from, to, p.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to parse iCal data: %v", err)
	}

	return events, nil
}

// fetchICalData retrieves iCal data from the URL with retry logic
func (p *Provider) fetchICalData(ctx context.Context) (string, error) {
	operation := func() (interface{}, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", p.url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %v", err)
		}

		req.Header.Set("Accept", "text/calendar,application/calendar")
		req.Header.Set("User-Agent", "calendar-notifier/1.0")

		p.logger.Debug("Fetching iCal data", "url", p.url)

		resp, err := p.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("HTTP request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			// Create HTTP error for proper retry classification
			httpErr := retry.NewHTTPError(resp.StatusCode, resp.Status, p.url)
			p.logger.Warn("HTTP error when fetching iCal data",
				"url", p.url,
				"status_code", resp.StatusCode,
				"status", resp.Status)
			return nil, httpErr
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %v", err)
		}

		p.logger.Debug("Successfully fetched iCal data",
			"url", p.url,
			"content_length", len(body))

		return string(body), nil
	}

	result, err := p.retryer.DoWithResult(ctx, operation)
	if err != nil {
		p.logger.Error("Failed to fetch iCal data after retries",
			"url", p.url,
			"error", err)
		return "", err
	}

	return result.(string), nil
}

// GetCalendars returns a basic calendar list
func (p *Provider) GetCalendars(ctx context.Context) ([]*calendarPkg.Calendar, error) {
	if p.url == "" {
		return nil, fmt.Errorf("iCal provider not initialized")
	}

	// Return a single calendar representing this iCal URL
	calendar := &calendarPkg.Calendar{
		ID:          p.url,
		Name:        "iCal Calendar",
		Description: fmt.Sprintf("Calendar from %s", p.url),
		Primary:     true,
		AccessRole:  "reader",
	}

	return []*calendarPkg.Calendar{calendar}, nil
}

// IsHealthy performs a health check by attempting to fetch calendar data
func (p *Provider) IsHealthy(ctx context.Context) error {
	if p.url == "" {
		return fmt.Errorf("iCal provider not initialized")
	}

	// Try to fetch data as a health check
	_, err := p.fetchICalData(ctx)
	if err != nil {
		return fmt.Errorf("iCal health check failed: %v", err)
	}

	return nil
}

// Close cleans up resources
func (p *Provider) Close() error {
	// HTTP client doesn't require explicit cleanup
	return nil
}
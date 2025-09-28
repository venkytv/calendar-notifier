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
)

// Provider is an iCal provider using the arran4/golang-ical library
type Provider struct {
	name   string
	url    string
	client *http.Client
	logger *slog.Logger
}

// NewProvider creates a new iCal provider using arran4/golang-ical
func NewProvider() *Provider {
	return &Provider{
		name: "iCal",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: slog.Default(),
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

// fetchICalData retrieves iCal data from the URL
func (p *Provider) fetchICalData(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Accept", "text/calendar,application/calendar")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	return string(body), nil
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
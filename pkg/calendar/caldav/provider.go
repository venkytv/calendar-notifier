package caldav

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/venkytv/calendar-notifier/internal/models"
	calendarPkg "github.com/venkytv/calendar-notifier/pkg/calendar"
	"github.com/venkytv/calendar-notifier/pkg/calendar/ical"
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
}

// NewSimpleProvider creates a new simple CalDAV provider
func NewSimpleProvider() *SimpleProvider {
	return &SimpleProvider{
		name: "CalDAV",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
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
	events, err := ical.ParseICalData(icalData, p.url, "CalDAV Calendar", from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to parse iCal data: %v", err)
	}

	return events, nil
}

// fetchICalData retrieves iCal data from the CalDAV server
func (p *SimpleProvider) fetchICalData(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Add basic authentication
	req.SetBasicAuth(p.username, p.password)
	req.Header.Set("Accept", "text/calendar")

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

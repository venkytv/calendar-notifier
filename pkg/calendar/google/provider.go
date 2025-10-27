package google

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/venkytv/calendar-notifier/internal/models"
	pkgcalendar "github.com/venkytv/calendar-notifier/pkg/calendar"
)

const (
	providerType = "google"
	providerName = "Google Calendar"
)

// Provider implements the calendar.Provider interface for Google Calendar
type Provider struct {
	name         string
	tokenManager *TokenManager
	service      *calendar.Service
	tokenFile    string
	calendarIDs  []string
	logger       *slog.Logger
}

// NewProvider creates a new Google Calendar provider
func NewProvider() *Provider {
	return &Provider{
		name:   providerName,
		logger: slog.Default(),
	}
}

// Name returns the human-readable name of the provider
func (p *Provider) Name() string {
	return p.name
}

// Type returns the provider type identifier
func (p *Provider) Type() string {
	return providerType
}

// SetLogger configures the logger for this provider
func (p *Provider) SetLogger(logger *slog.Logger) {
	if logger != nil {
		p.logger = logger
	}
}

// Initialize sets up the Google Calendar provider with OAuth2 credentials
// credentialsPath should point to the OAuth2 credentials JSON file
func (p *Provider) Initialize(ctx context.Context, credentialsPath string) error {
	p.logger.Info("initializing Google Calendar provider", "credentials", credentialsPath)

	// Token file path - store next to credentials with .token suffix
	if p.tokenFile == "" {
		p.tokenFile = credentialsPath + ".token"
	}

	// Create token manager
	tm, err := NewTokenManager(credentialsPath, p.tokenFile, p.logger)
	if err != nil {
		return fmt.Errorf("failed to create token manager: %w", err)
	}
	p.tokenManager = tm

	// Check if we have a valid token
	if !tm.IsTokenValid() {
		authURL := tm.GetAuthURL()
		return fmt.Errorf("authentication required: visit this URL to authorize:\n%s\n\nThen run with the authorization code", authURL)
	}

	// Get authenticated HTTP client
	client, err := tm.GetClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to get authenticated client: %w", err)
	}

	// Create Calendar service
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("failed to create calendar service: %w", err)
	}
	p.service = srv

	p.logger.Info("Google Calendar provider initialized successfully")
	return nil
}

// Authenticate performs the initial OAuth2 authentication with the provided code
func (p *Provider) Authenticate(ctx context.Context, authCode string) error {
	if p.tokenManager == nil {
		return fmt.Errorf("provider not initialized")
	}

	_, err := p.tokenManager.ExchangeCode(ctx, authCode)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Re-initialize with the new token
	client, err := p.tokenManager.GetClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to get authenticated client: %w", err)
	}

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("failed to create calendar service: %w", err)
	}
	p.service = srv

	p.logger.Info("authentication successful")
	return nil
}

// GetAuthURL returns the OAuth2 authorization URL for initial authentication
func (p *Provider) GetAuthURL() (string, error) {
	if p.tokenManager == nil {
		return "", fmt.Errorf("provider not initialized")
	}
	return p.tokenManager.GetAuthURL(), nil
}

// GetCalendars returns a list of available calendars
// If calendar IDs are configured, only those calendars are returned
func (p *Provider) GetCalendars(ctx context.Context) ([]*pkgcalendar.Calendar, error) {
	if p.service == nil {
		return nil, fmt.Errorf("provider not initialized")
	}

	p.logger.Debug("fetching calendar list")

	calendarList, err := p.service.CalendarList.List().Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch calendar list: %w", err)
	}

	// Build a map for quick lookup if we have configured IDs
	var configuredIDs map[string]bool
	if len(p.calendarIDs) > 0 {
		configuredIDs = make(map[string]bool)
		for _, id := range p.calendarIDs {
			configuredIDs[id] = true
		}
	}

	var calendars []*pkgcalendar.Calendar
	for _, item := range calendarList.Items {
		// If calendar IDs are configured, only include those
		if configuredIDs != nil && !configuredIDs[item.Id] {
			continue
		}

		calendars = append(calendars, &pkgcalendar.Calendar{
			ID:          item.Id,
			Name:        item.Summary,
			Description: item.Description,
			TimeZone:    item.TimeZone,
			Primary:     item.Primary,
			AccessRole:  item.AccessRole,
		})
	}

	p.logger.Debug("fetched calendars", "count", len(calendars))
	return calendars, nil
}

// GetEvents retrieves events from specified calendars within the time range
func (p *Provider) GetEvents(ctx context.Context, calendarIDs []string, from, to time.Time) ([]*models.Event, error) {
	if p.service == nil {
		return nil, fmt.Errorf("provider not initialized")
	}

	p.logger.Debug("fetching events",
		"calendar_count", len(calendarIDs),
		"from", from.Format(time.RFC3339),
		"to", to.Format(time.RFC3339))

	var allEvents []*models.Event

	for _, calendarID := range calendarIDs {
		events, err := p.getEventsFromCalendar(ctx, calendarID, from, to)
		if err != nil {
			p.logger.Error("failed to fetch events from calendar",
				"calendar_id", calendarID,
				"error", err)
			return nil, fmt.Errorf("failed to fetch events from calendar %s: %w", calendarID, err)
		}

		allEvents = append(allEvents, events...)
	}

	p.logger.Debug("fetched events", "total_count", len(allEvents))
	return allEvents, nil
}

// getEventsFromCalendar fetches events from a single calendar
func (p *Provider) getEventsFromCalendar(ctx context.Context, calendarID string, from, to time.Time) ([]*models.Event, error) {
	call := p.service.Events.List(calendarID).
		Context(ctx).
		TimeMin(from.Format(time.RFC3339)).
		TimeMax(to.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime")

	eventsResult, err := call.Do()
	if err != nil {
		return nil, err
	}

	var events []*models.Event
	for _, item := range eventsResult.Items {
		// Skip cancelled events
		if item.Status == "cancelled" {
			continue
		}

		event, err := p.convertEvent(item, calendarID)
		if err != nil {
			p.logger.Warn("failed to convert event",
				"event_id", item.Id,
				"calendar_id", calendarID,
				"error", err)
			continue
		}

		events = append(events, event)
	}

	return events, nil
}

// IsHealthy performs a health check on the provider
func (p *Provider) IsHealthy(ctx context.Context) error {
	if p.service == nil {
		return fmt.Errorf("provider not initialized")
	}

	// Try to fetch calendar list as a health check
	_, err := p.service.CalendarList.List().Context(ctx).MaxResults(1).Do()
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	return nil
}

// Close cleans up any resources used by the provider
func (p *Provider) Close() error {
	p.logger.Debug("closing Google Calendar provider")
	p.service = nil
	p.tokenManager = nil
	return nil
}

// SetTokenFile sets a custom token file path (must be called before Initialize)
func (p *Provider) SetTokenFile(tokenFile string) {
	p.tokenFile = tokenFile
}

// SetCalendarIDs sets the calendar IDs to monitor
func (p *Provider) SetCalendarIDs(calendarIDs []string) {
	p.calendarIDs = calendarIDs
}

// GetConfiguredCalendarIDs returns the configured calendar IDs
func (p *Provider) GetConfiguredCalendarIDs() []string {
	return p.calendarIDs
}

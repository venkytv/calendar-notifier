package google

import (
	"context"
	"fmt"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/venkytv/calendar-notifier/internal/models"
	calendarPkg "github.com/venkytv/calendar-notifier/pkg/calendar"
)

// Provider implements the calendar.Provider interface for Google Calendar
type Provider struct {
	name            string
	credentialsPath string
	service         *calendar.Service
	config          *oauth2.Config
}

// NewProvider creates a new Google Calendar provider
func NewProvider() *Provider {
	return &Provider{
		name: "Google Calendar",
	}
}

// Name returns the human-readable name of the provider
func (p *Provider) Name() string {
	return p.name
}

// Type returns the provider type identifier
func (p *Provider) Type() string {
	return "google"
}

// Initialize sets up the Google Calendar provider with credentials
func (p *Provider) Initialize(ctx context.Context, credentialsPath string) error {
	p.credentialsPath = credentialsPath

	// Read credentials from file
	credentials, err := os.ReadFile(credentialsPath)
	if err != nil {
		return fmt.Errorf("unable to read client secret file: %v", err)
	}

	// Parse the credentials and create oauth2 config
	config, err := google.ConfigFromJSON(credentials, calendar.CalendarReadonlyScope)
	if err != nil {
		return fmt.Errorf("unable to parse client secret file to config: %v", err)
	}
	p.config = config

	// Create calendar service with a basic token (for service accounts or pre-authorized tokens)
	// In a real implementation, this would handle the full OAuth2 flow
	client := config.Client(ctx, &oauth2.Token{})

	service, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("unable to retrieve Calendar client: %v", err)
	}

	p.service = service
	return nil
}

// GetEvents retrieves events from Google Calendar
func (p *Provider) GetEvents(ctx context.Context, calendarIDs []string, from, to time.Time) ([]*models.Event, error) {
	if p.service == nil {
		return nil, fmt.Errorf("calendar service not initialized")
	}

	var allEvents []*models.Event

	for _, calendarID := range calendarIDs {
		// Get events from Google Calendar API
		eventsCall := p.service.Events.List(calendarID).
			Context(ctx).
			TimeMin(from.Format(time.RFC3339)).
			TimeMax(to.Format(time.RFC3339)).
			SingleEvents(true).
			OrderBy("startTime")

		eventsResult, err := eventsCall.Do()
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve events for calendar %s: %v", calendarID, err)
		}

		for _, item := range eventsResult.Items {
			event, err := p.convertGoogleEventToInternalEvent(item, calendarID)
			if err != nil {
				continue // Skip events that can't be converted
			}
			allEvents = append(allEvents, event)
		}
	}

	return allEvents, nil
}

// GetCalendars returns available Google calendars
func (p *Provider) GetCalendars(ctx context.Context) ([]*calendarPkg.Calendar, error) {
	if p.service == nil {
		return nil, fmt.Errorf("calendar service not initialized")
	}

	calendarList, err := p.service.CalendarList.List().Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve calendar list: %v", err)
	}

	var calendars []*calendarPkg.Calendar
	for _, item := range calendarList.Items {
		cal := &calendarPkg.Calendar{
			ID:          item.Id,
			Name:        item.Summary,
			Description: item.Description,
			TimeZone:    item.TimeZone,
			Primary:     item.Primary,
			AccessRole:  item.AccessRole,
		}
		calendars = append(calendars, cal)
	}

	return calendars, nil
}

// IsHealthy performs a health check on the Google Calendar connection
func (p *Provider) IsHealthy(ctx context.Context) error {
	if p.service == nil {
		return fmt.Errorf("calendar service not initialized")
	}

	// Perform a simple API call to check connectivity
	_, err := p.service.CalendarList.List().Context(ctx).MaxResults(1).Do()
	if err != nil {
		return fmt.Errorf("health check failed: %v", err)
	}

	return nil
}

// Close cleans up resources
func (p *Provider) Close() error {
	// Google Calendar service doesn't require explicit cleanup
	// HTTP client connections are managed by the underlying client
	p.service = nil
	p.config = nil
	return nil
}

// convertGoogleEventToInternalEvent converts a Google Calendar event to our internal Event model
func (p *Provider) convertGoogleEventToInternalEvent(item *calendar.Event, calendarID string) (*models.Event, error) {
	event := &models.Event{
		ID:          item.Id,
		Title:       item.Summary,
		Description: item.Description,
		CalendarID:  calendarID,
		Location:    item.Location,
	}

	// Parse start time
	var startTime time.Time
	var err error
	if item.Start.DateTime != "" {
		startTime, err = time.Parse(time.RFC3339, item.Start.DateTime)
	} else if item.Start.Date != "" {
		startTime, err = time.Parse("2006-01-02", item.Start.Date)
	}
	if err != nil {
		return nil, fmt.Errorf("unable to parse start time: %v", err)
	}
	event.StartTime = startTime

	// Parse end time
	var endTime time.Time
	if item.End.DateTime != "" {
		endTime, err = time.Parse(time.RFC3339, item.End.DateTime)
	} else if item.End.Date != "" {
		endTime, err = time.Parse("2006-01-02", item.End.Date)
	}
	if err != nil {
		return nil, fmt.Errorf("unable to parse end time: %v", err)
	}
	event.EndTime = endTime

	// Parse created and modified times
	if item.Created != "" {
		if createdTime, err := time.Parse(time.RFC3339, item.Created); err == nil {
			event.CreatedAt = createdTime
		}
	}
	if item.Updated != "" {
		if modifiedTime, err := time.Parse(time.RFC3339, item.Updated); err == nil {
			event.ModifiedAt = modifiedTime
		}
	}

	// Convert reminders/alarms
	if item.Reminders != nil && item.Reminders.UseDefault {
		// Use default reminder settings
		event.Alarms = []models.Alarm{
			{LeadTimeMinutes: 10, Method: "popup", Severity: "normal"},
		}
	} else if item.Reminders != nil && len(item.Reminders.Overrides) > 0 {
		// Use custom reminder settings
		for _, reminder := range item.Reminders.Overrides {
			alarm := models.Alarm{
				LeadTimeMinutes: int(reminder.Minutes),
				Method:          reminder.Method,
				Severity:        "normal",
			}
			event.Alarms = append(event.Alarms, alarm)
		}
	}

	return event, nil
}
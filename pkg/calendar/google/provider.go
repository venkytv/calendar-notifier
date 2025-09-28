package google

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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
	oauth2Config    *oauth2.Config
	tokenPath       string
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

	// Read credentials file to determine type
	credentialsData, err := os.ReadFile(credentialsPath)
	if err != nil {
		return fmt.Errorf("unable to read credentials file: %v", err)
	}

	// Parse JSON to check credential type
	var credType struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(credentialsData, &credType); err != nil {
		return fmt.Errorf("unable to parse credentials JSON: %v", err)
	}

	var service *calendar.Service

	switch credType.Type {
	case "service_account":
		// Use service account credentials
		service, err = calendar.NewService(ctx, option.WithCredentialsFile(credentialsPath))
		if err != nil {
			return fmt.Errorf("unable to create Calendar service with service account: %v", err)
		}

	case "":
		// Assume OAuth2 client credentials (no "type" field)
		fallthrough
	default:
		// Use OAuth2 client credentials
		config, err := google.ConfigFromJSON(credentialsData, calendar.CalendarReadonlyScope)
		if err != nil {
			return fmt.Errorf("unable to parse OAuth2 client credentials: %v", err)
		}
		p.oauth2Config = config

		// Set up token storage path
		p.tokenPath = filepath.Join(filepath.Dir(credentialsPath), "token.json")

		// Get OAuth2 token (will prompt for auth if needed)
		client, err := p.getOAuth2Client(ctx)
		if err != nil {
			return fmt.Errorf("unable to get OAuth2 client: %v", err)
		}

		service, err = calendar.NewService(ctx, option.WithHTTPClient(client))
		if err != nil {
			return fmt.Errorf("unable to create Calendar service with OAuth2: %v", err)
		}
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

	// Log calendar access for debugging
	if len(calendarList.Items) == 0 {
		fmt.Printf("WARNING: Service account has access to 0 calendars. This usually means:\n")
		fmt.Printf("  1. No calendars are shared with the service account email, OR\n")
		fmt.Printf("  2. You need to use OAuth2 client credentials instead of service account\n")
		fmt.Printf("  3. Service account email: check 'client_email' field in your credentials JSON\n")
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
	return nil
}

// getOAuth2Client gets an OAuth2 HTTP client, handling token storage and refresh
func (p *Provider) getOAuth2Client(ctx context.Context) (*http.Client, error) {
	// Try to load existing token
	token, err := p.loadToken()
	if err != nil {
		// No valid token found, need to authorize
		token, err = p.authorizeNewToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("unable to authorize: %v", err)
		}
	}

	// Create client with token
	client := p.oauth2Config.Client(ctx, token)
	return client, nil
}

// loadToken loads token from file
func (p *Provider) loadToken() (*oauth2.Token, error) {
	file, err := os.Open(p.tokenPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	token := &oauth2.Token{}
	err = json.NewDecoder(file).Decode(token)
	return token, err
}

// saveToken saves token to file
func (p *Provider) saveToken(token *oauth2.Token) error {
	file, err := os.OpenFile(p.tokenPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewEncoder(file).Encode(token)
}

// authorizeNewToken performs OAuth2 authorization flow
func (p *Provider) authorizeNewToken(ctx context.Context) (*oauth2.Token, error) {
	authURL := p.oauth2Config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("\nGo to the following link in your browser:\n%v\n\n", authURL)
	fmt.Print("Enter the authorization code: ")

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %v", err)
	}

	token, err := p.oauth2Config.Exchange(ctx, authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %v", err)
	}

	// Save token for future use
	if err := p.saveToken(token); err != nil {
		fmt.Printf("Warning: unable to save token: %v\n", err)
	}

	return token, nil
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
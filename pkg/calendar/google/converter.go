package google

import (
	"fmt"
	"time"

	"google.golang.org/api/calendar/v3"

	"github.com/venkytv/calendar-notifier/internal/models"
)

// convertEvent converts a Google Calendar event to our internal Event model
func (p *Provider) convertEvent(item *calendar.Event, calendarID string) (*models.Event, error) {
	// Parse start time
	startTime, err := parseEventTime(item.Start)
	if err != nil {
		return nil, fmt.Errorf("failed to parse start time: %w", err)
	}

	// Parse end time
	endTime, err := parseEventTime(item.End)
	if err != nil {
		return nil, fmt.Errorf("failed to parse end time: %w", err)
	}

	// Parse created time
	var createdAt time.Time
	if item.Created != "" {
		createdAt, err = time.Parse(time.RFC3339, item.Created)
		if err != nil {
			p.logger.Warn("failed to parse created time, using zero value",
				"event_id", item.Id,
				"error", err)
		}
	}

	// Parse updated time
	var modifiedAt time.Time
	if item.Updated != "" {
		modifiedAt, err = time.Parse(time.RFC3339, item.Updated)
		if err != nil {
			p.logger.Warn("failed to parse updated time, using zero value",
				"event_id", item.Id,
				"error", err)
		}
	}

	// Convert reminders to alarms
	alarms := p.convertReminders(item)

	// Extract response status from attendees
	responseStatus := extractResponseStatus(item)

	event := &models.Event{
		ID:             item.Id,
		Title:          item.Summary,
		Description:    item.Description,
		StartTime:      startTime,
		EndTime:        endTime,
		Alarms:         alarms,
		CalendarID:     calendarID,
		Location:       item.Location,
		CreatedAt:      createdAt,
		ModifiedAt:     modifiedAt,
		ResponseStatus: responseStatus,
	}

	return event, nil
}

// parseEventTime parses Google Calendar event time (handles both dateTime and date fields)
func parseEventTime(eventTime *calendar.EventDateTime) (time.Time, error) {
	if eventTime == nil {
		return time.Time{}, fmt.Errorf("event time is nil")
	}

	// Try DateTime first (for events with specific times)
	if eventTime.DateTime != "" {
		t, err := time.Parse(time.RFC3339, eventTime.DateTime)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse datetime: %w", err)
		}
		return t, nil
	}

	// Fall back to Date (for all-day events)
	if eventTime.Date != "" {
		// Parse as date only (YYYY-MM-DD format)
		t, err := time.Parse("2006-01-02", eventTime.Date)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse date: %w", err)
		}

		// If timezone is specified, use it
		if eventTime.TimeZone != "" {
			loc, err := time.LoadLocation(eventTime.TimeZone)
			if err == nil {
				t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
			}
		}

		return t, nil
	}

	return time.Time{}, fmt.Errorf("no datetime or date field found")
}

// convertReminders converts Google Calendar reminders to our Alarm model
func (p *Provider) convertReminders(item *calendar.Event) []models.Alarm {
	if item.Reminders == nil {
		return nil
	}

	var alarms []models.Alarm

	// Check if default reminders are enabled
	if item.Reminders.UseDefault {
		// For default reminders, we can't know the exact values without fetching
		// the calendar settings. We'll use a sensible default.
		p.logger.Debug("event uses default reminders",
			"event_id", item.Id,
			"title", item.Summary)

		// Google's typical default is 10 minutes before
		alarms = append(alarms, models.Alarm{
			LeadTimeMinutes: 10,
			Severity:        "normal",
			Method:          "popup",
		})

		return alarms
	}

	// Process override reminders
	if len(item.Reminders.Overrides) > 0 {
		for _, override := range item.Reminders.Overrides {
			alarm := convertReminderOverride(override)
			alarms = append(alarms, alarm)
		}
	}

	return alarms
}

// extractResponseStatus extracts the authenticated user's response status from attendees
func extractResponseStatus(item *calendar.Event) string {
	if item.Attendees == nil || len(item.Attendees) == 0 {
		// No attendees means this is likely an event the user created
		// or a calendar without attendee tracking - treat as accepted
		return ""
	}

	// Find the attendee marked as "self" (the authenticated user)
	for _, attendee := range item.Attendees {
		if attendee.Self {
			// Map Google Calendar response status to our internal format
			// Google uses: "needsAction", "declined", "tentative", "accepted"
			switch attendee.ResponseStatus {
			case "accepted":
				return "accepted"
			case "declined":
				return "declined"
			case "tentative":
				return "tentative"
			case "needsAction":
				return "needsAction"
			default:
				return attendee.ResponseStatus
			}
		}
	}

	// If we didn't find "self", this might be an event the user organizes
	// or we're looking at someone else's calendar - treat as accepted
	return ""
}

// convertReminderOverride converts a Google Calendar reminder override to an Alarm
func convertReminderOverride(override *calendar.EventReminder) models.Alarm {
	// Google Calendar minutes are already in minutes before the event
	leadTime := int(override.Minutes)

	// Determine severity based on method
	// Valid Google Calendar reminder methods: "email", "popup"
	severity := "normal"
	method := override.Method

	switch override.Method {
	case "email":
		// Email reminders tend to be for longer lead times or more important events
		severity = "high"
		method = "email"
	case "popup":
		// Standard popup/notification reminder
		severity = "normal"
		method = "popup"
	default:
		// Unknown method, log and use as-is
		method = override.Method
	}

	return models.Alarm{
		LeadTimeMinutes: leadTime,
		Severity:        severity,
		Method:          method,
	}
}

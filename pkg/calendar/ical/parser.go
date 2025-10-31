package ical

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"

	"github.com/venkytv/calendar-notifier/internal/models"
)

// ParseICalData parses iCal data using the arran4/golang-ical library
func ParseICalData(icalData string, calendarID, calendarName string, from, to time.Time, userEmail string, logger *slog.Logger) ([]*models.Event, error) {
	if logger == nil {
		logger = slog.Default()
	}
	// Parse iCal data using arran4/golang-ical
	calendar, err := ics.ParseCalendar(strings.NewReader(icalData))
	if err != nil {
		return nil, fmt.Errorf("failed to parse iCal data: %v", err)
	}

	var events []*models.Event

	// Process each event in the calendar
	for _, event := range calendar.Events() {
		internalEvent, err := ConvertICSEventToInternalEvent(event, calendarID, calendarName, userEmail, logger)
		if err != nil {
			logger.Warn("Failed to convert iCal event", "error", err, "calendar_id", calendarID)
			continue
		}

		// Filter events that fall within our time range
		if internalEvent.StartTime.Before(to) && internalEvent.EndTime.After(from) {
			events = append(events, internalEvent)
		}
	}

	return events, nil
}

// ConvertICSEventToInternalEvent converts an ics.VEvent to our internal Event model
func ConvertICSEventToInternalEvent(event *ics.VEvent, calendarID, calendarName string, userEmail string, logger *slog.Logger) (*models.Event, error) {
	if logger == nil {
		logger = slog.Default()
	}
	internalEvent := &models.Event{
		CalendarID:   calendarID,
		CalendarName: calendarName,
	}

	// Extract basic properties using the library's methods
	if event.Id() != "" {
		internalEvent.ID = event.Id()
	}

	if summary := event.GetProperty(ics.ComponentPropertySummary); summary != nil {
		internalEvent.Title = summary.Value
	}

	if description := event.GetProperty(ics.ComponentPropertyDescription); description != nil {
		internalEvent.Description = description.Value
	}

	if location := event.GetProperty(ics.ComponentPropertyLocation); location != nil {
		internalEvent.Location = location.Value
	}

	// Parse start time
	startTime, err := event.GetStartAt()
	if err == nil {
		internalEvent.StartTime = startTime
	} else {
		return nil, fmt.Errorf("failed to parse start time: %v", err)
	}

	// Parse end time
	endTime, err := event.GetEndAt()
	if err == nil {
		internalEvent.EndTime = endTime
	} else {
		// Set default end time if not provided (assume 1 hour duration)
		if !internalEvent.StartTime.IsZero() {
			internalEvent.EndTime = internalEvent.StartTime.Add(1 * time.Hour)
		}
	}

	// Extract alarms using the library's methods
	for _, alarm := range event.Alarms() {
		internalAlarm, err := ConvertICSAlarmToInternalAlarm(alarm, event, calendarID, logger)
		if err != nil {
			logger.Warn("Failed to convert iCal alarm", "error", err, "event_id", internalEvent.ID, "calendar_id", calendarID)
			continue
		}
		internalEvent.Alarms = append(internalEvent.Alarms, *internalAlarm)
	}

	// Extract response status from attendees
	internalEvent.ResponseStatus = extractResponseStatusFromAttendees(event, userEmail)

	// Validate required fields
	if internalEvent.ID == "" {
		return nil, fmt.Errorf("event missing UID")
	}
	if internalEvent.StartTime.IsZero() {
		return nil, fmt.Errorf("event missing start time")
	}

	return internalEvent, nil
}

// ConvertICSAlarmToInternalAlarm converts an ics.VAlarm to our internal Alarm model
func ConvertICSAlarmToInternalAlarm(alarm *ics.VAlarm, event *ics.VEvent, calendarID string, logger *slog.Logger) (*models.Alarm, error) {
	if logger == nil {
		logger = slog.Default()
	}
	internalAlarm := &models.Alarm{
		Method:   "popup", // Default
		Severity: "normal", // Default
	}

	// Get action
	if action := alarm.GetProperty(ics.ComponentPropertyAction); action != nil {
		internalAlarm.Method = action.Value
	}

	// Get trigger - handle both duration and absolute time formats
	if trigger := alarm.GetProperty(ics.ComponentPropertyTrigger); trigger != nil {
		triggerValue := trigger.Value

		// Check if it's an absolute time (format: YYYYMMDDTHHMMSSZ)
		if len(triggerValue) > 10 && (triggerValue[8] == 'T' || triggerValue[len(triggerValue)-1] == 'Z') {
			// This is an absolute trigger time - we can't calculate lead time without event context
			// For now, use a default lead time
			logger.Warn("Absolute trigger times not fully supported, using default lead time", "trigger_value", triggerValue, "event_id", event.Id(), "calendar_id", calendarID)
			internalAlarm.LeadTimeMinutes = 15 // Default
		} else {
			// Parse iCal duration format (e.g., "-P0DT0H5M0S", "-PT15M")
			duration, err := parseICalDuration(triggerValue)
			if err != nil {
				logger.Warn("Failed to parse trigger duration, using default", "trigger_value", triggerValue, "error", err, "event_id", event.Id(), "calendar_id", calendarID)
				internalAlarm.LeadTimeMinutes = 15 // Default
			} else {
				// Convert to positive minutes (scheduler expects positive lead times)
				minutes := int(duration.Abs().Minutes())
				internalAlarm.LeadTimeMinutes = minutes
			}
		}
	}

	return internalAlarm, nil
}

// parseICalDuration parses iCal duration (e.g., "-P0DT0H5M0S", "-PT15M", "P0DT0H5M0S")
func parseICalDuration(duration string) (time.Duration, error) {
	// Remove leading negative sign and remember it
	negative := false
	if len(duration) > 0 && duration[0] == '-' {
		negative = true
		duration = duration[1:]
	}

	var result time.Duration

	// Handle full iCal format: P[n]DT[n]H[n]M[n]S
	if len(duration) > 2 && duration[0] == 'P' {
		// Parse P0DT0H5M0S format
		remaining := duration[1:] // Skip 'P'

		// Parse days (if present)
		if dayIndex := strings.Index(remaining, "D"); dayIndex >= 0 {
			// Skip days for now, move to time part
			if tIndex := strings.Index(remaining, "T"); tIndex >= 0 {
				remaining = remaining[tIndex+1:] // Skip to time part after 'T'
			}
		} else if strings.HasPrefix(remaining, "T") {
			remaining = remaining[1:] // Skip 'T' if no days
		}

		// Parse hours
		if hIndex := strings.Index(remaining, "H"); hIndex >= 0 {
			hoursStr := remaining[:hIndex]
			if hours := parseInt(hoursStr); hours > 0 {
				result += time.Duration(hours) * time.Hour
			}
			remaining = remaining[hIndex+1:]
		}

		// Parse minutes
		if mIndex := strings.Index(remaining, "M"); mIndex >= 0 {
			minutesStr := remaining[:mIndex]
			if minutes := parseInt(minutesStr); minutes > 0 {
				result += time.Duration(minutes) * time.Minute
			}
			remaining = remaining[mIndex+1:]
		}

		// Parse seconds
		if sIndex := strings.Index(remaining, "S"); sIndex >= 0 {
			secondsStr := remaining[:sIndex]
			if seconds := parseInt(secondsStr); seconds > 0 {
				result += time.Duration(seconds) * time.Second
			}
		}
	} else {
		// Handle simple formats: PT15M, PT1H, etc.
		switch duration {
		case "PT15M":
			result = 15 * time.Minute
		case "PT5M":
			result = 5 * time.Minute
		case "PT10M":
			result = 10 * time.Minute
		case "PT30M":
			result = 30 * time.Minute
		case "PT1H":
			result = 1 * time.Hour
		case "PT2H":
			result = 2 * time.Hour
		default:
			return 0, fmt.Errorf("unsupported duration format: %s", duration)
		}
	}

	if negative {
		result = -result
	}
	return result, nil
}

// extractResponseStatusFromAttendees extracts the user's response status from ATTENDEE properties
func extractResponseStatusFromAttendees(event *ics.VEvent, userEmail string) string {
	// Get all ATTENDEE properties
	attendees := event.GetProperties(ics.ComponentPropertyAttendee)
	if len(attendees) == 0 {
		// No attendees means this is likely an event the user created
		// or a calendar without attendee tracking - treat as accepted
		return ""
	}

	// If no user email provided, we can't identify the user
	if userEmail == "" {
		return ""
	}

	// Normalize user email for comparison
	normalizedUserEmail := strings.ToLower(strings.TrimSpace(userEmail))

	// Look for the attendee matching the user's email
	for _, attendee := range attendees {
		// ATTENDEE value is typically "mailto:email@example.com"
		attendeeValue := strings.ToLower(strings.TrimSpace(attendee.Value))
		attendeeEmail := strings.TrimPrefix(attendeeValue, "mailto:")

		if attendeeEmail == normalizedUserEmail {
			// Found the user - extract PARTSTAT parameter
			// PARTSTAT values: NEEDS-ACTION, ACCEPTED, DECLINED, TENTATIVE, DELEGATED
			partstat := attendee.ICalParameters["PARTSTAT"]
			if len(partstat) > 0 {
				status := strings.ToLower(partstat[0])
				// Map iCal PARTSTAT values to our internal format
				switch status {
				case "accepted":
					return "accepted"
				case "declined":
					return "declined"
				case "tentative":
					return "tentative"
				case "needs-action":
					return "needsAction"
				default:
					return status
				}
			}
			// Found attendee but no PARTSTAT - assume needsAction
			return "needsAction"
		}
	}

	// User not found in attendee list - might be viewing someone else's calendar
	// or calendar doesn't track this user - treat as accepted
	return ""
}

// parseInt is a simple helper to parse integers, returning 0 if invalid
func parseInt(s string) int {
	if s == "" {
		return 0
	}
	// Simple conversion - could use strconv.Atoi but this handles edge cases
	var result int
	for _, r := range s {
		if r >= '0' && r <= '9' {
			result = result*10 + int(r-'0')
		} else {
			return 0 // Invalid character
		}
	}
	return result
}
package models

import (
	"time"
)

// Event represents a calendar event with all necessary fields
type Event struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Description    string    `json:"description,omitempty"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	Alarms         []Alarm   `json:"alarms,omitempty"`
	CalendarID     string    `json:"calendar_id"`
	CalendarName   string    `json:"calendar_name"`
	Location       string    `json:"location,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	ModifiedAt     time.Time `json:"modified_at"`
	ResponseStatus string    `json:"response_status,omitempty"` // accepted, declined, tentative, needsAction
}

// Alarm represents a notification trigger for an event
type Alarm struct {
	LeadTimeMinutes int    `json:"lead_time_minutes"`
	Severity        string `json:"severity,omitempty"`
	Method          string `json:"method,omitempty"` // email, popup, etc.
}

// Notification represents the message format sent to NATS
// This matches the expected format for calendar-siren consumer
type Notification struct {
	Title    string    `json:"title"`
	When     time.Time `json:"when"`
	Lead     int       `json:"lead"`
	Severity string    `json:"severity,omitempty"`
}

// NewNotification creates a Notification from an Event and Alarm
func NewNotification(event *Event, alarm *Alarm) *Notification {
	severity := alarm.Severity
	if severity == "" {
		severity = "normal"
	}

	return &Notification{
		Title:    event.Title,
		When:     event.StartTime,
		Lead:     alarm.LeadTimeMinutes,
		Severity: severity,
	}
}

// HasAlarms returns true if the event has any configured alarms
func (e *Event) HasAlarms() bool {
	return len(e.Alarms) > 0
}

// IsUpcoming returns true if the event starts after the given time
func (e *Event) IsUpcoming(now time.Time) bool {
	return e.StartTime.After(now)
}

// ShouldNotify determines if a notification should be sent for this event
// based on the given alarm and current time
func (e *Event) ShouldNotify(alarm *Alarm, now time.Time) bool {
	if !e.IsUpcoming(now) {
		return false
	}

	notificationTime := e.StartTime.Add(-time.Duration(alarm.LeadTimeMinutes) * time.Minute)
	return now.After(notificationTime) || now.Equal(notificationTime)
}

// IsAccepted returns true if the user has accepted the event invitation
// Empty response status is treated as accepted for backward compatibility
func (e *Event) IsAccepted() bool {
	// Empty status means we don't have response info - treat as accepted for backward compatibility
	// This handles events from calendars without attendee info or meetings you organized
	if e.ResponseStatus == "" {
		return true
	}
	return e.ResponseStatus == "accepted"
}
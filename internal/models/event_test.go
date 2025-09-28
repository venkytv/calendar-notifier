package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEvent_HasAlarms(t *testing.T) {
	// Event without alarms
	event := &Event{
		ID:    "test-event",
		Title: "Test Event",
	}
	if event.HasAlarms() {
		t.Error("Expected event without alarms to return false")
	}

	// Event with alarms
	event.Alarms = []Alarm{
		{LeadTimeMinutes: 10},
	}
	if !event.HasAlarms() {
		t.Error("Expected event with alarms to return true")
	}

	// Event with empty alarms slice
	event.Alarms = []Alarm{}
	if event.HasAlarms() {
		t.Error("Expected event with empty alarms slice to return false")
	}
}

func TestEvent_IsUpcoming(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	// Event in the future
	futureEvent := &Event{
		StartTime: time.Date(2025, 1, 1, 13, 0, 0, 0, time.UTC),
	}
	if !futureEvent.IsUpcoming(now) {
		t.Error("Expected future event to be upcoming")
	}

	// Event in the past
	pastEvent := &Event{
		StartTime: time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC),
	}
	if pastEvent.IsUpcoming(now) {
		t.Error("Expected past event to not be upcoming")
	}

	// Event at the exact same time
	currentEvent := &Event{
		StartTime: now,
	}
	if currentEvent.IsUpcoming(now) {
		t.Error("Expected current event to not be upcoming (should be exactly now)")
	}
}

func TestEvent_ShouldNotify(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	// Event starting at 13:00 with 30-minute alarm
	event := &Event{
		StartTime: time.Date(2025, 1, 1, 13, 0, 0, 0, time.UTC),
	}
	alarm := &Alarm{
		LeadTimeMinutes: 30,
	}

	// At 12:30 (exactly notification time), should notify
	notifyTime := time.Date(2025, 1, 1, 12, 30, 0, 0, time.UTC)
	if !event.ShouldNotify(alarm, notifyTime) {
		t.Error("Expected to notify at exact notification time")
	}

	// At 12:31 (after notification time), should notify
	afterTime := time.Date(2025, 1, 1, 12, 31, 0, 0, time.UTC)
	if !event.ShouldNotify(alarm, afterTime) {
		t.Error("Expected to notify after notification time")
	}

	// At 12:29 (before notification time), should not notify
	beforeTime := time.Date(2025, 1, 1, 12, 29, 0, 0, time.UTC)
	if event.ShouldNotify(alarm, beforeTime) {
		t.Error("Expected not to notify before notification time")
	}

	// Past event should not notify
	pastEvent := &Event{
		StartTime: time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC),
	}
	if pastEvent.ShouldNotify(alarm, now) {
		t.Error("Expected not to notify for past event")
	}

	// Test with different lead times
	longAlarm := &Alarm{
		LeadTimeMinutes: 120, // 2 hours
	}

	// At 11:00 (exactly 2 hours before), should notify
	longNotifyTime := time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC)
	if !event.ShouldNotify(longAlarm, longNotifyTime) {
		t.Error("Expected to notify with long lead time")
	}
}

func TestAlarm_JSONSerialization(t *testing.T) {
	alarm := Alarm{
		LeadTimeMinutes: 15,
		Severity:        "high",
		Method:          "popup",
	}

	data, err := json.Marshal(alarm)
	if err != nil {
		t.Fatalf("Failed to marshal alarm: %v", err)
	}

	var unmarshaledAlarm Alarm
	err = json.Unmarshal(data, &unmarshaledAlarm)
	if err != nil {
		t.Fatalf("Failed to unmarshal alarm: %v", err)
	}

	if unmarshaledAlarm.LeadTimeMinutes != alarm.LeadTimeMinutes {
		t.Errorf("Expected LeadTimeMinutes %d, got %d", alarm.LeadTimeMinutes, unmarshaledAlarm.LeadTimeMinutes)
	}
	if unmarshaledAlarm.Severity != alarm.Severity {
		t.Errorf("Expected Severity %s, got %s", alarm.Severity, unmarshaledAlarm.Severity)
	}
	if unmarshaledAlarm.Method != alarm.Method {
		t.Errorf("Expected Method %s, got %s", alarm.Method, unmarshaledAlarm.Method)
	}
}

func TestEvent_JSONSerialization(t *testing.T) {
	startTime := time.Date(2025, 1, 1, 14, 0, 0, 0, time.UTC)
	endTime := time.Date(2025, 1, 1, 15, 0, 0, 0, time.UTC)
	createdAt := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	modifiedAt := time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC)

	event := Event{
		ID:           "test-event-123",
		Title:        "Test Meeting",
		Description:  "A test meeting description",
		StartTime:    startTime,
		EndTime:      endTime,
		CalendarID:   "cal-123",
		CalendarName: "My Calendar",
		Location:     "Conference Room A",
		CreatedAt:    createdAt,
		ModifiedAt:   modifiedAt,
		Alarms: []Alarm{
			{LeadTimeMinutes: 10, Severity: "normal"},
			{LeadTimeMinutes: 30, Severity: "high"},
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	var unmarshaledEvent Event
	err = json.Unmarshal(data, &unmarshaledEvent)
	if err != nil {
		t.Fatalf("Failed to unmarshal event: %v", err)
	}

	// Check all fields
	if unmarshaledEvent.ID != event.ID {
		t.Errorf("Expected ID %s, got %s", event.ID, unmarshaledEvent.ID)
	}
	if unmarshaledEvent.Title != event.Title {
		t.Errorf("Expected Title %s, got %s", event.Title, unmarshaledEvent.Title)
	}
	if unmarshaledEvent.Description != event.Description {
		t.Errorf("Expected Description %s, got %s", event.Description, unmarshaledEvent.Description)
	}
	if !unmarshaledEvent.StartTime.Equal(event.StartTime) {
		t.Errorf("Expected StartTime %v, got %v", event.StartTime, unmarshaledEvent.StartTime)
	}
	if !unmarshaledEvent.EndTime.Equal(event.EndTime) {
		t.Errorf("Expected EndTime %v, got %v", event.EndTime, unmarshaledEvent.EndTime)
	}
	if len(unmarshaledEvent.Alarms) != len(event.Alarms) {
		t.Errorf("Expected %d alarms, got %d", len(event.Alarms), len(unmarshaledEvent.Alarms))
	}
}

func TestNewNotification(t *testing.T) {
	startTime := time.Date(2025, 1, 1, 14, 0, 0, 0, time.UTC)
	event := &Event{
		Title:     "Test Meeting",
		StartTime: startTime,
	}

	// Test with alarm that has severity
	alarm := &Alarm{
		LeadTimeMinutes: 15,
		Severity:        "high",
	}

	notification := NewNotification(event, alarm)
	if notification.Title != event.Title {
		t.Errorf("Expected title %s, got %s", event.Title, notification.Title)
	}
	if !notification.When.Equal(event.StartTime) {
		t.Errorf("Expected when %v, got %v", event.StartTime, notification.When)
	}
	if notification.Lead != alarm.LeadTimeMinutes {
		t.Errorf("Expected lead %d, got %d", alarm.LeadTimeMinutes, notification.Lead)
	}
	if notification.Severity != alarm.Severity {
		t.Errorf("Expected severity %s, got %s", alarm.Severity, notification.Severity)
	}

	// Test with alarm without severity (should default to "normal")
	alarmNoSeverity := &Alarm{
		LeadTimeMinutes: 10,
	}

	notificationDefault := NewNotification(event, alarmNoSeverity)
	if notificationDefault.Severity != "normal" {
		t.Errorf("Expected default severity 'normal', got %s", notificationDefault.Severity)
	}
}

func TestNotification_JSONSerialization(t *testing.T) {
	when := time.Date(2025, 1, 1, 14, 0, 0, 0, time.UTC)
	notification := Notification{
		Title:    "Test Meeting",
		When:     when,
		Lead:     15,
		Severity: "high",
	}

	data, err := json.Marshal(notification)
	if err != nil {
		t.Fatalf("Failed to marshal notification: %v", err)
	}

	var unmarshaledNotification Notification
	err = json.Unmarshal(data, &unmarshaledNotification)
	if err != nil {
		t.Fatalf("Failed to unmarshal notification: %v", err)
	}

	if unmarshaledNotification.Title != notification.Title {
		t.Errorf("Expected Title %s, got %s", notification.Title, unmarshaledNotification.Title)
	}
	if !unmarshaledNotification.When.Equal(notification.When) {
		t.Errorf("Expected When %v, got %v", notification.When, unmarshaledNotification.When)
	}
	if unmarshaledNotification.Lead != notification.Lead {
		t.Errorf("Expected Lead %d, got %d", notification.Lead, unmarshaledNotification.Lead)
	}
	if unmarshaledNotification.Severity != notification.Severity {
		t.Errorf("Expected Severity %s, got %s", notification.Severity, unmarshaledNotification.Severity)
	}
}

func TestNotification_MatchesNATSContract(t *testing.T) {
	// Test that the notification structure matches the expected NATS message format
	expectedJSON := `{
		"title": "Meeting Title",
		"when": "2025-09-25T14:00:00Z",
		"lead": 10,
		"severity": "normal"
	}`

	var expectedNotification Notification
	err := json.Unmarshal([]byte(expectedJSON), &expectedNotification)
	if err != nil {
		t.Fatalf("Failed to unmarshal expected JSON: %v", err)
	}

	// Create a notification and verify it matches the expected structure
	when, _ := time.Parse(time.RFC3339, "2025-09-25T14:00:00Z")
	notification := Notification{
		Title:    "Meeting Title",
		When:     when,
		Lead:     10,
		Severity: "normal",
	}

	actualJSON, err := json.Marshal(notification)
	if err != nil {
		t.Fatalf("Failed to marshal notification: %v", err)
	}

	// Parse back to ensure structure is correct
	var parsedNotification Notification
	err = json.Unmarshal(actualJSON, &parsedNotification)
	if err != nil {
		t.Fatalf("Failed to unmarshal actual JSON: %v", err)
	}

	// Verify all required fields are present
	if parsedNotification.Title != expectedNotification.Title {
		t.Errorf("Expected Title %s, got %s", expectedNotification.Title, parsedNotification.Title)
	}
	if !parsedNotification.When.Equal(expectedNotification.When) {
		t.Errorf("Expected When %v, got %v", expectedNotification.When, parsedNotification.When)
	}
	if parsedNotification.Lead != expectedNotification.Lead {
		t.Errorf("Expected Lead %d, got %d", expectedNotification.Lead, parsedNotification.Lead)
	}
	if parsedNotification.Severity != expectedNotification.Severity {
		t.Errorf("Expected Severity %s, got %s", expectedNotification.Severity, parsedNotification.Severity)
	}
}
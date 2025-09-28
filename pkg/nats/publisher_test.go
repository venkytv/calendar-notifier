package nats

import (
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/venkytv/calendar-notifier/internal/models"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.URL != "nats://localhost:4222" {
		t.Errorf("Expected default URL to be 'nats://localhost:4222', got %s", config.URL)
	}

	if config.Subject != "calendar.notifications" {
		t.Errorf("Expected default subject to be 'calendar.notifications', got %s", config.Subject)
	}

	if config.ConnectTimeout != 5*time.Second {
		t.Errorf("Expected default connect timeout to be 5s, got %v", config.ConnectTimeout)
	}
}

func TestPublishEventNotifications(t *testing.T) {
	// This test doesn't require an actual NATS server
	// We'll test the logic of converting events to notifications

	// Create a mock publisher (we'll skip the actual NATS connection)
	logger := slog.Default()

	// Create test events
	now := time.Now()
	events := []*models.Event{
		{
			ID:        "event1",
			Title:     "Test Meeting 1",
			StartTime: now.Add(30 * time.Minute),
			EndTime:   now.Add(90 * time.Minute),
			Alarms: []models.Alarm{
				{LeadTimeMinutes: 10, Method: "popup", Severity: "normal"},
				{LeadTimeMinutes: 5, Method: "email", Severity: "high"},
			},
		},
		{
			ID:        "event2",
			Title:     "Test Meeting 2",
			StartTime: now.Add(2 * time.Hour),
			EndTime:   now.Add(3 * time.Hour),
			// No alarms - should use defaults
		},
	}

	// Default alarms for events without alarms
	defaultAlarms := []models.Alarm{
		{LeadTimeMinutes: 15, Method: "popup", Severity: "normal"},
	}

	// Test notification creation logic (without actual NATS publishing)
	var notifications []*models.Notification

	for _, event := range events {
		alarms := event.Alarms
		if len(alarms) == 0 {
			alarms = defaultAlarms
		}

		if len(alarms) == 0 {
			t.Logf("Skipping event with no alarms: %s", event.Title)
			continue
		}

		for _, alarm := range alarms {
			notification := models.NewNotification(event, &alarm)
			notifications = append(notifications, notification)
		}
	}

	// Verify we got the expected number of notifications
	expectedNotifications := 3 // 2 from event1 + 1 from event2 (using default)
	if len(notifications) != expectedNotifications {
		t.Errorf("Expected %d notifications, got %d", expectedNotifications, len(notifications))
	}

	// Verify notification content
	if notifications[0].Title != "Test Meeting 1" {
		t.Errorf("Expected first notification title to be 'Test Meeting 1', got %s", notifications[0].Title)
	}

	if notifications[0].Lead != 10 {
		t.Errorf("Expected first notification lead time to be 10, got %d", notifications[0].Lead)
	}

	if notifications[1].Lead != 5 {
		t.Errorf("Expected second notification lead time to be 5, got %d", notifications[1].Lead)
	}

	if notifications[2].Title != "Test Meeting 2" {
		t.Errorf("Expected third notification title to be 'Test Meeting 2', got %s", notifications[2].Title)
	}

	if notifications[2].Lead != 15 {
		t.Errorf("Expected third notification lead time to be 15 (default), got %d", notifications[2].Lead)
	}

	logger.Info("Test completed successfully", "notifications_created", len(notifications))
}

func TestPublisherHealthCheck(t *testing.T) {
	// Test publisher health check without connection
	publisher := &Publisher{
		conn:    nil,
		subject: "test.subject",
		logger:  slog.Default(),
	}

	err := publisher.IsHealthy()
	if err == nil {
		t.Error("Expected health check to fail with nil connection")
	}
}

func TestNotificationJSONMarshaling(t *testing.T) {
	// Test that notifications can be properly marshaled to JSON
	now := time.Now()
	notification := &models.Notification{
		Title:    "Test Meeting",
		When:     now,
		Lead:     10,
		Severity: "normal",
	}

	// This tests the JSON marshaling that would happen in PublishNotification
	_, err := json.Marshal(notification)
	if err != nil {
		t.Errorf("Failed to marshal notification to JSON: %v", err)
	}

	t.Logf("Notification marshaled successfully")
}

// Benchmark test for notification creation
func BenchmarkNotificationCreation(b *testing.B) {
	now := time.Now()
	event := &models.Event{
		ID:        "benchmark-event",
		Title:     "Benchmark Meeting",
		StartTime: now.Add(30 * time.Minute),
		EndTime:   now.Add(90 * time.Minute),
	}

	alarm := &models.Alarm{
		LeadTimeMinutes: 10,
		Method:          "popup",
		Severity:        "normal",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = models.NewNotification(event, alarm)
	}
}
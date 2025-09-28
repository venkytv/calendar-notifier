package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/venkytv/calendar-notifier/internal/models"
)

// MockCalendarManager is a mock implementation for testing
type MockCalendarManager struct {
	events []*models.Event
	err    error
}

func (m *MockCalendarManager) GetAllEvents(ctx context.Context, from, to time.Time) ([]*models.Event, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.events, nil
}

func (m *MockCalendarManager) Close() error {
	return nil
}

// MockPublisher is a mock NATS publisher for testing
type MockPublisher struct {
	published []*models.Notification
	err       error
}

func (m *MockPublisher) PublishNotification(ctx context.Context, notification *models.Notification) error {
	if m.err != nil {
		return m.err
	}
	m.published = append(m.published, notification)
	return nil
}

func (m *MockPublisher) Close() error {
	return nil
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.PollInterval != 5*time.Minute {
		t.Errorf("Expected default poll interval to be 5m, got %v", config.PollInterval)
	}

	if config.LookaheadWindow != 24*time.Hour {
		t.Errorf("Expected default lookahead window to be 24h, got %v", config.LookaheadWindow)
	}

	if len(config.DefaultLeadTimes) != 2 {
		t.Errorf("Expected 2 default lead times, got %d", len(config.DefaultLeadTimes))
	}

	if config.DefaultLeadTimes[0] != 15 || config.DefaultLeadTimes[1] != 5 {
		t.Errorf("Expected default lead times [15, 5], got %v", config.DefaultLeadTimes)
	}
}

func TestNewEventScheduler(t *testing.T) {
	mockCalendarManager := &MockCalendarManager{}
	mockPublisher := &MockPublisher{}
	logger := slog.Default()

	scheduler := NewEventScheduler(nil, mockCalendarManager, mockPublisher, logger)

	if scheduler == nil {
		t.Fatal("Expected scheduler to be created")
	}

	if scheduler.config == nil {
		t.Error("Expected default config to be set")
	}

	if scheduler.scheduledEvents == nil {
		t.Error("Expected scheduled events map to be initialized")
	}

	if scheduler.running {
		t.Error("Expected scheduler to not be running initially")
	}
}

func TestScheduleEventNotifications(t *testing.T) {
	mockCalendarManager := &MockCalendarManager{}
	mockPublisher := &MockPublisher{}
	logger := slog.Default()

	scheduler := NewEventScheduler(nil, mockCalendarManager, mockPublisher, logger)

	// Create a test event starting in 1 hour
	now := time.Now()
	event := &models.Event{
		ID:        "test-event-1",
		Title:     "Test Meeting",
		StartTime: now.Add(1 * time.Hour),
		EndTime:   now.Add(2 * time.Hour),
		Alarms: []models.Alarm{
			{LeadTimeMinutes: 15, Method: "popup", Severity: "normal"},
			{LeadTimeMinutes: 5, Method: "email", Severity: "high"},
		},
	}

	// Schedule the event
	scheduler.scheduleEventNotifications(event)

	// Check that event was scheduled
	scheduledEvents := scheduler.GetScheduledEvents()
	if len(scheduledEvents) != 1 {
		t.Errorf("Expected 1 scheduled event, got %d", len(scheduledEvents))
	}

	scheduledEvent := scheduledEvents["test-event-1"]
	if scheduledEvent == nil {
		t.Fatal("Expected scheduled event to exist")
	}

	if len(scheduledEvent.Notifications) != 2 {
		t.Errorf("Expected 2 notifications, got %d", len(scheduledEvent.Notifications))
	}

	// Check notification timing
	for i, notification := range scheduledEvent.Notifications {
		expectedLeadTime := event.Alarms[i].LeadTimeMinutes
		if notification.Notification.Lead != expectedLeadTime {
			t.Errorf("Expected lead time %d, got %d", expectedLeadTime, notification.Notification.Lead)
		}

		expectedTriggerTime := event.StartTime.Add(-time.Duration(expectedLeadTime) * time.Minute)
		if !notification.TriggerTime.Equal(expectedTriggerTime) {
			t.Errorf("Expected trigger time %v, got %v", expectedTriggerTime, notification.TriggerTime)
		}
	}
}

func TestScheduleEventWithoutAlarms(t *testing.T) {
	config := &Config{
		PollInterval:        1 * time.Minute,
		LookaheadWindow:     4 * time.Hour,
		DefaultLeadTimes:    []int{10, 2},
		MaxConcurrentEvents: 100,
		TimerBufferSize:     10,
	}

	mockCalendarManager := &MockCalendarManager{}
	mockPublisher := &MockPublisher{}
	logger := slog.Default()

	scheduler := NewEventScheduler(config, mockCalendarManager, mockPublisher, logger)

	// Create a test event without alarms
	now := time.Now()
	event := &models.Event{
		ID:        "test-event-no-alarms",
		Title:     "Meeting Without Alarms",
		StartTime: now.Add(2 * time.Hour),
		EndTime:   now.Add(3 * time.Hour),
		Alarms:    []models.Alarm{}, // No alarms
	}

	// Schedule the event
	scheduler.scheduleEventNotifications(event)

	// Check that event was scheduled with default alarms
	scheduledEvents := scheduler.GetScheduledEvents()
	scheduledEvent := scheduledEvents["test-event-no-alarms"]
	if scheduledEvent == nil {
		t.Fatal("Expected scheduled event to exist")
	}

	if len(scheduledEvent.Notifications) != 2 {
		t.Errorf("Expected 2 notifications (from defaults), got %d", len(scheduledEvent.Notifications))
	}

	// Verify default lead times were used
	expectedLeadTimes := []int{10, 2}
	for i, notification := range scheduledEvent.Notifications {
		if notification.Notification.Lead != expectedLeadTimes[i] {
			t.Errorf("Expected default lead time %d, got %d", expectedLeadTimes[i], notification.Notification.Lead)
		}
	}
}

func TestSchedulerStats(t *testing.T) {
	mockCalendarManager := &MockCalendarManager{}
	mockPublisher := &MockPublisher{}
	logger := slog.Default()

	scheduler := NewEventScheduler(nil, mockCalendarManager, mockPublisher, logger)

	// Initially no stats
	stats := scheduler.GetStats()
	if stats.TotalEvents != 0 {
		t.Errorf("Expected 0 total events, got %d", stats.TotalEvents)
	}
	if stats.PendingNotifications != 0 {
		t.Errorf("Expected 0 pending notifications, got %d", stats.PendingNotifications)
	}
	if stats.IsRunning != false {
		t.Error("Expected scheduler to not be running")
	}

	// Add a test event
	now := time.Now()
	event := &models.Event{
		ID:        "stats-test",
		Title:     "Stats Test Meeting",
		StartTime: now.Add(30 * time.Minute),
		EndTime:   now.Add(90 * time.Minute),
		Alarms: []models.Alarm{
			{LeadTimeMinutes: 15, Method: "popup", Severity: "normal"},
		},
	}

	scheduler.scheduleEventNotifications(event)

	// Check updated stats
	stats = scheduler.GetStats()
	if stats.TotalEvents != 1 {
		t.Errorf("Expected 1 total event, got %d", stats.TotalEvents)
	}
	if stats.PendingNotifications != 1 {
		t.Errorf("Expected 1 pending notification, got %d", stats.PendingNotifications)
	}
}

func TestPastEventHandling(t *testing.T) {
	mockCalendarManager := &MockCalendarManager{}
	mockPublisher := &MockPublisher{}
	logger := slog.Default()

	scheduler := NewEventScheduler(nil, mockCalendarManager, mockPublisher, logger)

	// Create a past event
	now := time.Now()
	pastEvent := &models.Event{
		ID:        "past-event",
		Title:     "Past Meeting",
		StartTime: now.Add(-2 * time.Hour),
		EndTime:   now.Add(-1 * time.Hour),
		Alarms: []models.Alarm{
			{LeadTimeMinutes: 15, Method: "popup", Severity: "normal"},
		},
	}

	// Schedule the past event (should be skipped)
	scheduler.scheduleEventNotifications(pastEvent)

	// Check that no events were scheduled
	scheduledEvents := scheduler.GetScheduledEvents()
	if len(scheduledEvents) != 0 {
		t.Errorf("Expected 0 scheduled events for past event, got %d", len(scheduledEvents))
	}
}

func TestCleanupOldEvents(t *testing.T) {
	mockCalendarManager := &MockCalendarManager{}
	mockPublisher := &MockPublisher{}
	logger := slog.Default()

	scheduler := NewEventScheduler(nil, mockCalendarManager, mockPublisher, logger)

	now := time.Now()

	// Add an old event (ended more than 24 hours ago)
	oldEvent := &models.Event{
		ID:        "old-event",
		Title:     "Old Meeting",
		StartTime: now.Add(-30 * time.Hour),
		EndTime:   now.Add(-26 * time.Hour), // Ended 26 hours ago
		Alarms: []models.Alarm{
			{LeadTimeMinutes: 15, Method: "popup", Severity: "normal"},
		},
	}

	// Add a recent event
	recentEvent := &models.Event{
		ID:        "recent-event",
		Title:     "Recent Meeting",
		StartTime: now.Add(-2 * time.Hour),
		EndTime:   now.Add(-1 * time.Hour), // Ended 1 hour ago
		Alarms: []models.Alarm{
			{LeadTimeMinutes: 15, Method: "popup", Severity: "normal"},
		},
	}

	// Force schedule them (bypass the upcoming check for testing)
	scheduler.mu.Lock()
	scheduler.scheduledEvents["old-event"] = &ScheduledEvent{
		Event:       oldEvent,
		LastUpdated: now,
	}
	scheduler.scheduledEvents["recent-event"] = &ScheduledEvent{
		Event:       recentEvent,
		LastUpdated: now,
	}
	scheduler.mu.Unlock()

	// Verify both events are present
	scheduledEvents := scheduler.GetScheduledEvents()
	if len(scheduledEvents) != 2 {
		t.Errorf("Expected 2 scheduled events before cleanup, got %d", len(scheduledEvents))
	}

	// Run cleanup
	scheduler.CleanupOldEvents()

	// Verify only recent event remains
	scheduledEvents = scheduler.GetScheduledEvents()
	if len(scheduledEvents) != 1 {
		t.Errorf("Expected 1 scheduled event after cleanup, got %d", len(scheduledEvents))
	}

	if _, exists := scheduledEvents["recent-event"]; !exists {
		t.Error("Expected recent event to remain after cleanup")
	}

	if _, exists := scheduledEvents["old-event"]; exists {
		t.Error("Expected old event to be removed after cleanup")
	}
}

// Benchmark test for event scheduling
func BenchmarkScheduleEventNotifications(b *testing.B) {
	mockCalendarManager := &MockCalendarManager{}
	mockPublisher := &MockPublisher{}
	logger := slog.Default()

	scheduler := NewEventScheduler(nil, mockCalendarManager, mockPublisher, logger)

	now := time.Now()
	event := &models.Event{
		ID:        "benchmark-event",
		Title:     "Benchmark Meeting",
		StartTime: now.Add(1 * time.Hour),
		EndTime:   now.Add(2 * time.Hour),
		Alarms: []models.Alarm{
			{LeadTimeMinutes: 15, Method: "popup", Severity: "normal"},
			{LeadTimeMinutes: 5, Method: "email", Severity: "high"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		event.ID = fmt.Sprintf("benchmark-event-%d", i)
		scheduler.scheduleEventNotifications(event)
	}
}

func TestSchedulerStartStop(t *testing.T) {
	mockCalendarManager := &MockCalendarManager{
		events: []*models.Event{},
	}
	mockPublisher := &MockPublisher{}
	logger := slog.Default()

	// Use short intervals for testing
	config := &Config{
		PollInterval:        100 * time.Millisecond,
		LookaheadWindow:     1 * time.Hour,
		DefaultLeadTimes:    []int{5},
		MaxConcurrentEvents: 10,
		TimerBufferSize:     5,
	}

	scheduler := NewEventScheduler(config, mockCalendarManager, mockPublisher, logger)

	// Test starting
	err := scheduler.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// Verify it's running
	stats := scheduler.GetStats()
	if !stats.IsRunning {
		t.Error("Expected scheduler to be running after start")
	}

	// Wait a bit to let it poll
	time.Sleep(200 * time.Millisecond)

	// Test stopping
	err = scheduler.Stop()
	if err != nil {
		t.Fatalf("Failed to stop scheduler: %v", err)
	}

	// Verify it's stopped
	stats = scheduler.GetStats()
	if stats.IsRunning {
		t.Error("Expected scheduler to be stopped after stop")
	}
}
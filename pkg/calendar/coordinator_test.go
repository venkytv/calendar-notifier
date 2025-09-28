package calendar

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/venkytv/calendar-notifier/internal/models"
)

func TestDefaultCoordinatorConfig(t *testing.T) {
	config := DefaultCoordinatorConfig()

	if !config.DeduplicationEnabled {
		t.Error("Expected deduplication to be enabled by default")
	}

	if config.DeduplicationWindow != 5*time.Minute {
		t.Errorf("Expected default deduplication window to be 5 minutes, got %v", config.DeduplicationWindow)
	}

	if len(config.PriorityProviders) == 0 {
		t.Error("Expected some priority providers to be configured")
	}

	if config.ProviderPriorities["caldav"] != 1 {
		t.Errorf("Expected CalDAV to have priority 1, got %d", config.ProviderPriorities["caldav"])
	}
}

func TestEventCoordinatorBasicFunctionality(t *testing.T) {
	coordinator := NewEventCoordinator(nil, slog.Default())

	if coordinator == nil {
		t.Fatal("Expected coordinator to be created")
	}

	if coordinator.config == nil {
		t.Error("Expected default config to be set")
	}
}

func TestCoordinateEventsNoDuplicates(t *testing.T) {
	coordinator := NewEventCoordinator(nil, slog.Default())

	now := time.Now()
	events := []*models.Event{
		{
			ID:           "event1",
			Title:        "Meeting 1",
			StartTime:    now.Add(1 * time.Hour),
			EndTime:      now.Add(2 * time.Hour),
			CalendarName: "caldav",
		},
		{
			ID:           "event2",
			Title:        "Meeting 2",
			StartTime:    now.Add(3 * time.Hour),
			EndTime:      now.Add(4 * time.Hour),
			CalendarName: "outlook",
		},
	}

	coordinated, err := coordinator.CoordinateEvents(events)
	if err != nil {
		t.Fatalf("Failed to coordinate events: %v", err)
	}

	if len(coordinated) != 2 {
		t.Errorf("Expected 2 coordinated events, got %d", len(coordinated))
	}

	// Should be sorted by start time
	if !coordinated[0].StartTime.Before(coordinated[1].StartTime) {
		t.Error("Expected events to be sorted by start time")
	}
}

func TestDeduplicateEventsByTitle(t *testing.T) {
	coordinator := NewEventCoordinator(nil, slog.Default())

	now := time.Now()
	events := []*models.Event{
		{
			ID:           "caldav-event1",
			Title:        "Team Meeting",
			StartTime:    now.Add(1 * time.Hour),
			EndTime:      now.Add(2 * time.Hour),
			CalendarName: "caldav",
			Alarms: []models.Alarm{
				{LeadTimeMinutes: 15, Method: "popup"},
			},
		},
		{
			ID:           "outlook-event1",
			Title:        "Team Meeting", // Same title
			StartTime:    now.Add(1*time.Hour + 2*time.Minute), // Slightly different time but within window
			EndTime:      now.Add(2*time.Hour + 2*time.Minute),
			CalendarName: "outlook",
			Alarms: []models.Alarm{
				{LeadTimeMinutes: 10, Method: "email"},
			},
		},
	}

	coordinated, err := coordinator.CoordinateEvents(events)
	if err != nil {
		t.Fatalf("Failed to coordinate events: %v", err)
	}

	if len(coordinated) != 1 {
		t.Errorf("Expected 1 coordinated event (duplicates merged), got %d", len(coordinated))
	}

	merged := coordinated[0]
	if merged.CalendarName != "caldav" { // CalDAV has higher priority
		t.Errorf("Expected merged event to use CalDAV calendar name, got %s", merged.CalendarName)
	}

	// Check that it has a merged ID
	if merged.ID != "merged-caldav-event1-outlook-event1" {
		t.Errorf("Expected merged ID, got %s", merged.ID)
	}
}

func TestDeduplicateEventsWithMergeAlarmsStrategy(t *testing.T) {
	config := &CoordinatorConfig{
		DeduplicationEnabled: true,
		DeduplicationWindow:  5 * time.Minute,
		ProviderPriorities: map[string]int{
			"caldav":  1,
			"outlook": 2,
		},
		MergeStrategies: map[string]string{
			"default": "merge_alarms",
		},
	}

	coordinator := NewEventCoordinator(config, slog.Default())

	now := time.Now()
	events := []*models.Event{
		{
			ID:           "caldav-event1",
			Title:        "Team Meeting",
			StartTime:    now.Add(1 * time.Hour),
			EndTime:      now.Add(2 * time.Hour),
			CalendarName: "caldav",
			Alarms: []models.Alarm{
				{LeadTimeMinutes: 15, Method: "popup", Severity: "normal"},
			},
		},
		{
			ID:           "outlook-event1",
			Title:        "Team Meeting",
			StartTime:    now.Add(1*time.Hour + 1*time.Minute),
			EndTime:      now.Add(2*time.Hour + 1*time.Minute),
			CalendarName: "outlook",
			Alarms: []models.Alarm{
				{LeadTimeMinutes: 10, Method: "email", Severity: "high"},
				{LeadTimeMinutes: 15, Method: "popup", Severity: "normal"}, // Duplicate
			},
		},
	}

	coordinated, err := coordinator.CoordinateEvents(events)
	if err != nil {
		t.Fatalf("Failed to coordinate events: %v", err)
	}

	if len(coordinated) != 1 {
		t.Errorf("Expected 1 coordinated event, got %d", len(coordinated))
	}

	merged := coordinated[0]
	if len(merged.Alarms) != 2 {
		t.Errorf("Expected 2 unique alarms after merging, got %d", len(merged.Alarms))
	}

	// Should be sorted by lead time (descending)
	if merged.Alarms[0].LeadTimeMinutes < merged.Alarms[1].LeadTimeMinutes {
		t.Error("Expected alarms to be sorted by lead time (descending)")
	}
}

func TestPrioritizeEventsByProvider(t *testing.T) {
	config := &CoordinatorConfig{
		ProviderPriorities: map[string]int{
			"caldav":  1, // Highest priority
			"outlook": 2,
			"apple":   3,
		},
	}

	coordinator := NewEventCoordinator(config, slog.Default())

	now := time.Now()
	events := []*models.Event{
		{
			ID:           "apple-event",
			Title:        "Apple Meeting",
			StartTime:    now.Add(1 * time.Hour),
			CalendarName: "apple",
		},
		{
			ID:           "caldav-event",
			Title:        "CalDAV Meeting",
			StartTime:    now.Add(1 * time.Hour),
			CalendarName: "caldav",
		},
		{
			ID:           "outlook-event",
			Title:        "Outlook Meeting",
			StartTime:    now.Add(1 * time.Hour),
			CalendarName: "outlook",
		},
	}

	coordinator.prioritizeEventsByProvider(events)

	// Should be sorted by priority: CalDAV (1), Outlook (2), Apple (3)
	if events[0].CalendarName != "caldav" {
		t.Errorf("Expected CalDAV event first, got %s", events[0].CalendarName)
	}
	if events[1].CalendarName != "outlook" {
		t.Errorf("Expected Outlook event second, got %s", events[1].CalendarName)
	}
	if events[2].CalendarName != "apple" {
		t.Errorf("Expected Apple event third, got %s", events[2].CalendarName)
	}
}

func TestAreTitlesSimilar(t *testing.T) {
	coordinator := NewEventCoordinator(nil, slog.Default())

	tests := []struct {
		titleA   string
		titleB   string
		expected bool
	}{
		{"Team Meeting", "Team Meeting", true},           // Exact match
		{"Team Meeting", "team meeting", true},           // Case insensitive
		{"Weekly Team Meeting", "Team Meeting", true},    // One contains the other
		{"Team Meeting", "Weekly Team Meeting", true},    // One contains the other (reverse)
		{"Daily Standup Meeting", "Daily Standup", true}, // Word overlap
		{"Meeting A", "Meeting B", false},                // Different meetings
		{"", "Meeting", false},                           // Empty string
		{"Meeting", "", false},                           // Empty string
		{"Short", "Long Meeting Title", false},           // No significant overlap
	}

	for _, test := range tests {
		result := coordinator.areTitlesSimilar(test.titleA, test.titleB)
		if result != test.expected {
			t.Errorf("areTitlesSimilar(%q, %q) = %v, expected %v",
				test.titleA, test.titleB, result, test.expected)
		}
	}
}

func TestAreEventsSimilar(t *testing.T) {
	coordinator := NewEventCoordinator(nil, slog.Default())

	now := time.Now()
	baseEvent := &models.Event{
		ID:        "event1",
		Title:     "Team Meeting",
		StartTime: now.Add(1 * time.Hour),
		EndTime:   now.Add(2 * time.Hour),
	}

	tests := []struct {
		name     string
		eventB   *models.Event
		expected bool
	}{
		{
			name:     "Same event",
			eventB:   baseEvent,
			expected: true,
		},
		{
			name: "Same ID",
			eventB: &models.Event{
				ID:        "event1", // Same ID
				Title:     "Different Title",
				StartTime: now.Add(2 * time.Hour), // Different time
			},
			expected: true,
		},
		{
			name: "Same title, close time",
			eventB: &models.Event{
				ID:        "event2",
				Title:     "Team Meeting", // Same title
				StartTime: now.Add(1*time.Hour + 2*time.Minute), // Within window
			},
			expected: true,
		},
		{
			name: "Same title, too far apart",
			eventB: &models.Event{
				ID:        "event3",
				Title:     "Team Meeting", // Same title
				StartTime: now.Add(1*time.Hour + 10*time.Minute), // Outside window
			},
			expected: false,
		},
		{
			name: "Different title and time",
			eventB: &models.Event{
				ID:        "event4",
				Title:     "Different Meeting",
				StartTime: now.Add(3 * time.Hour),
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := coordinator.areEventsSimilar(baseEvent, test.eventB)
			if result != test.expected {
				t.Errorf("areEventsSimilar() = %v, expected %v", result, test.expected)
			}
		})
	}
}

func TestGetCoordinationStats(t *testing.T) {
	coordinator := NewEventCoordinator(nil, slog.Default())

	now := time.Now()
	originalEvents := []*models.Event{
		{ID: "1", Title: "Meeting A", CalendarName: "caldav", StartTime: now},
		{ID: "2", Title: "Meeting A", CalendarName: "outlook", StartTime: now}, // Duplicate
		{ID: "3", Title: "Meeting B", CalendarName: "apple", StartTime: now},
	}

	coordinatedEvents := []*models.Event{
		{ID: "merged-1-2", Title: "Meeting A", CalendarName: "caldav", StartTime: now},
		{ID: "3", Title: "Meeting B", CalendarName: "apple", StartTime: now},
	}

	stats := coordinator.GetCoordinationStats(originalEvents, coordinatedEvents)

	if stats.OriginalCount != 3 {
		t.Errorf("Expected original count 3, got %d", stats.OriginalCount)
	}

	if stats.CoordinatedCount != 2 {
		t.Errorf("Expected coordinated count 2, got %d", stats.CoordinatedCount)
	}

	if stats.DuplicatesRemoved != 1 {
		t.Errorf("Expected 1 duplicate removed, got %d", stats.DuplicatesRemoved)
	}

	if stats.ProviderCounts["caldav"] != 1 {
		t.Errorf("Expected 1 CalDAV event, got %d", stats.ProviderCounts["caldav"])
	}

	if stats.ProviderCounts["apple"] != 1 {
		t.Errorf("Expected 1 Apple event, got %d", stats.ProviderCounts["apple"])
	}
}

func TestCoordinateEventsDisabledDeduplication(t *testing.T) {
	config := &CoordinatorConfig{
		DeduplicationEnabled: false, // Disabled
		ProviderPriorities: map[string]int{
			"caldav": 1,
		},
	}

	coordinator := NewEventCoordinator(config, slog.Default())

	now := time.Now()
	events := []*models.Event{
		{
			ID:           "event1",
			Title:        "Team Meeting",
			StartTime:    now.Add(1 * time.Hour),
			CalendarName: "caldav",
		},
		{
			ID:           "event2",
			Title:        "Team Meeting", // Same title (would be duplicate if enabled)
			StartTime:    now.Add(1*time.Hour + 1*time.Minute),
			CalendarName: "outlook",
		},
	}

	coordinated, err := coordinator.CoordinateEvents(events)
	if err != nil {
		t.Fatalf("Failed to coordinate events: %v", err)
	}

	// Should keep both events since deduplication is disabled
	if len(coordinated) != 2 {
		t.Errorf("Expected 2 events (no deduplication), got %d", len(coordinated))
	}
}

// Benchmark test for event coordination
func BenchmarkCoordinateEvents(b *testing.B) {
	coordinator := NewEventCoordinator(nil, slog.Default())

	now := time.Now()
	var events []*models.Event

	// Create a mix of events with some duplicates
	for i := 0; i < 100; i++ {
		events = append(events,
			&models.Event{
				ID:           fmt.Sprintf("event-%d", i),
				Title:        fmt.Sprintf("Meeting %d", i%20), // Some duplicate titles
				StartTime:    now.Add(time.Duration(i) * time.Minute),
				EndTime:      now.Add(time.Duration(i+60) * time.Minute),
				CalendarName: []string{"caldav", "outlook", "apple"}[i%3],
			})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := coordinator.CoordinateEvents(events)
		if err != nil {
			b.Fatal(err)
		}
	}
}
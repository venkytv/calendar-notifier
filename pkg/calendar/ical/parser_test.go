package ical

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	ics "github.com/arran4/golang-ical"
)

func TestParseICalDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{
			name:     "negative 15 minutes",
			input:    "-PT15M",
			expected: -15 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "negative 5 minutes",
			input:    "-PT5M",
			expected: -5 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "positive 10 minutes",
			input:    "PT10M",
			expected: 10 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "negative 30 minutes",
			input:    "-PT30M",
			expected: -30 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "negative 1 hour",
			input:    "-PT1H",
			expected: -1 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "positive 2 hours",
			input:    "PT2H",
			expected: 2 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "complex format: negative 0 days 0 hours 5 minutes 0 seconds",
			input:    "-P0DT0H5M0S",
			expected: -5 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "complex format: negative 0 days 1 hour 30 minutes 0 seconds",
			input:    "-P0DT1H30M0S",
			expected: -(1*time.Hour + 30*time.Minute),
			wantErr:  false,
		},
		{
			name:     "complex format: positive 2 hours 15 minutes 30 seconds",
			input:    "P0DT2H15M30S",
			expected: 2*time.Hour + 15*time.Minute + 30*time.Second,
			wantErr:  false,
		},
		{
			name:     "complex format: just minutes and seconds",
			input:    "PT45M30S",
			expected: 45*time.Minute + 30*time.Second,
			wantErr:  false,
		},
		{
			name:     "complex format: just hours",
			input:    "PT3H",
			expected: 3 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "complex format: just seconds",
			input:    "PT120S",
			expected: 120 * time.Second,
			wantErr:  false,
		},
		{
			name:     "unsupported format",
			input:    "INVALID",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseICalDuration(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseICalDuration() expected error for input %s, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("parseICalDuration() unexpected error for input %s: %v", tt.input, err)
				return
			}

			if result != tt.expected {
				t.Errorf("parseICalDuration() for input %s = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"123", 123},
		{"0", 0},
		{"5", 5},
		{"999", 999},
		{"", 0},
		{"abc", 0}, // Invalid characters should return 0
		{"12abc", 0}, // Mixed should return 0
		{"0123", 123}, // Leading zeros should work
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseInt(tt.input)
			if result != tt.expected {
				t.Errorf("parseInt(%s) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestConvertICSAlarmToInternalAlarm tests alarm conversion through iCal data parsing
func TestConvertICSAlarmToInternalAlarm(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name            string
		icalData        string
		expectedMinutes int
		expectedMethod  string
		expectWarning   bool
	}{
		{
			name: "15 minute DISPLAY alarm",
			icalData: `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-event-1
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Test Meeting
BEGIN:VALARM
TRIGGER:-PT15M
ACTION:DISPLAY
END:VALARM
END:VEVENT
END:VCALENDAR`,
			expectedMinutes: 15,
			expectedMethod:  "DISPLAY",
			expectWarning:   false,
		},
		{
			name: "30 minute AUDIO alarm",
			icalData: `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-event-2
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Test Meeting
BEGIN:VALARM
TRIGGER:-PT30M
ACTION:AUDIO
END:VALARM
END:VEVENT
END:VCALENDAR`,
			expectedMinutes: 30,
			expectedMethod:  "AUDIO",
			expectWarning:   false,
		},
		{
			name: "complex duration alarm",
			icalData: `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-event-3
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Test Meeting
BEGIN:VALARM
TRIGGER:-P0DT0H45M0S
ACTION:DISPLAY
END:VALARM
END:VEVENT
END:VCALENDAR`,
			expectedMinutes: 45,
			expectedMethod:  "DISPLAY",
			expectWarning:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the iCal data and extract the first event's first alarm
			calendar, err := ics.ParseCalendar(strings.NewReader(tt.icalData))
			if err != nil {
				t.Fatalf("Failed to parse iCal data: %v", err)
			}

			events := calendar.Events()
			if len(events) != 1 {
				t.Fatalf("Expected 1 event, got %d", len(events))
			}

			event := events[0]
			alarms := event.Alarms()
			if len(alarms) != 1 {
				t.Fatalf("Expected 1 alarm, got %d", len(alarms))
			}

			alarm := alarms[0]
			result, err := ConvertICSAlarmToInternalAlarm(alarm, event, "test-calendar", logger)

			if err != nil {
				t.Errorf("ConvertICSAlarmToInternalAlarm() unexpected error: %v", err)
				return
			}

			if result.LeadTimeMinutes != tt.expectedMinutes {
				t.Errorf("ConvertICSAlarmToInternalAlarm() LeadTimeMinutes = %d, expected %d",
					result.LeadTimeMinutes, tt.expectedMinutes)
			}

			if result.Method != tt.expectedMethod {
				t.Errorf("ConvertICSAlarmToInternalAlarm() Method = %s, expected %s",
					result.Method, tt.expectedMethod)
			}

			if result.Severity != "normal" {
				t.Errorf("ConvertICSAlarmToInternalAlarm() Severity = %s, expected normal",
					result.Severity)
			}
		})
	}
}

func TestConvertICSEventToInternalEvent(t *testing.T) {
	logger := slog.Default()
	calendarID := "test-calendar-id"
	calendarName := "Test Calendar"

	// Test successful conversion
	t.Run("successful conversion", func(t *testing.T) {
		icalData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-event-123
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Test Meeting
DESCRIPTION:This is a test meeting
LOCATION:Conference Room A
BEGIN:VALARM
TRIGGER:-PT15M
ACTION:DISPLAY
END:VALARM
END:VEVENT
END:VCALENDAR`

		calendar, err := ics.ParseCalendar(strings.NewReader(icalData))
		if err != nil {
			t.Fatalf("Failed to parse iCal data: %v", err)
		}

		events := calendar.Events()
		if len(events) != 1 {
			t.Fatalf("Expected 1 event, got %d", len(events))
		}

		event := events[0]
		result, err := ConvertICSEventToInternalEvent(event, calendarID, calendarName, logger)

		if err != nil {
			t.Errorf("ConvertICSEventToInternalEvent() unexpected error: %v", err)
			return
		}

		// Verify basic properties
		if result.ID != "test-event-123" {
			t.Errorf("Expected ID 'test-event-123', got '%s'", result.ID)
		}
		if result.Title != "Test Meeting" {
			t.Errorf("Expected Title 'Test Meeting', got '%s'", result.Title)
		}
		if result.Description != "This is a test meeting" {
			t.Errorf("Expected Description 'This is a test meeting', got '%s'", result.Description)
		}
		if result.Location != "Conference Room A" {
			t.Errorf("Expected Location 'Conference Room A', got '%s'", result.Location)
		}
		if result.CalendarID != calendarID {
			t.Errorf("Expected CalendarID '%s', got '%s'", calendarID, result.CalendarID)
		}
		if result.CalendarName != calendarName {
			t.Errorf("Expected CalendarName '%s', got '%s'", calendarName, result.CalendarName)
		}

		// Verify times
		expectedStart := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		expectedEnd := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)
		if !result.StartTime.Equal(expectedStart) {
			t.Errorf("Expected StartTime %v, got %v", expectedStart, result.StartTime)
		}
		if !result.EndTime.Equal(expectedEnd) {
			t.Errorf("Expected EndTime %v, got %v", expectedEnd, result.EndTime)
		}

		// Verify alarm
		if len(result.Alarms) != 1 {
			t.Errorf("Expected 1 alarm, got %d", len(result.Alarms))
		} else {
			if result.Alarms[0].LeadTimeMinutes != 15 {
				t.Errorf("Expected alarm lead time 15 minutes, got %d", result.Alarms[0].LeadTimeMinutes)
			}
		}
	})

	// Test missing UID
	t.Run("missing UID", func(t *testing.T) {
		icalData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Test Meeting
END:VEVENT
END:VCALENDAR`

		calendar, err := ics.ParseCalendar(strings.NewReader(icalData))
		if err != nil {
			t.Fatalf("Failed to parse iCal data: %v", err)
		}

		events := calendar.Events()
		if len(events) != 1 {
			t.Fatalf("Expected 1 event, got %d", len(events))
		}

		event := events[0]
		_, err = ConvertICSEventToInternalEvent(event, calendarID, calendarName, logger)

		if err == nil {
			t.Error("ConvertICSEventToInternalEvent() expected error for missing UID, got nil")
		}
		if !strings.Contains(err.Error(), "missing UID") {
			t.Errorf("Expected error to mention 'missing UID', got: %v", err)
		}
	})

	// Test missing start time
	t.Run("missing start time", func(t *testing.T) {
		icalData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-event-456
SUMMARY:Test Meeting
END:VEVENT
END:VCALENDAR`

		calendar, err := ics.ParseCalendar(strings.NewReader(icalData))
		if err != nil {
			t.Fatalf("Failed to parse iCal data: %v", err)
		}

		events := calendar.Events()
		if len(events) != 1 {
			t.Fatalf("Expected 1 event, got %d", len(events))
		}

		event := events[0]
		_, err = ConvertICSEventToInternalEvent(event, calendarID, calendarName, logger)

		if err == nil {
			t.Error("ConvertICSEventToInternalEvent() expected error for missing start time, got nil")
		}
	})

	// Test default end time (1 hour after start)
	t.Run("default end time", func(t *testing.T) {
		icalData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-event-789
DTSTART:20240115T100000Z
SUMMARY:Test Meeting
END:VEVENT
END:VCALENDAR`

		calendar, err := ics.ParseCalendar(strings.NewReader(icalData))
		if err != nil {
			t.Fatalf("Failed to parse iCal data: %v", err)
		}

		events := calendar.Events()
		if len(events) != 1 {
			t.Fatalf("Expected 1 event, got %d", len(events))
		}

		event := events[0]
		result, err := ConvertICSEventToInternalEvent(event, calendarID, calendarName, logger)

		if err != nil {
			t.Errorf("ConvertICSEventToInternalEvent() unexpected error: %v", err)
			return
		}

		expectedStart := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		expectedEnd := expectedStart.Add(1 * time.Hour)
		if !result.EndTime.Equal(expectedEnd) {
			t.Errorf("Expected default EndTime %v, got %v", expectedEnd, result.EndTime)
		}
	})
}

func TestParseICalData(t *testing.T) {
	logger := slog.Default()
	calendarID := "test-calendar"
	calendarName := "Test Calendar"

	// Create a sample iCal data string
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test Calendar//EN
BEGIN:VEVENT
UID:event1@example.com
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Team Meeting
DESCRIPTION:Weekly team sync
LOCATION:Conference Room A
BEGIN:VALARM
TRIGGER:-PT15M
ACTION:DISPLAY
END:VALARM
END:VEVENT
BEGIN:VEVENT
UID:event2@example.com
DTSTART:20240116T140000Z
DTEND:20240116T150000Z
SUMMARY:Project Review
DESCRIPTION:Review project progress
LOCATION:Conference Room B
BEGIN:VALARM
TRIGGER:-PT30M
ACTION:DISPLAY
END:VALARM
END:VEVENT
BEGIN:VEVENT
UID:event3@example.com
DTSTART:20240120T100000Z
DTEND:20240120T110000Z
SUMMARY:Future Meeting
DESCRIPTION:Meeting far in the future
END:VEVENT
END:VCALENDAR`

	// Define time range that includes first two events but not the third
	from := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 17, 0, 0, 0, 0, time.UTC)

	events, err := ParseICalData(icalData, calendarID, calendarName, from, to, logger)

	if err != nil {
		t.Errorf("ParseICalData() unexpected error: %v", err)
		return
	}

	// Should return 2 events (first two are in range, third is outside)
	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
		return
	}

	// Verify first event
	event1 := events[0]
	if event1.ID != "event1@example.com" {
		t.Errorf("Expected first event ID 'event1@example.com', got '%s'", event1.ID)
	}
	if event1.Title != "Team Meeting" {
		t.Errorf("Expected first event title 'Team Meeting', got '%s'", event1.Title)
	}
	if len(event1.Alarms) != 1 {
		t.Errorf("Expected 1 alarm on first event, got %d", len(event1.Alarms))
	} else if event1.Alarms[0].LeadTimeMinutes != 15 {
		t.Errorf("Expected first event alarm lead time 15, got %d", event1.Alarms[0].LeadTimeMinutes)
	}

	// Verify second event
	event2 := events[1]
	if event2.ID != "event2@example.com" {
		t.Errorf("Expected second event ID 'event2@example.com', got '%s'", event2.ID)
	}
	if event2.Title != "Project Review" {
		t.Errorf("Expected second event title 'Project Review', got '%s'", event2.Title)
	}
	if len(event2.Alarms) != 1 {
		t.Errorf("Expected 1 alarm on second event, got %d", len(event2.Alarms))
	} else if event2.Alarms[0].LeadTimeMinutes != 30 {
		t.Errorf("Expected second event alarm lead time 30, got %d", event2.Alarms[0].LeadTimeMinutes)
	}
}

func TestParseICalDataInvalidData(t *testing.T) {
	logger := slog.Default()
	calendarID := "test-calendar"
	calendarName := "Test Calendar"
	from := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 17, 0, 0, 0, 0, time.UTC)

	// Test invalid iCal data
	invalidICalData := "This is not valid iCal data"

	_, err := ParseICalData(invalidICalData, calendarID, calendarName, from, to, logger)

	if err == nil {
		t.Error("ParseICalData() expected error for invalid iCal data, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse iCal data") {
		t.Errorf("Expected error to mention 'failed to parse iCal data', got: %v", err)
	}
}

func TestParseICalDataEmptyCalendar(t *testing.T) {
	logger := slog.Default()
	calendarID := "test-calendar"
	calendarName := "Test Calendar"
	from := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 17, 0, 0, 0, 0, time.UTC)

	// Empty but valid iCal data
	emptyICalData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test Calendar//EN
END:VCALENDAR`

	events, err := ParseICalData(emptyICalData, calendarID, calendarName, from, to, logger)

	if err != nil {
		t.Errorf("ParseICalData() unexpected error for empty calendar: %v", err)
		return
	}

	if len(events) != 0 {
		t.Errorf("Expected 0 events for empty calendar, got %d", len(events))
	}
}

// TestParseICalDurationEdgeCases tests additional edge cases for duration parsing
func TestParseICalDurationEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{
			name:     "zero duration",
			input:    "PT0M",
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "very large duration",
			input:    "PT9999M",
			expected: 9999 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "mixed components with zeros",
			input:    "P0DT0H0M30S",
			expected: 30 * time.Second,
			wantErr:  false,
		},
		{
			name:     "just 'P' (invalid)",
			input:    "P",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "just 'PT' (invalid)",
			input:    "PT",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "negative complex with all components",
			input:    "-P1DT2H3M4S",
			expected: -(2*time.Hour + 3*time.Minute + 4*time.Second), // Days are skipped in current implementation
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseICalDuration(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseICalDuration() expected error for input %s, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("parseICalDuration() unexpected error for input %s: %v", tt.input, err)
				return
			}

			if result != tt.expected {
				t.Errorf("parseICalDuration() for input %s = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestConvertICSAlarmEdgeCases tests edge cases for alarm conversion
func TestConvertICSAlarmEdgeCases(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name            string
		icalData        string
		expectedMinutes int
		expectedMethod  string
		expectDefault   bool // Whether to expect default fallback values
	}{
		{
			name: "alarm without trigger (should use zero minutes)",
			icalData: `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-event-1
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Test Meeting
BEGIN:VALARM
ACTION:DISPLAY
END:VALARM
END:VEVENT
END:VCALENDAR`,
			expectedMinutes: 0, // No trigger means no lead time set
			expectedMethod:  "DISPLAY",
			expectDefault:   false,
		},
		{
			name: "alarm without action (should use popup default)",
			icalData: `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-event-2
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Test Meeting
BEGIN:VALARM
TRIGGER:-PT20M
END:VALARM
END:VEVENT
END:VCALENDAR`,
			expectedMinutes: 20,
			expectedMethod:  "popup", // Default method
			expectDefault:   false,
		},
		{
			name: "alarm with absolute time trigger (should use default)",
			icalData: `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-event-3
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Test Meeting
BEGIN:VALARM
TRIGGER:20240115T094500Z
ACTION:EMAIL
END:VALARM
END:VEVENT
END:VCALENDAR`,
			expectedMinutes: 15, // Default fallback
			expectedMethod:  "EMAIL",
			expectDefault:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the iCal data and extract the first event's first alarm
			calendar, err := ics.ParseCalendar(strings.NewReader(tt.icalData))
			if err != nil {
				t.Fatalf("Failed to parse iCal data: %v", err)
			}

			events := calendar.Events()
			if len(events) != 1 {
				t.Fatalf("Expected 1 event, got %d", len(events))
			}

			event := events[0]
			alarms := event.Alarms()
			if len(alarms) != 1 {
				t.Fatalf("Expected 1 alarm, got %d", len(alarms))
			}

			alarm := alarms[0]
			result, err := ConvertICSAlarmToInternalAlarm(alarm, event, "test-calendar", logger)

			if err != nil {
				t.Errorf("ConvertICSAlarmToInternalAlarm() unexpected error: %v", err)
				return
			}

			if result.LeadTimeMinutes != tt.expectedMinutes {
				t.Errorf("ConvertICSAlarmToInternalAlarm() LeadTimeMinutes = %d, expected %d",
					result.LeadTimeMinutes, tt.expectedMinutes)
			}

			if result.Method != tt.expectedMethod {
				t.Errorf("ConvertICSAlarmToInternalAlarm() Method = %s, expected %s",
					result.Method, tt.expectedMethod)
			}

			if result.Severity != "normal" {
				t.Errorf("ConvertICSAlarmToInternalAlarm() Severity = %s, expected normal",
					result.Severity)
			}
		})
	}
}

// TestParseICalDataTimeFiltering tests the time range filtering functionality
func TestParseICalDataTimeFiltering(t *testing.T) {
	logger := slog.Default()
	calendarID := "test-calendar"
	calendarName := "Test Calendar"

	icalData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test Calendar//EN
BEGIN:VEVENT
UID:past-event@example.com
DTSTART:20240101T100000Z
DTEND:20240101T110000Z
SUMMARY:Past Event
END:VEVENT
BEGIN:VEVENT
UID:current-event@example.com
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Current Event
END:VEVENT
BEGIN:VEVENT
UID:future-event@example.com
DTSTART:20240201T100000Z
DTEND:20240201T110000Z
SUMMARY:Future Event
END:VEVENT
BEGIN:VEVENT
UID:overlapping-start@example.com
DTSTART:20240110T100000Z
DTEND:20240116T110000Z
SUMMARY:Overlapping Start
END:VEVENT
BEGIN:VEVENT
UID:overlapping-end@example.com
DTSTART:20240114T100000Z
DTEND:20240120T110000Z
SUMMARY:Overlapping End
END:VEVENT
END:VCALENDAR`

	// Set time range from Jan 15 to Jan 17
	from := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 17, 0, 0, 0, 0, time.UTC)

	events, err := ParseICalData(icalData, calendarID, calendarName, from, to, logger)

	if err != nil {
		t.Errorf("ParseICalData() unexpected error: %v", err)
		return
	}

	// Should include: current-event, overlapping-start, overlapping-end
	// Should exclude: past-event, future-event
	expectedEventIDs := map[string]bool{
		"current-event@example.com":   true,
		"overlapping-start@example.com": true,
		"overlapping-end@example.com":   true,
	}

	if len(events) != 3 {
		t.Errorf("Expected 3 events in range, got %d", len(events))
	}

	for _, event := range events {
		if !expectedEventIDs[event.ID] {
			t.Errorf("Unexpected event in results: %s", event.ID)
		}
	}

	// Verify all expected events are present
	foundEventIDs := make(map[string]bool)
	for _, event := range events {
		foundEventIDs[event.ID] = true
	}

	for expectedID := range expectedEventIDs {
		if !foundEventIDs[expectedID] {
			t.Errorf("Expected event missing from results: %s", expectedID)
		}
	}
}
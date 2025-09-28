// +build integration

package ical

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestProvider_IntegrationWithRealServer tests the provider with a more realistic server setup
func TestProvider_IntegrationWithRealServer(t *testing.T) {
	// Complex iCal data with multiple events, recurring events, and various alarm types
	complexICalData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test Corp//Test Calendar//EN
CALSCALE:GREGORIAN
METHOD:PUBLISH
BEGIN:VEVENT
UID:meeting-001@example.com
DTSTART:20251001T090000Z
DTEND:20251001T100000Z
SUMMARY:Team Standup
DESCRIPTION:Daily team standup meeting
LOCATION:Conference Room A
BEGIN:VALARM
ACTION:DISPLAY
DESCRIPTION:Team Standup Reminder
TRIGGER:-PT15M
END:VALARM
BEGIN:VALARM
ACTION:EMAIL
DESCRIPTION:Team Standup Email Reminder
TRIGGER:-PT30M
SUMMARY:Reminder: Team Standup
ATTENDEE:mailto:team@example.com
END:VALARM
END:VEVENT
BEGIN:VEVENT
UID:workshop-001@example.com
DTSTART:20251002T140000Z
DTEND:20251002T170000Z
SUMMARY:Technical Workshop
DESCRIPTION:Deep dive into new technologies
LOCATION:Training Room B
CATEGORIES:WORK,TRAINING
BEGIN:VALARM
ACTION:POPUP
DESCRIPTION:Workshop starts in 10 minutes
TRIGGER:-PT10M
END:VALARM
END:VEVENT
BEGIN:VEVENT
UID:recurring-001@example.com
DTSTART:20251001T180000Z
DTEND:20251001T190000Z
SUMMARY:Weekly Review
DESCRIPTION:Weekly project review meeting
RRULE:FREQ=WEEKLY;BYDAY=TU;COUNT=10
BEGIN:VALARM
ACTION:DISPLAY
DESCRIPTION:Weekly Review Reminder
TRIGGER:-PT20M
END:VALARM
END:VEVENT
END:VCALENDAR`

	// Create a test server with more realistic behavior
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate some network latency
		time.Sleep(50 * time.Millisecond)

		// Check for proper headers
		if userAgent := r.Header.Get("User-Agent"); userAgent == "" {
			t.Logf("Warning: No User-Agent header set")
		}

		// Return the complex iCal data
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Content-Length", string(rune(len(complexICalData))))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(complexICalData))
	}))
	defer server.Close()

	provider := NewProvider()

	// Initialize with the test server URL
	err := provider.Initialize(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// Test the health check
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = provider.IsHealthy(ctx)
	if err != nil {
		t.Errorf("Health check failed: %v", err)
	}

	// Test getting calendars
	calendars, err := provider.GetCalendars(ctx)
	if err != nil {
		t.Errorf("Failed to get calendars: %v", err)
	}
	if len(calendars) != 1 {
		t.Errorf("Expected 1 calendar, got %d", len(calendars))
	}

	// Test getting events with a broader time range
	from := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 10, 31, 23, 59, 59, 0, time.UTC)

	events, err := provider.GetEvents(ctx, []string{}, from, to)
	if err != nil {
		t.Errorf("Failed to get events: %v", err)
	}

	// Verify we got the expected number of events
	// Note: recurring events might generate multiple instances
	if len(events) < 3 {
		t.Errorf("Expected at least 3 events, got %d", len(events))
	}

	// Verify specific events and their properties
	eventTitles := make(map[string]bool)
	for _, event := range events {
		eventTitles[event.Title] = true

		// Verify that events have proper structure
		if event.ID == "" {
			t.Errorf("Event missing ID: %+v", event)
		}
		if event.Title == "" {
			t.Errorf("Event missing Title: %+v", event)
		}
		if event.StartTime.IsZero() {
			t.Errorf("Event missing StartTime: %+v", event)
		}
		if event.EndTime.IsZero() {
			t.Errorf("Event missing EndTime: %+v", event)
		}
		if event.CalendarID == "" {
			t.Errorf("Event missing CalendarID: %+v", event)
		}
	}

	// Verify expected events are present
	expectedTitles := []string{"Team Standup", "Technical Workshop", "Weekly Review"}
	for _, expectedTitle := range expectedTitles {
		if !eventTitles[expectedTitle] {
			t.Errorf("Expected event '%s' not found in results", expectedTitle)
		}
	}

	// Test with different time ranges
	narrowFrom := time.Date(2025, 10, 1, 8, 0, 0, 0, time.UTC)
	narrowTo := time.Date(2025, 10, 1, 12, 0, 0, 0, time.UTC)

	narrowEvents, err := provider.GetEvents(ctx, []string{}, narrowFrom, narrowTo)
	if err != nil {
		t.Errorf("Failed to get events with narrow range: %v", err)
	}

	// Should only get the Team Standup event (9:00-10:00)
	if len(narrowEvents) != 1 {
		t.Errorf("Expected 1 event in narrow range, got %d", len(narrowEvents))
	}
	if len(narrowEvents) > 0 && narrowEvents[0].Title != "Team Standup" {
		t.Errorf("Expected 'Team Standup' event, got '%s'", narrowEvents[0].Title)
	}

	// Test cleanup
	err = provider.Close()
	if err != nil {
		t.Errorf("Failed to close provider: %v", err)
	}
}

// TestProvider_IntegrationErrorScenarios tests various error conditions
func TestProvider_IntegrationErrorScenarios(t *testing.T) {
	// Test with server that returns different HTTP status codes
	testCases := []struct {
		name       string
		statusCode int
		expectErr  bool
	}{
		{"Success", http.StatusOK, false},
		{"Not Found", http.StatusNotFound, true},
		{"Unauthorized", http.StatusUnauthorized, true},
		{"Internal Server Error", http.StatusInternalServerError, true},
		{"Bad Gateway", http.StatusBadGateway, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.statusCode == http.StatusOK {
					w.Header().Set("Content-Type", "text/calendar")
					w.WriteHeader(tc.statusCode)
					w.Write([]byte("BEGIN:VCALENDAR\nVERSION:2.0\nEND:VCALENDAR"))
				} else {
					w.WriteHeader(tc.statusCode)
					w.Write([]byte("Error occurred"))
				}
			}))
			defer server.Close()

			provider := NewProvider()
			err := provider.Initialize(context.Background(), server.URL)
			if err != nil {
				t.Fatalf("Failed to initialize provider: %v", err)
			}

			ctx := context.Background()
			_, err = provider.GetEvents(ctx, []string{}, time.Now(), time.Now().Add(time.Hour))

			if tc.expectErr && err == nil {
				t.Errorf("Expected error for status %d, but got none", tc.statusCode)
			}
			if !tc.expectErr && err != nil {
				t.Errorf("Expected no error for status %d, but got: %v", tc.statusCode, err)
			}
		})
	}
}

// TestProvider_IntegrationTimeout tests timeout behavior
func TestProvider_IntegrationTimeout(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Delay longer than our timeout
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("BEGIN:VCALENDAR\nVERSION:2.0\nEND:VCALENDAR"))
	}))
	defer server.Close()

	provider := NewProvider()
	err := provider.Initialize(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// Create a context with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err = provider.GetEvents(ctx, []string{}, time.Now(), time.Now().Add(time.Hour))
	if err == nil {
		t.Error("Expected timeout error, but got none")
	}

	// Verify it's a context timeout error
	if ctx.Err() != context.DeadlineExceeded {
		t.Errorf("Expected context deadline exceeded, got: %v", ctx.Err())
	}
}

// TestProvider_IntegrationMalformedData tests handling of malformed iCal data
func TestProvider_IntegrationMalformedData(t *testing.T) {
	malformedData := []struct {
		name string
		data string
	}{
		{"Empty", ""},
		{"Invalid iCal", "This is not iCal data"},
		{"Incomplete iCal", "BEGIN:VCALENDAR\nVERSION:2.0\n"}, // Missing END:VCALENDAR
		{"Mixed Content", "Some text\nBEGIN:VCALENDAR\nVERSION:2.0\nEND:VCALENDAR\nMore text"},
	}

	for _, tc := range malformedData {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/calendar")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tc.data))
			}))
			defer server.Close()

			provider := NewProvider()
			err := provider.Initialize(context.Background(), server.URL)
			if err != nil {
				t.Fatalf("Failed to initialize provider: %v", err)
			}

			ctx := context.Background()
			events, err := provider.GetEvents(ctx, []string{}, time.Now(), time.Now().Add(time.Hour))

			// We expect either an error or empty events for malformed data
			if err == nil && len(events) > 0 {
				t.Errorf("Expected error or empty events for malformed data, got %d events", len(events))
			}
		})
	}
}
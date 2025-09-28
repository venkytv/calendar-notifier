package ical

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewProvider(t *testing.T) {
	provider := NewProvider()
	if provider == nil {
		t.Fatal("NewProvider returned nil")
	}
	if provider.Name() != "iCal" {
		t.Errorf("Expected name 'iCal', got %s", provider.Name())
	}
	if provider.Type() != "ical" {
		t.Errorf("Expected type 'ical', got %s", provider.Type())
	}
}

func TestProvider_Initialize(t *testing.T) {
	provider := NewProvider()

	// Test with empty URL
	err := provider.Initialize(context.Background(), "")
	if err == nil {
		t.Error("Expected error for empty URL")
	}

	// Test with valid URL
	testURL := "https://example.com/calendar.ics"
	err = provider.Initialize(context.Background(), testURL)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if provider.url != testURL {
		t.Errorf("Expected URL %s, got %s", testURL, provider.url)
	}
}

func TestProvider_SetLogger(t *testing.T) {
	provider := NewProvider()
	logger := slog.Default()

	provider.SetLogger(logger)
	// No direct way to verify the logger was set, but should not panic

	// Test with nil logger
	provider.SetLogger(nil)
	// Should not change the logger
}

func TestProvider_GetCalendars(t *testing.T) {
	provider := NewProvider()

	// Test without initialization
	_, err := provider.GetCalendars(context.Background())
	if err == nil {
		t.Error("Expected error when not initialized")
	}

	// Test with initialization
	testURL := "https://example.com/calendar.ics"
	err = provider.Initialize(context.Background(), testURL)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	calendars, err := provider.GetCalendars(context.Background())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(calendars) != 1 {
		t.Errorf("Expected 1 calendar, got %d", len(calendars))
	}
	if calendars[0].ID != testURL {
		t.Errorf("Expected calendar ID %s, got %s", testURL, calendars[0].ID)
	}
	if calendars[0].Name != "iCal Calendar" {
		t.Errorf("Expected calendar name 'iCal Calendar', got %s", calendars[0].Name)
	}
}

func TestProvider_GetEvents(t *testing.T) {
	// Create a test server that returns iCal data
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:test-event-1@example.com
DTSTART:20251001T100000Z
DTEND:20251001T110000Z
SUMMARY:Test Meeting
DESCRIPTION:A test meeting
END:VEVENT
END:VCALENDAR`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/calendar")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(icalData))
	}))
	defer server.Close()

	provider := NewProvider()
	err := provider.Initialize(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	from := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 10, 2, 0, 0, 0, 0, time.UTC)

	events, err := provider.GetEvents(context.Background(), []string{}, from, to)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}
	if len(events) > 0 && events[0].Title != "Test Meeting" {
		t.Errorf("Expected event title 'Test Meeting', got %s", events[0].Title)
	}
}

func TestProvider_GetEvents_NotInitialized(t *testing.T) {
	provider := NewProvider()
	from := time.Now()
	to := time.Now().Add(time.Hour)

	_, err := provider.GetEvents(context.Background(), []string{}, from, to)
	if err == nil {
		t.Error("Expected error when not initialized")
	}
}

func TestProvider_GetEvents_HTTPError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	provider := NewProvider()
	err := provider.Initialize(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	from := time.Now()
	to := time.Now().Add(time.Hour)

	_, err = provider.GetEvents(context.Background(), []string{}, from, to)
	if err == nil {
		t.Error("Expected error for HTTP 500")
	}
}

func TestProvider_IsHealthy(t *testing.T) {
	provider := NewProvider()

	// Test without initialization
	err := provider.IsHealthy(context.Background())
	if err == nil {
		t.Error("Expected error when not initialized")
	}

	// Test with successful server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("BEGIN:VCALENDAR\nVERSION:2.0\nEND:VCALENDAR"))
	}))
	defer server.Close()

	err = provider.Initialize(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	err = provider.IsHealthy(context.Background())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Test with failing server
	server.Close()
	err = provider.IsHealthy(context.Background())
	if err == nil {
		t.Error("Expected error for unavailable server")
	}
}

func TestProvider_Close(t *testing.T) {
	provider := NewProvider()
	err := provider.Close()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestProvider_fetchICalData(t *testing.T) {
	testData := "test calendar data"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that proper headers are set
		accept := r.Header.Get("Accept")
		if accept != "text/calendar,application/calendar" {
			t.Errorf("Expected Accept header 'text/calendar,application/calendar', got %s", accept)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testData))
	}))
	defer server.Close()

	provider := NewProvider()
	provider.url = server.URL

	data, err := provider.fetchICalData(context.Background())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if data != testData {
		t.Errorf("Expected data '%s', got '%s'", testData, data)
	}
}

func TestProvider_fetchICalData_ContextCancellation(t *testing.T) {
	// Create a server that never responds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Never respond
		select {}
	}))
	defer server.Close()

	provider := NewProvider()
	provider.url = server.URL

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := provider.fetchICalData(ctx)
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}
package caldav

import (
	"context"
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewSimpleProvider(t *testing.T) {
	provider := NewSimpleProvider()
	if provider == nil {
		t.Fatal("NewSimpleProvider returned nil")
	}
	if provider.Name() != "CalDAV" {
		t.Errorf("Expected name 'CalDAV', got %s", provider.Name())
	}
	if provider.Type() != "caldav" {
		t.Errorf("Expected type 'caldav', got %s", provider.Type())
	}
}

func TestSimpleProvider_Initialize(t *testing.T) {
	provider := NewSimpleProvider()

	// Initialize method should return error for file-based config
	err := provider.Initialize(context.Background(), "test-config.json")
	if err == nil {
		t.Error("Expected error for file-based initialization")
	}
	if !strings.Contains(err.Error(), "direct configuration") {
		t.Errorf("Expected error message about direct configuration, got: %v", err)
	}
}

func TestSimpleProvider_InitializeWithConfig(t *testing.T) {
	provider := NewSimpleProvider()

	// Test with empty URL
	config := &Config{URL: "", Username: "user", Password: "pass"}
	err := provider.InitializeWithConfig(config)
	if err == nil {
		t.Error("Expected error for empty URL")
	}

	// Test with empty username
	config = &Config{URL: "https://example.com", Username: "", Password: "pass"}
	err = provider.InitializeWithConfig(config)
	if err == nil {
		t.Error("Expected error for empty username")
	}

	// Test with empty password
	config = &Config{URL: "https://example.com", Username: "user", Password: ""}
	err = provider.InitializeWithConfig(config)
	if err == nil {
		t.Error("Expected error for empty password")
	}

	// Test with valid config
	config = &Config{
		URL:      "https://example.com/caldav",
		Username: "testuser",
		Password: "testpass",
	}
	err = provider.InitializeWithConfig(config)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if provider.url != config.URL {
		t.Errorf("Expected URL %s, got %s", config.URL, provider.url)
	}
	if provider.username != config.Username {
		t.Errorf("Expected username %s, got %s", config.Username, provider.username)
	}
	if provider.password != config.Password {
		t.Errorf("Expected password %s, got %s", config.Password, provider.password)
	}
}

func TestSimpleProvider_SetLogger(t *testing.T) {
	provider := NewSimpleProvider()
	logger := slog.Default()

	provider.SetLogger(logger)
	// No direct way to verify the logger was set, but should not panic

	// Test with nil logger
	provider.SetLogger(nil)
	// Should not change the logger
}

func TestSimpleProvider_GetCalendars(t *testing.T) {
	provider := NewSimpleProvider()

	// Test without initialization
	_, err := provider.GetCalendars(context.Background())
	if err == nil {
		t.Error("Expected error when not initialized")
	}

	// Test with initialization
	config := &Config{
		URL:      "https://example.com/caldav",
		Username: "user",
		Password: "pass",
	}
	err = provider.InitializeWithConfig(config)
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
	if calendars[0].ID != config.URL {
		t.Errorf("Expected calendar ID %s, got %s", config.URL, calendars[0].ID)
	}
	if calendars[0].Name != "CalDAV Calendar" {
		t.Errorf("Expected calendar name 'CalDAV Calendar', got %s", calendars[0].Name)
	}
	if calendars[0].AccessRole != "owner" {
		t.Errorf("Expected access role 'owner', got %s", calendars[0].AccessRole)
	}
}

func TestSimpleProvider_GetEvents(t *testing.T) {
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
		// Verify authentication header
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Basic ") {
			t.Errorf("Expected Basic auth header, got: %s", authHeader)
		}

		// Decode and verify credentials
		encoded := strings.TrimPrefix(authHeader, "Basic ")
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Errorf("Failed to decode auth header: %v", err)
		}
		credentials := string(decoded)
		if credentials != "testuser:testpass" {
			t.Errorf("Expected credentials 'testuser:testpass', got %s", credentials)
		}

		// Verify Accept header
		accept := r.Header.Get("Accept")
		if accept != "text/calendar" {
			t.Errorf("Expected Accept header 'text/calendar', got %s", accept)
		}

		w.Header().Set("Content-Type", "text/calendar")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(icalData))
	}))
	defer server.Close()

	provider := NewSimpleProvider()
	config := &Config{
		URL:      server.URL,
		Username: "testuser",
		Password: "testpass",
	}
	err := provider.InitializeWithConfig(config)
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

func TestSimpleProvider_GetEvents_NotInitialized(t *testing.T) {
	provider := NewSimpleProvider()
	from := time.Now()
	to := time.Now().Add(time.Hour)

	_, err := provider.GetEvents(context.Background(), []string{}, from, to)
	if err == nil {
		t.Error("Expected error when not initialized")
	}
}

func TestSimpleProvider_GetEvents_HTTPError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	provider := NewSimpleProvider()
	config := &Config{
		URL:      server.URL,
		Username: "user",
		Password: "pass",
	}
	err := provider.InitializeWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	from := time.Now()
	to := time.Now().Add(time.Hour)

	_, err = provider.GetEvents(context.Background(), []string{}, from, to)
	if err == nil {
		t.Error("Expected error for HTTP 401")
	}
}

func TestSimpleProvider_IsHealthy(t *testing.T) {
	provider := NewSimpleProvider()

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

	config := &Config{
		URL:      server.URL,
		Username: "user",
		Password: "pass",
	}
	err = provider.InitializeWithConfig(config)
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

func TestSimpleProvider_Close(t *testing.T) {
	provider := NewSimpleProvider()
	err := provider.Close()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestSimpleProvider_fetchICalData(t *testing.T) {
	testData := "test calendar data"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testData))
	}))
	defer server.Close()

	provider := NewSimpleProvider()
	config := &Config{
		URL:      server.URL,
		Username: "user",
		Password: "pass",
	}
	err := provider.InitializeWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	data, err := provider.fetchICalData(context.Background())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if data != testData {
		t.Errorf("Expected data '%s', got '%s'", testData, data)
	}
}

func TestSimpleProvider_fetchICalData_ContextCancellation(t *testing.T) {
	// Create a server that never responds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Never respond
		select {}
	}))
	defer server.Close()

	provider := NewSimpleProvider()
	config := &Config{
		URL:      server.URL,
		Username: "user",
		Password: "pass",
	}
	err := provider.InitializeWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = provider.fetchICalData(ctx)
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}
// +build integration

package caldav

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestSimpleProvider_IntegrationWithAuth tests the provider with authentication scenarios
func TestSimpleProvider_IntegrationWithAuth(t *testing.T) {
	calendarData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//CalDAV Server//CalDAV Server//EN
BEGIN:VEVENT
UID:auth-test-001@example.com
DTSTART:20251201T100000Z
DTEND:20251201T110000Z
SUMMARY:Authenticated Event
DESCRIPTION:Event that requires authentication to access
LOCATION:Secure Room
BEGIN:VALARM
ACTION:DISPLAY
DESCRIPTION:Authenticated event reminder
TRIGGER:-PT10M
END:VALARM
END:VEVENT
END:VCALENDAR`

	// Test cases for different authentication scenarios
	testCases := []struct {
		name           string
		serverUsername string
		serverPassword string
		clientUsername string
		clientPassword string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "Valid Authentication",
			serverUsername: "testuser",
			serverPassword: "testpass",
			clientUsername: "testuser",
			clientPassword: "testpass",
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "Invalid Username",
			serverUsername: "correctuser",
			serverPassword: "correctpass",
			clientUsername: "wronguser",
			clientPassword: "correctpass",
			expectedStatus: http.StatusUnauthorized,
			expectError:    true,
		},
		{
			name:           "Invalid Password",
			serverUsername: "correctuser",
			serverPassword: "correctpass",
			clientUsername: "correctuser",
			clientPassword: "wrongpass",
			expectedStatus: http.StatusUnauthorized,
			expectError:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check for authentication header
				authHeader := r.Header.Get("Authorization")
				if authHeader == "" {
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte("Authentication required"))
					return
				}

				// Verify Basic auth format
				if !strings.HasPrefix(authHeader, "Basic ") {
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte("Invalid authentication format"))
					return
				}

				// Decode credentials
				encoded := strings.TrimPrefix(authHeader, "Basic ")
				decoded, err := base64.StdEncoding.DecodeString(encoded)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte("Invalid base64 encoding"))
					return
				}

				credentials := strings.SplitN(string(decoded), ":", 2)
				if len(credentials) != 2 {
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte("Invalid credentials format"))
					return
				}

				username := credentials[0]
				password := credentials[1]

				// Check credentials
				if username != tc.serverUsername || password != tc.serverPassword {
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte("Invalid credentials"))
					return
				}

				// Verify other headers
				if accept := r.Header.Get("Accept"); accept != "text/calendar" {
					t.Logf("Warning: Expected Accept header 'text/calendar', got '%s'", accept)
				}

				// Success - return calendar data
				w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(calendarData))
			}))
			defer server.Close()

			provider := NewSimpleProvider()
			config := &Config{
				URL:      server.URL,
				Username: tc.clientUsername,
				Password: tc.clientPassword,
			}

			err := provider.InitializeWithConfig(config)
			if err != nil {
				t.Fatalf("Failed to initialize provider: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			from := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
			to := time.Date(2025, 12, 2, 0, 0, 0, 0, time.UTC)

			events, err := provider.GetEvents(ctx, []string{}, from, to)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", tc.name)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for %s, but got: %v", tc.name, err)
				}
				if len(events) != 1 {
					t.Errorf("Expected 1 event for %s, got %d", tc.name, len(events))
				}
				if len(events) > 0 && events[0].Title != "Authenticated Event" {
					t.Errorf("Expected event title 'Authenticated Event' for %s, got '%s'", tc.name, events[0].Title)
				}
			}
		})
	}
}

// TestSimpleProvider_IntegrationHealthCheck tests comprehensive health check scenarios
func TestSimpleProvider_IntegrationHealthCheck(t *testing.T) {
	healthCheckCases := []struct {
		name        string
		setupServer func() *httptest.Server
		expectError bool
	}{
		{
			name: "Healthy Server",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/calendar")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("BEGIN:VCALENDAR\nVERSION:2.0\nEND:VCALENDAR"))
				}))
			},
			expectError: false,
		},
		{
			name: "Server Returns Error",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("Internal server error"))
				}))
			},
			expectError: true,
		},
		{
			name: "Slow Server",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(100 * time.Millisecond) // Simulate slow response
					w.Header().Set("Content-Type", "text/calendar")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("BEGIN:VCALENDAR\nVERSION:2.0\nEND:VCALENDAR"))
				}))
			},
			expectError: false,
		},
		{
			name: "Authentication Required",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					auth := r.Header.Get("Authorization")
					if auth == "" {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					w.Header().Set("Content-Type", "text/calendar")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("BEGIN:VCALENDAR\nVERSION:2.0\nEND:VCALENDAR"))
				}))
			},
			expectError: false, // Should work with credentials
		},
	}

	for _, tc := range healthCheckCases {
		t.Run(tc.name, func(t *testing.T) {
			server := tc.setupServer()
			defer server.Close()

			provider := NewSimpleProvider()
			config := &Config{
				URL:      server.URL,
				Username: "testuser",
				Password: "testpass",
			}

			err := provider.InitializeWithConfig(config)
			if err != nil {
				t.Fatalf("Failed to initialize provider: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err = provider.IsHealthy(ctx)

			if tc.expectError && err == nil {
				t.Errorf("Expected error for %s, but got none", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no error for %s, but got: %v", tc.name, err)
			}
		})
	}
}

// TestSimpleProvider_IntegrationConcurrency tests concurrent access to the provider
func TestSimpleProvider_IntegrationConcurrency(t *testing.T) {
	calendarData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:concurrent-test-001@example.com
DTSTART:20251101T120000Z
DTEND:20251101T130000Z
SUMMARY:Concurrent Access Test
END:VEVENT
END:VCALENDAR`

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		// Add some processing delay to make concurrency issues more likely
		time.Sleep(10 * time.Millisecond)

		w.Header().Set("Content-Type", "text/calendar")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(calendarData))
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
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	// Run multiple concurrent requests
	const numRequests = 10
	results := make(chan error, numRequests)

	from := time.Date(2025, 11, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 11, 2, 0, 0, 0, 0, time.UTC)

	for i := 0; i < numRequests; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			events, err := provider.GetEvents(ctx, []string{}, from, to)
			if err != nil {
				results <- err
				return
			}

			if len(events) != 1 {
				results <- err
				return
			}

			results <- nil
		}()
	}

	// Wait for all requests to complete
	var errors []error
	for i := 0; i < numRequests; i++ {
		if err := <-results; err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		t.Errorf("Got %d errors from concurrent requests: %v", len(errors), errors[0])
	}

	// Verify that all requests were actually made (requestCount should be numRequests)
	if requestCount != numRequests {
		t.Errorf("Expected %d requests to be made, but got %d", numRequests, requestCount)
	}
}

// TestSimpleProvider_IntegrationLargeResponse tests handling of large calendar responses
func TestSimpleProvider_IntegrationLargeResponse(t *testing.T) {
	// Generate a large iCal file with many events
	largeCalendarData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Large Test//Large Test//EN
`

	// Add 100 events to create a large response
	for i := 1; i <= 100; i++ {
		eventData := `BEGIN:VEVENT
UID:large-test-%03d@example.com
DTSTART:202511%02dT%02d0000Z
DTEND:202511%02dT%02d3000Z
SUMMARY:Large Test Event %d
DESCRIPTION:This is event number %d in a large calendar dataset for testing provider performance and memory handling.
LOCATION:Test Location %d
BEGIN:VALARM
ACTION:DISPLAY
DESCRIPTION:Reminder for event %d
TRIGGER:-PT15M
END:VALARM
END:VEVENT
`
		day := (i % 30) + 1
		hour := (i % 24)
		largeCalendarData += fmt.Sprintf(eventData, i, day, hour, day, hour+1, i, i, i, i)
	}

	largeCalendarData += "END:VCALENDAR\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set appropriate headers for large content
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Content-Length", string(rune(len(largeCalendarData))))

		// Write data in chunks to simulate realistic network conditions
		data := []byte(largeCalendarData)
		chunkSize := 1024
		for i := 0; i < len(data); i += chunkSize {
			end := i + chunkSize
			if end > len(data) {
				end = len(data)
			}
			w.Write(data[i:end])

			// Small delay between chunks to simulate network latency
			time.Sleep(1 * time.Millisecond)
		}
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
		t.Fatalf("Failed to initialize provider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	from := time.Date(2025, 11, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 11, 30, 23, 59, 59, 0, time.UTC)

	startTime := time.Now()
	events, err := provider.GetEvents(ctx, []string{}, from, to)
	elapsed := time.Since(startTime)

	if err != nil {
		t.Fatalf("Failed to get events from large calendar: %v", err)
	}

	t.Logf("Retrieved %d events in %v", len(events), elapsed)

	// Verify we got a reasonable number of events (should be 100)
	if len(events) != 100 {
		t.Errorf("Expected 100 events from large calendar, got %d", len(events))
	}

	// Verify performance is reasonable (should complete within 60 seconds)
	if elapsed > 60*time.Second {
		t.Errorf("Large calendar processing took too long: %v", elapsed)
	}

	// Verify events have proper structure
	for i, event := range events[:5] { // Check first 5 events
		if event.ID == "" {
			t.Errorf("Event %d missing ID", i)
		}
		if event.Title == "" {
			t.Errorf("Event %d missing title", i)
		}
		if event.StartTime.IsZero() {
			t.Errorf("Event %d missing start time", i)
		}
		if len(event.Alarms) == 0 {
			t.Errorf("Event %d missing alarms", i)
		}
	}
}
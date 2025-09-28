package calendar

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/venkytv/calendar-notifier/internal/models"
)

// MockProvider implements Provider for testing
type MockProvider struct {
	name         string
	providerType string
	events       []*models.Event
	calendars    []*Calendar
	healthy      bool
}

func NewMockProvider(name, providerType string) *MockProvider {
	return &MockProvider{
		name:         name,
		providerType: providerType,
		events:       []*models.Event{},
		calendars:    []*Calendar{},
		healthy:      true,
	}
}

func (m *MockProvider) Name() string { return m.name }
func (m *MockProvider) Type() string { return m.providerType }

func (m *MockProvider) Initialize(ctx context.Context, credentialsPath string) error {
	return nil
}

func (m *MockProvider) GetEvents(ctx context.Context, calendarIDs []string, from, to time.Time) ([]*models.Event, error) {
	return m.events, nil
}

func (m *MockProvider) GetCalendars(ctx context.Context) ([]*Calendar, error) {
	return m.calendars, nil
}

func (m *MockProvider) IsHealthy(ctx context.Context) error {
	if !m.healthy {
		return errors.New("provider is unhealthy")
	}
	return nil
}

func (m *MockProvider) Close() error {
	return nil
}

func (m *MockProvider) SetEvents(events []*models.Event) {
	m.events = events
}

func (m *MockProvider) SetCalendars(calendars []*Calendar) {
	m.calendars = calendars
}

func TestManagerBasicOperations(t *testing.T) {
	factory := NewDefaultProviderFactory()
	manager := NewManager(factory)

	// Test adding and getting providers
	mockProvider := NewMockProvider("test-calendar", "mock")
	manager.AddProvider("test", mockProvider)

	provider, exists := manager.GetProvider("test")
	if !exists {
		t.Error("Expected provider to exist after adding")
	}

	if provider.Name() != "test-calendar" {
		t.Errorf("Expected provider name 'test-calendar', got '%s'", provider.Name())
	}
}

func TestManagerGetAllEvents(t *testing.T) {
	factory := NewDefaultProviderFactory()
	manager := NewManager(factory)

	// Create mock provider with test data
	mockProvider := NewMockProvider("test-calendar", "mock")

	// Set up mock calendars - this is required for GetAllEvents to work
	mockProvider.SetCalendars([]*Calendar{
		{ID: "cal1", Name: "Test Calendar"},
	})

	// Set up mock events
	now := time.Now()
	testEvent := &models.Event{
		ID:        "event1",
		Title:     "Test Meeting",
		StartTime: now.Add(time.Hour),
		EndTime:   now.Add(2 * time.Hour),
	}
	mockProvider.SetEvents([]*models.Event{testEvent})

	manager.AddProvider("test", mockProvider)

	// Get all events
	ctx := context.Background()
	events, err := manager.GetAllEvents(ctx, now, now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("Failed to get events: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	if events[0].Title != "Test Meeting" {
		t.Errorf("Expected event title 'Test Meeting', got '%s'", events[0].Title)
	}

	if events[0].CalendarName != "test" {
		t.Errorf("Expected calendar name 'test', got '%s'", events[0].CalendarName)
	}
}

func TestManagerHealthCheck(t *testing.T) {
	factory := NewDefaultProviderFactory()
	manager := NewManager(factory)

	mockProvider := NewMockProvider("test-calendar", "mock")
	manager.AddProvider("test", mockProvider)

	ctx := context.Background()
	results := manager.HealthCheck(ctx)

	if len(results) != 1 {
		t.Errorf("Expected 1 health check result, got %d", len(results))
	}

	if results["test"] != nil {
		t.Errorf("Expected healthy provider, got error: %v", results["test"])
	}
}
package calendar

import (
	"context"
	"time"

	"github.com/venkytv/calendar-notifier/internal/models"
)

// Provider defines the interface that all calendar implementations must satisfy
type Provider interface {
	// Name returns the human-readable name of the calendar provider
	Name() string

	// Type returns the provider type identifier (e.g., "google", "apple")
	Type() string

	// Initialize sets up the calendar provider with the given credentials
	Initialize(ctx context.Context, credentialsPath string) error

	// GetEvents retrieves events from the specified calendar IDs within the time range
	GetEvents(ctx context.Context, calendarIDs []string, from, to time.Time) ([]*models.Event, error)

	// GetCalendars returns a list of available calendars for this provider
	GetCalendars(ctx context.Context) ([]*Calendar, error)

	// IsHealthy performs a health check on the calendar provider
	IsHealthy(ctx context.Context) error

	// Close cleans up any resources used by the provider
	Close() error
}


// ProviderFactory creates calendar providers based on configuration
type ProviderFactory interface {
	// CreateProvider creates a new calendar provider instance
	CreateProvider(providerType string) (Provider, error)

	// SupportedTypes returns a list of supported provider types
	SupportedTypes() []string
}

// Manager coordinates multiple calendar providers
type Manager struct {
	providers map[string]Provider
	factory   ProviderFactory
}

// NewManager creates a new calendar manager
func NewManager(factory ProviderFactory) *Manager {
	return &Manager{
		providers: make(map[string]Provider),
		factory:   factory,
	}
}

// AddProvider adds a calendar provider to the manager
func (m *Manager) AddProvider(name string, provider Provider) {
	m.providers[name] = provider
}

// GetProvider retrieves a calendar provider by name
func (m *Manager) GetProvider(name string) (Provider, bool) {
	provider, exists := m.providers[name]
	return provider, exists
}

// GetAllEvents retrieves events from all configured providers within the time range
func (m *Manager) GetAllEvents(ctx context.Context, from, to time.Time) ([]*models.Event, error) {
	var allEvents []*models.Event

	for name, provider := range m.providers {
		// Get available calendars from the provider
		calendars, err := provider.GetCalendars(ctx)
		if err != nil {
			return nil, err
		}

		var calendarIDs []string
		for _, cal := range calendars {
			calendarIDs = append(calendarIDs, cal.ID)
		}

		// If no calendars found, skip this provider
		if len(calendarIDs) == 0 {
			continue
		}

		events, err := provider.GetEvents(ctx, calendarIDs, from, to)
		if err != nil {
			return nil, err
		}

		// Set calendar name for each event
		for _, event := range events {
			event.CalendarName = name
		}

		allEvents = append(allEvents, events...)
	}

	return allEvents, nil
}

// Close closes all providers
func (m *Manager) Close() error {
	for _, provider := range m.providers {
		if err := provider.Close(); err != nil {
			return err
		}
	}
	return nil
}

// HealthCheck performs health checks on all providers
func (m *Manager) HealthCheck(ctx context.Context) map[string]error {
	results := make(map[string]error)
	for name, provider := range m.providers {
		results[name] = provider.IsHealthy(ctx)
	}
	return results
}
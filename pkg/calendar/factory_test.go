package calendar

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/venkytv/calendar-notifier/internal/models"
)

// TestProvider for testing factory functionality - renamed to avoid conflicts
type TestProvider struct {
	name         string
	providerType string
}

func (t *TestProvider) Name() string                                                                  { return t.name }
func (t *TestProvider) Type() string                                                                  { return t.providerType }
func (t *TestProvider) SetLogger(logger *slog.Logger)                                                 {}
func (t *TestProvider) Initialize(ctx context.Context, config string) error                          { return nil }
func (t *TestProvider) GetEvents(ctx context.Context, calendarIDs []string, from, to time.Time) ([]*models.Event, error) {
	return nil, nil
}
func (t *TestProvider) GetCalendars(ctx context.Context) ([]*Calendar, error) { return nil, nil }
func (t *TestProvider) IsHealthy(ctx context.Context) error                   { return nil }
func (t *TestProvider) Close() error                                         { return nil }

func TestNewDefaultProviderFactory(t *testing.T) {
	factory := NewDefaultProviderFactory()
	if factory == nil {
		t.Fatal("NewDefaultProviderFactory returned nil")
	}
	if factory.providers == nil {
		t.Error("providers map should be initialized")
	}
}

func TestDefaultProviderFactory_RegisterProvider(t *testing.T) {
	factory := NewDefaultProviderFactory()

	// Register a test provider
	constructor := func() Provider {
		return &TestProvider{name: "Test", providerType: "test"}
	}

	factory.RegisterProvider("test", constructor)

	// Check if provider was registered
	types := factory.SupportedTypes()
	if len(types) != 1 {
		t.Errorf("Expected 1 supported type, got %d", len(types))
	}
	if types[0] != "test" {
		t.Errorf("Expected type 'test', got %s", types[0])
	}
}

func TestDefaultProviderFactory_CreateProvider(t *testing.T) {
	factory := NewDefaultProviderFactory()

	// Try to create unregistered provider
	_, err := factory.CreateProvider("nonexistent")
	if err == nil {
		t.Error("Expected error for unregistered provider")
	}

	// Register a test provider
	constructor := func() Provider {
		return &TestProvider{name: "Test Provider", providerType: "test"}
	}
	factory.RegisterProvider("test", constructor)

	// Create the provider
	provider, err := factory.CreateProvider("test")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if provider == nil {
		t.Error("Created provider is nil")
	}
	if provider.Name() != "Test Provider" {
		t.Errorf("Expected name 'Test Provider', got %s", provider.Name())
	}
	if provider.Type() != "test" {
		t.Errorf("Expected type 'test', got %s", provider.Type())
	}
}

func TestDefaultProviderFactory_SupportedTypes(t *testing.T) {
	factory := NewDefaultProviderFactory()

	// Initially should be empty
	types := factory.SupportedTypes()
	if len(types) != 0 {
		t.Errorf("Expected 0 supported types initially, got %d", len(types))
	}

	// Register multiple providers
	factory.RegisterProvider("test1", func() Provider {
		return &TestProvider{name: "Test1", providerType: "test1"}
	})
	factory.RegisterProvider("test2", func() Provider {
		return &TestProvider{name: "Test2", providerType: "test2"}
	})

	types = factory.SupportedTypes()
	if len(types) != 2 {
		t.Errorf("Expected 2 supported types, got %d", len(types))
	}

	// Check that both types are present (order doesn't matter)
	typeSet := make(map[string]bool)
	for _, t := range types {
		typeSet[t] = true
	}
	if !typeSet["test1"] {
		t.Error("Expected 'test1' to be in supported types")
	}
	if !typeSet["test2"] {
		t.Error("Expected 'test2' to be in supported types")
	}
}

func TestDefaultProviderFactory_CreateProvider_Multiple(t *testing.T) {
	factory := NewDefaultProviderFactory()

	// Register multiple providers
	factory.RegisterProvider("provider1", func() Provider {
		return &TestProvider{name: "Provider1", providerType: "provider1"}
	})
	factory.RegisterProvider("provider2", func() Provider {
		return &TestProvider{name: "Provider2", providerType: "provider2"}
	})

	// Create provider1
	provider1, err := factory.CreateProvider("provider1")
	if err != nil {
		t.Errorf("Unexpected error creating provider1: %v", err)
	}
	if provider1.Name() != "Provider1" {
		t.Errorf("Expected name 'Provider1', got %s", provider1.Name())
	}

	// Create provider2
	provider2, err := factory.CreateProvider("provider2")
	if err != nil {
		t.Errorf("Unexpected error creating provider2: %v", err)
	}
	if provider2.Name() != "Provider2" {
		t.Errorf("Expected name 'Provider2', got %s", provider2.Name())
	}

	// Verify they are different instances
	if provider1 == provider2 {
		t.Error("Providers should be different instances")
	}
}

func TestDefaultProviderFactory_RegisterProvider_Overwrite(t *testing.T) {
	factory := NewDefaultProviderFactory()

	// Register initial provider
	factory.RegisterProvider("test", func() Provider {
		return &TestProvider{name: "Original", providerType: "test"}
	})

	// Overwrite with new provider
	factory.RegisterProvider("test", func() Provider {
		return &TestProvider{name: "Overwritten", providerType: "test"}
	})

	// Should still have only one type
	types := factory.SupportedTypes()
	if len(types) != 1 {
		t.Errorf("Expected 1 supported type, got %d", len(types))
	}

	// Should create the overwritten provider
	provider, err := factory.CreateProvider("test")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if provider.Name() != "Overwritten" {
		t.Errorf("Expected name 'Overwritten', got %s", provider.Name())
	}
}
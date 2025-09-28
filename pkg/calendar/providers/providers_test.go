package providers

import (
	"testing"

	"github.com/venkytv/calendar-notifier/pkg/calendar"
)

func TestInitializeBuiltinProviders(t *testing.T) {
	factory := calendar.NewDefaultProviderFactory()

	// Initially should have no providers
	initialTypes := factory.SupportedTypes()
	if len(initialTypes) != 0 {
		t.Errorf("Expected 0 initial providers, got %d", len(initialTypes))
	}

	// Initialize built-in providers
	InitializeBuiltinProviders(factory)

	// Should now have providers registered
	types := factory.SupportedTypes()
	if len(types) == 0 {
		t.Error("Expected providers to be registered, but got none")
	}

	// Check that specific providers are registered
	typeSet := make(map[string]bool)
	for _, providerType := range types {
		typeSet[providerType] = true
	}

	if !typeSet["caldav"] {
		t.Error("Expected 'caldav' provider to be registered")
	}
	if !typeSet["ical"] {
		t.Error("Expected 'ical' provider to be registered")
	}
}

func TestInitializeBuiltinProviders_CalDAVProvider(t *testing.T) {
	factory := calendar.NewDefaultProviderFactory()
	InitializeBuiltinProviders(factory)

	// Test that CalDAV provider can be created
	provider, err := factory.CreateProvider("caldav")
	if err != nil {
		t.Errorf("Failed to create caldav provider: %v", err)
	}
	if provider == nil {
		t.Error("CalDAV provider is nil")
	}
	if provider.Type() != "caldav" {
		t.Errorf("Expected provider type 'caldav', got %s", provider.Type())
	}
}

func TestInitializeBuiltinProviders_iCalProvider(t *testing.T) {
	factory := calendar.NewDefaultProviderFactory()
	InitializeBuiltinProviders(factory)

	// Test that iCal provider can be created
	provider, err := factory.CreateProvider("ical")
	if err != nil {
		t.Errorf("Failed to create ical provider: %v", err)
	}
	if provider == nil {
		t.Error("iCal provider is nil")
	}
	if provider.Type() != "ical" {
		t.Errorf("Expected provider type 'ical', got %s", provider.Type())
	}
}

func TestInitializeBuiltinProviders_MultipleProviders(t *testing.T) {
	factory := calendar.NewDefaultProviderFactory()
	InitializeBuiltinProviders(factory)

	// Test creating multiple instances of the same provider type
	provider1, err := factory.CreateProvider("ical")
	if err != nil {
		t.Errorf("Failed to create first ical provider: %v", err)
	}

	provider2, err := factory.CreateProvider("ical")
	if err != nil {
		t.Errorf("Failed to create second ical provider: %v", err)
	}

	// They should be different instances
	if provider1 == provider2 {
		t.Error("Expected different provider instances, got the same")
	}

	// But should have the same type
	if provider1.Type() != provider2.Type() {
		t.Errorf("Expected same provider type, got %s and %s", provider1.Type(), provider2.Type())
	}
}

func TestInitializeBuiltinProviders_Idempotent(t *testing.T) {
	factory := calendar.NewDefaultProviderFactory()

	// Call InitializeBuiltinProviders multiple times
	InitializeBuiltinProviders(factory)
	types1 := factory.SupportedTypes()

	InitializeBuiltinProviders(factory)
	types2 := factory.SupportedTypes()

	// Should have the same number of providers (no duplicates)
	if len(types1) != len(types2) {
		t.Errorf("Expected same number of providers after multiple initializations, got %d and %d", len(types1), len(types2))
	}

	// Should still be able to create providers
	provider, err := factory.CreateProvider("ical")
	if err != nil {
		t.Errorf("Failed to create provider after multiple initializations: %v", err)
	}
	if provider == nil {
		t.Error("Provider is nil after multiple initializations")
	}
}

func TestInitializeBuiltinProviders_ProviderFunctionality(t *testing.T) {
	factory := calendar.NewDefaultProviderFactory()
	InitializeBuiltinProviders(factory)

	// Test that created providers have expected basic functionality
	supportedTypes := []string{"caldav", "ical"}

	for _, providerType := range supportedTypes {
		provider, err := factory.CreateProvider(providerType)
		if err != nil {
			t.Errorf("Failed to create %s provider: %v", providerType, err)
			continue
		}

		// Test basic interface methods
		if provider.Name() == "" {
			t.Errorf("Provider %s returned empty name", providerType)
		}
		if provider.Type() != providerType {
			t.Errorf("Provider %s returned wrong type: expected %s, got %s", providerType, providerType, provider.Type())
		}

		// Test that Close doesn't error (basic interface compliance)
		if err := provider.Close(); err != nil {
			t.Errorf("Provider %s Close() failed: %v", providerType, err)
		}
	}
}
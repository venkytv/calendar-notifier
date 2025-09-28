package calendar

import (
	"fmt"
)

// DefaultProviderFactory is the default implementation of ProviderFactory
type DefaultProviderFactory struct {
	providers map[string]func() Provider
}

// NewDefaultProviderFactory creates a new default provider factory
func NewDefaultProviderFactory() *DefaultProviderFactory {
	return &DefaultProviderFactory{
		providers: make(map[string]func() Provider),
	}
}

// RegisterProvider registers a provider constructor function
func (f *DefaultProviderFactory) RegisterProvider(providerType string, constructor func() Provider) {
	f.providers[providerType] = constructor
}

// CreateProvider creates a new calendar provider instance based on the type
func (f *DefaultProviderFactory) CreateProvider(providerType string) (Provider, error) {
	constructor, exists := f.providers[providerType]
	if !exists {
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}
	return constructor(), nil
}

// SupportedTypes returns a list of supported provider types
func (f *DefaultProviderFactory) SupportedTypes() []string {
	types := make([]string, 0, len(f.providers))
	for providerType := range f.providers {
		types = append(types, providerType)
	}
	return types
}
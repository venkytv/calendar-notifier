package providers

import (
	"github.com/venkytv/calendar-notifier/pkg/calendar"
	"github.com/venkytv/calendar-notifier/pkg/calendar/google"
)

// InitializeBuiltinProviders registers all built-in calendar providers with the factory
func InitializeBuiltinProviders(factory *calendar.DefaultProviderFactory) {
	// Register Google Calendar provider
	factory.RegisterProvider("google", func() calendar.Provider {
		return google.NewProvider()
	})
}
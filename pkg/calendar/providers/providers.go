package providers

import (
	"github.com/venkytv/calendar-notifier/pkg/calendar"
	"github.com/venkytv/calendar-notifier/pkg/calendar/caldav"
	"github.com/venkytv/calendar-notifier/pkg/calendar/google"
	"github.com/venkytv/calendar-notifier/pkg/calendar/ical"
)

// InitializeBuiltinProviders registers all built-in calendar providers with the factory
func InitializeBuiltinProviders(factory *calendar.DefaultProviderFactory) {
	// Register Google Calendar provider
	factory.RegisterProvider("google", func() calendar.Provider {
		return google.NewProvider()
	})

	// Register CalDAV provider
	factory.RegisterProvider("caldav", func() calendar.Provider {
		return caldav.NewSimpleProvider()
	})

	// Register iCal provider (for public iCal URLs)
	factory.RegisterProvider("ical", func() calendar.Provider {
		return ical.NewProvider()
	})
}
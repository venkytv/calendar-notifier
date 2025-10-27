// List available Google Calendar IDs
//
// This tool lists all calendars accessible to your Google account
// so you can find the correct calendar IDs to use in your config.
//
// Usage:
//   go run examples/list-google-calendars.go \
//     /path/to/google-credentials.json \
//     /path/to/google-token.json

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/venkytv/calendar-notifier/pkg/calendar/google"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Only show warnings/errors
	}))

	if len(os.Args) != 3 {
		fmt.Println("Usage: go run list-google-calendars.go <credentials-file> <token-file>")
		os.Exit(1)
	}

	credentialsFile := os.Args[1]
	tokenFile := os.Args[2]

	// Check if files exist
	if _, err := os.Stat(credentialsFile); os.IsNotExist(err) {
		fmt.Printf("Error: Credentials file not found: %s\n", credentialsFile)
		os.Exit(1)
	}
	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		fmt.Printf("Error: Token file not found: %s\n", tokenFile)
		fmt.Println("Run google-auth-helper.go first to authenticate.")
		os.Exit(1)
	}

	ctx := context.Background()

	// Create provider without calendar ID filtering
	provider := google.NewProvider()
	provider.SetLogger(logger)
	provider.SetTokenFile(tokenFile)

	// Initialize
	if err := provider.Initialize(ctx, credentialsFile); err != nil {
		fmt.Printf("Error initializing provider: %v\n", err)
		os.Exit(1)
	}

	// Get all calendars (no filtering)
	calendars, err := provider.GetCalendars(ctx)
	if err != nil {
		fmt.Printf("Error fetching calendars: %v\n", err)
		os.Exit(1)
	}

	if len(calendars) == 0 {
		fmt.Println("No calendars found in your Google account.")
		os.Exit(0)
	}

	fmt.Printf("\nFound %d calendar(s):\n\n", len(calendars))
	fmt.Println("================================================================")

	for i, cal := range calendars {
		fmt.Printf("%d. %s\n", i+1, cal.Name)
		fmt.Printf("   ID: %s\n", cal.ID)
		if cal.Description != "" {
			fmt.Printf("   Description: %s\n", cal.Description)
		}
		if cal.Primary {
			fmt.Printf("   *** PRIMARY CALENDAR ***\n")
		}
		fmt.Printf("   Access Role: %s\n", cal.AccessRole)
		if cal.TimeZone != "" {
			fmt.Printf("   Timezone: %s\n", cal.TimeZone)
		}
		fmt.Println()
	}

	fmt.Println("================================================================")
	fmt.Println("\nTo use these calendars in your config.yaml, add:")
	fmt.Println("\ncalendar_ids:")
	for _, cal := range calendars {
		if cal.Primary {
			fmt.Printf("  - \"%s\"  # %s (PRIMARY)\n", cal.ID, cal.Name)
		} else {
			fmt.Printf("  - \"%s\"  # %s\n", cal.ID, cal.Name)
		}
	}
	fmt.Println()
}

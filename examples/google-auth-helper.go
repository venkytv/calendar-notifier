// Google Calendar OAuth2 Authentication Helper
//
// This tool helps complete the initial OAuth2 authentication for Google Calendar.
//
// Usage:
//   1. Run calendar-notifier to get the authorization URL
//   2. Visit the URL in a browser and authorize the app
//   3. Copy the authorization code
//   4. Run this tool with the credentials file, token file path, and auth code
//
// Example:
//   go run examples/google-auth-helper.go \
//     /path/to/google-credentials.json \
//     /path/to/google-token.json \
//     "YOUR_AUTH_CODE_HERE"

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/venkytv/calendar-notifier/pkg/calendar/google"
)

func main() {
	// Set up logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	if len(os.Args) < 3 {
		printUsage()
		os.Exit(1)
	}

	credentialsFile := os.Args[1]
	tokenFile := os.Args[2]

	// Check if files exist
	if _, err := os.Stat(credentialsFile); os.IsNotExist(err) {
		logger.Error("Credentials file not found", "file", credentialsFile)
		os.Exit(1)
	}

	ctx := context.Background()

	// Create provider
	provider := google.NewProvider()
	provider.SetLogger(logger)
	provider.SetTokenFile(tokenFile)

	// If auth code is provided, complete authentication
	if len(os.Args) == 4 {
		authCode := os.Args[3]

		logger.Info("Initializing Google Calendar provider")
		if err := provider.Initialize(ctx, credentialsFile); err != nil {
			// Expected to fail if no token exists, that's okay
			logger.Debug("Initialize returned error (expected)", "error", err)
		}

		logger.Info("Completing authentication with provided code")
		if err := provider.Authenticate(ctx, authCode); err != nil {
			logger.Error("Authentication failed", "error", err)
			os.Exit(1)
		}

		logger.Info("Authentication successful!", "token_file", tokenFile)
		fmt.Println("\n✓ Token saved successfully!")
		fmt.Printf("You can now start the calendar-notifier service.\n\n")
		return
	}

	// Otherwise, just show the auth URL
	logger.Info("Getting authorization URL")

	// Create a temporary token manager to get the auth URL
	tm, err := google.NewTokenManager(credentialsFile, tokenFile, logger)
	if err != nil {
		logger.Error("Failed to create token manager", "error", err)
		os.Exit(1)
	}

	authURL := tm.GetAuthURL()

	fmt.Println("\n" + separator)
	fmt.Println("GOOGLE CALENDAR AUTHORIZATION")
	fmt.Println(separator)
	fmt.Println("\nStep 1: Visit this URL in your browser:")
	fmt.Println("\n" + authURL)
	fmt.Println("\nStep 2: Sign in and authorize the application")
	fmt.Println("\nStep 3: Copy the authorization code")
	fmt.Println("\nStep 4: Run this command again with the code:")
	fmt.Printf("\n  go run examples/google-auth-helper.go \\\n    %s \\\n    %s \\\n    \"YOUR_AUTH_CODE_HERE\"\n\n",
		credentialsFile, tokenFile)
	fmt.Println(separator)
}

const separator = "================================================================"

func printUsage() {
	fmt.Println("Google Calendar OAuth2 Authentication Helper")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  # Step 1: Get authorization URL")
	fmt.Println("  go run examples/google-auth-helper.go <credentials-file> <token-file>")
	fmt.Println()
	fmt.Println("  # Step 2: Complete authentication with code")
	fmt.Println("  go run examples/google-auth-helper.go <credentials-file> <token-file> <auth-code>")
	fmt.Println()
	fmt.Println("Arguments:")
	fmt.Println("  credentials-file  Path to Google OAuth2 credentials JSON")
	fmt.Println("  token-file        Path where the token will be saved")
	fmt.Println("  auth-code         Authorization code from Google (optional)")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  go run examples/google-auth-helper.go \\")
	fmt.Println("    /etc/calendar-notifier/google-creds.json \\")
	fmt.Println("    /var/lib/calendar-notifier/google-token.json \\")
	fmt.Println("    \"4/0AY0e-g7X...\"")
	fmt.Println()
}

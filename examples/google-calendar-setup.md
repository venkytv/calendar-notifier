# Google Calendar Setup Guide

This guide explains how to set up Google Calendar integration with OAuth2 authentication for the calendar-notifier service.

## Prerequisites

- Google Cloud Console account
- Access to the Google Calendar(s) you want to monitor

## Step 1: Create Google Cloud Project and Enable Calendar API

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project (or select an existing one)
3. Navigate to **APIs & Services** > **Library**
4. Search for "Google Calendar API" and enable it
5. Navigate to **APIs & Services** > **Credentials**

## Step 2: Create OAuth2 Credentials

1. Click **Create Credentials** > **OAuth client ID**
2. If prompted, configure the OAuth consent screen:
   - Choose "Internal" for enterprise workspace, or "External" for personal use
   - Fill in required fields (App name, User support email, Developer contact)
   - Add the scope: `https://www.googleapis.com/auth/calendar.readonly`
   - Add test users if using "External" type

3. For Application type, choose **Desktop app**
4. Give it a name (e.g., "Calendar Notifier")
5. Click **Create**
6. Download the JSON credentials file and save it (e.g., `google-credentials.json`)

## Step 3: Configure calendar-notifier

Add a Google Calendar configuration to your `config.yaml`:

```yaml
calendars:
  - name: "Work Calendar"
    type: "google"
    credentials_file: "/path/to/google-credentials.json"
    token_file: "/path/to/google-token.json"  # Optional, will be created
    calendar_ids:
      - "primary"                              # Your primary calendar
      - "work@example.com"                     # Other calendars by email
    poll_interval: 5m
```

### Configuration Fields

- `credentials_file` (required): Path to the OAuth2 credentials JSON from Google Cloud Console
- `token_file` (optional): Path where OAuth2 access/refresh tokens will be stored. If not specified, defaults to `{credentials_file}.token`
- `calendar_ids` (required): List of calendar IDs to monitor
  - Use `"primary"` for your main calendar
  - Use the calendar email address for other calendars (e.g., `"work@company.com"`)

## Step 4: Initial Authentication

The first time you run the service, it will need to authenticate:

```bash
./calendar-notifier -config config.yaml
```

If authentication is required, you'll see output like:

```
Error: failed to initialize Work Calendar Google Calendar provider: authentication required:
visit this URL to authorize:
https://accounts.google.com/o/oauth2/auth?client_id=...

Then run with the authorization code
```

### Authentication Steps:

1. Copy the URL from the error message
2. Open it in a browser
3. Sign in with your Google account
4. Grant the requested permissions (read-only calendar access)
5. Copy the authorization code from the browser

6. Create a helper program to complete authentication:

```go
// tools/google-auth/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/venkytv/calendar-notifier/pkg/calendar/google"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Printf("Usage: %s <credentials-file> <token-file> <auth-code>\n", os.Args[0])
		os.Exit(1)
	}

	credentialsFile := os.Args[1]
	tokenFile := os.Args[2]
	authCode := os.Args[3]

	provider := google.NewProvider()
	provider.SetTokenFile(tokenFile)

	ctx := context.Background()

	// This will fail but give us the auth URL
	provider.Initialize(ctx, credentialsFile)

	// Complete authentication
	if err := provider.Authenticate(ctx, authCode); err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	fmt.Println("Authentication successful! Token saved.")
}
```

Or authenticate using a simple script:

```bash
# After getting the auth code, use this Go one-liner
go run -c "
package main
import (\"context\"; \"github.com/venkytv/calendar-notifier/pkg/calendar/google\"; \"log\")
func main() {
  p := google.NewProvider()
  p.SetTokenFile(\"google-token.json\")
  p.Initialize(context.Background(), \"google-credentials.json\")
  p.Authenticate(context.Background(), \"YOUR_AUTH_CODE_HERE\")
  log.Println(\"Done!\")
}
"
```

7. Once authenticated, the token file will be created and the service will start normally

## Step 5: Finding Calendar IDs

To find the ID of a specific calendar:

1. Go to [Google Calendar](https://calendar.google.com)
2. Click the three dots next to the calendar name
3. Select "Settings and sharing"
4. Scroll to "Integrate calendar"
5. Copy the "Calendar ID" (looks like an email address)

Or use `"primary"` for your main calendar.

## Token Management

- **Access tokens** expire after 1 hour
- **Refresh tokens** are used to automatically get new access tokens
- The service handles token refresh automatically
- Tokens are stored in the file specified by `token_file`
- **Keep token files secure** - they grant access to your calendar

### Token File Security

The token file contains sensitive credentials. Secure it:

```bash
chmod 600 /path/to/google-token.json
```

## Troubleshooting

### "Failed to get valid token"

- Token may have expired or been revoked
- Delete the token file and re-authenticate
- Check that the credentials file is valid

### "Failed to fetch calendar list"

- Check that the Calendar API is enabled in Google Cloud Console
- Verify the OAuth2 consent screen configuration
- Ensure the account has access to the requested calendars

### "Calendar not found"

- Verify the calendar ID is correct
- Check that the authenticated account has access to the calendar
- For shared calendars, ensure you've accepted the share invitation

## Configuration Example

Complete example with multiple calendar types:

```yaml
nats:
  url: "nats://localhost:4222"
  subject: "calendar.notifications"

calendars:
  - name: "Work Calendar"
    type: "google"
    credentials_file: "/etc/calendar-notifier/google-credentials.json"
    token_file: "/var/lib/calendar-notifier/google-token.json"
    calendar_ids:
      - "primary"
      - "team@company.com"
    poll_interval: 5m

  - name: "Personal iCal"
    type: "ical"
    url: "https://calendar.example.com/personal.ics"
    poll_interval: 10m

defaults:
  notification_intervals: [10, 30, 60]  # minutes before event
  default_severity: "normal"

logging:
  level: "info"
  format: "json"
```

## API Quotas and Rate Limits

Google Calendar API has the following limits:

- **Queries per day**: 1,000,000 (default)
- **Queries per 100 seconds per user**: 1,000

The calendar-notifier respects these limits by:
- Polling at configurable intervals (default 5 minutes)
- Using efficient queries with time ranges
- Automatically handling rate limit errors with exponential backoff

## Security Best Practices

1. **Use read-only scope**: The service only requests `calendar.readonly`
2. **Secure credential files**: Store credentials with restricted permissions (chmod 600)
3. **Use Internal OAuth consent**: For enterprise accounts, use "Internal" consent type
4. **Rotate credentials periodically**: Generate new OAuth2 credentials every 6-12 months
5. **Monitor API usage**: Check Google Cloud Console for unexpected usage patterns

## Reminders and Alarms

Google Calendar reminders are mapped to internal alarms:

| Google Method | Internal Severity | Notes |
|---------------|------------------|-------|
| `popup`       | `normal`         | Standard notification |
| `email`       | `high`           | Email reminders |
| Default       | `normal`         | 10 minutes before (if using default reminders) |

If an event uses "default reminders", the service applies a 10-minute lead time, as the actual calendar defaults can't be queried efficiently.

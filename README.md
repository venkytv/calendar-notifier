# Calendar Notifier

A Go-based calendar notification daemon that monitors multiple calendars and publishes meeting reminders as NATS messages for consumption by [calendar-siren](https://github.com/venkytv/calendar-siren).

## Features

- **Multiple Calendar Providers**:
  - **CalDAV**: Universal support for Google Calendar, Apple iCloud, Outlook, Nextcloud, and more
  - **iCal**: Direct URL-based calendar feeds
  - **Google Calendar API**: Full OAuth2-based Google Calendar integration
- **NATS Integration**: Publishes notifications in JSON format compatible with calendar-siren
- **Flexible Scheduling**: Respects event-specific alarms or uses configurable defaults
- **Multi-Calendar Coordination**: Deduplicates events across multiple calendar sources
- **Graceful Shutdown**: Proper signal handling and resource cleanup
- **Dry Run Mode**: Test configuration without publishing notifications
- **Structured Logging**: JSON and text logging with configurable levels

## Quick Start

1. **Build the application:**
   ```bash
   go build -o calendar-notifier ./cmd/calendar-notifier
   # Or use the Makefile
   make build
   ```

2. **Create configuration file:**
   ```bash
   # Choose an example configuration based on your needs
   cp examples/minimal-config.yaml config.yaml
   # Edit config.yaml with your settings
   ```

3. **Set up calendar credentials** (see Calendar Setup section below for your provider)

4. **Run the service:**
   ```bash
   ./calendar-notifier --config config.yaml
   # Or with debug output
   ./calendar-notifier --config config.yaml --debug
   ```

## Calendar Setup

The calendar notifier supports three calendar provider types:

1. **CalDAV** (Recommended): Universal support for any CalDAV-compatible calendar service
2. **iCal**: Simple URL-based calendar feeds (.ics files)
3. **Google Calendar API**: Full OAuth2-based integration with advanced features

Choose the provider that best fits your needs. See `examples/` directory for complete configuration examples.

### CalDAV Setup (Recommended - Universal)

CalDAV works with **any** calendar provider: Google, Apple, Outlook, Nextcloud, etc. It's much simpler than provider-specific APIs.

#### Google Calendar via CalDAV

1. **Enable 2-Factor Authentication** in your Google Account
2. **Generate App Password:**
   - Go to [Google Account settings](https://myaccount.google.com/)
   - Security → 2-Step Verification → App passwords
   - Generate password for "Mail" or "Other"
3. **Configure calendar:**
   ```yaml
   calendars:
     - name: "my-google-calendar"
       type: "caldav"
       url: "https://calendar.google.com/calendar/dav/your-email@gmail.com/events"
       username: "your-email@gmail.com"
       password: "your-16-character-app-password"
   ```

#### Apple iCloud Calendar

1. **Enable 2-Factor Authentication** for your Apple ID
2. **Generate App-Specific Password:**
   - Go to [appleid.apple.com](https://appleid.apple.com/)
   - Sign In → App-Specific Passwords → Generate
3. **Find your calendar URL:**
   - Calendar app → Preferences → Accounts → iCloud → Server Settings
4. **Configure calendar:**
   ```yaml
   calendars:
     - name: "my-icloud-calendar"
       type: "caldav"
       url: "https://caldav.icloud.com/published/2/YOUR_CALENDAR_ID"
       username: "your-apple-id@icloud.com"
       password: "your-app-specific-password"
   ```

#### Outlook/Office365 Calendar

```yaml
calendars:
  - name: "my-outlook-calendar"
    type: "caldav"
    url: "https://outlook.office365.com/EWS/Exchange.asmx"
    username: "your-email@outlook.com"
    password: "your-password"
```

### iCal Setup (URL-based Calendar Feeds)

iCal support allows you to subscribe to any `.ics` calendar URL:

```yaml
calendars:
  - name: "my-ical-feed"
    type: "ical"
    url: "https://calendar.example.com/public-calendar.ics"
    poll_interval: 10m
```

**Use cases:**
- Public calendar feeds
- Read-only calendar subscriptions
- Shared team calendars
- Calendar exports from services without CalDAV

### Google Calendar API Setup (OAuth2-based)

For full Google Calendar API integration with OAuth2 authentication:

**See the complete guide:** [examples/google-calendar-setup.md](examples/google-calendar-setup.md)

**Quick configuration example:**
```yaml
calendars:
  - name: "Work Calendar"
    type: "google"
    credentials_file: "/path/to/google-credentials.json"
    token_file: "/path/to/google-token.json"  # Optional
    calendar_ids:
      - "primary"                              # Your primary calendar
      - "work@example.com"                     # Other calendars
    poll_interval: 5m
```

**Setup steps:**
1. Create Google Cloud Project and enable Calendar API
2. Create OAuth2 credentials for desktop application
3. Download credentials JSON file
4. Run the service for initial authentication
5. Complete OAuth2 flow in browser

**Helper utilities:**
- `examples/google-auth-helper.go` - Complete OAuth2 authentication
- `examples/list-google-calendars.go` - List available calendars

## Configuration

The service uses YAML configuration. See the `examples/` directory for complete configuration examples:
- `minimal-config.yaml` - Basic single-calendar setup
- `multi-source-config.yaml` - Multiple calendars with different providers
- `config-google.yaml` - Google Calendar API configuration
- `caldav-config.yaml` - CalDAV configuration examples

### Configuration Format

```yaml
nats:
  url: "nats://localhost:4222"      # NATS server URL
  subject: "calendar.notifications"  # Subject to publish to

calendars:
  # Google Calendar (OAuth2)
  - name: "work-calendar"
    type: "google"
    credentials_file: "/path/to/google-credentials.json"
    token_file: "/path/to/google-token.json"  # Optional
    calendar_ids: ["primary", "team@company.com"]
    poll_interval: "5m"

  # CalDAV (Universal)
  - name: "personal-caldav"
    type: "caldav"
    url: "https://caldav.example.com/calendars/user@example.com/calendar/"
    username: "user@example.com"
    password: "app-password"
    poll_interval: "5m"

  # iCal (URL-based)
  - name: "public-events"
    type: "ical"
    url: "https://example.com/public-calendar.ics"
    poll_interval: "10m"

defaults:
  notification_intervals: [15, 5]  # Minutes before event (for events without alarms)
  default_severity: "normal"       # Default severity: low, normal, high, critical

logging:
  level: "info"    # debug, info, warn, error
  format: "json"   # json (recommended) or text
```

## Troubleshooting

### Common Issues

**"failed to connect to NATS"**
- Make sure NATS server is running: `docker run -p 4222:4222 nats:latest`
- Check the NATS URL in your configuration

**"calendar service not initialized"**
- Check that your credentials/configuration file exists and has correct permissions (`chmod 600`)
- Verify the calendar URLs are accessible
- For Google Calendar: Ensure the OAuth2 flow has been completed

**CalDAV Issues:**
- Verify the CalDAV URL is correct (check your calendar provider's documentation)
- Test credentials separately using a CalDAV client
- Check that 2FA is enabled and you're using an app password

**iCal Issues:**
- Verify the URL is publicly accessible
- Check that the URL returns a valid `.ics` file format
- Test the URL in a browser or with `curl`

**Google Calendar Issues:**
- See [examples/google-calendar-setup.md](examples/google-calendar-setup.md) for detailed troubleshooting
- Ensure the Calendar API is enabled in Google Cloud Console
- Verify OAuth2 credentials are for "Desktop application" type
- Check that the token file has valid refresh token
- Use `examples/list-google-calendars.go` to verify authentication and calendar access

### Testing Your Setup

Use dry-run mode to test without publishing:
```bash
./calendar-notifier --config config.yaml --dry-run --debug
```

This will show detailed logs without actually sending notifications.

### Running as a Service

See `examples/calendar-notifier.service` for a systemd service unit file example.

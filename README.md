# Calendar Notifier

A Go-based calendar notification daemon that monitors multiple calendars and publishes meeting reminders as NATS messages for consumption by [calendar-siren](https://github.com/venkytv/calendar-siren).

## Features

- **Multiple Calendar Support**: Monitor Google Calendar (with more providers planned)
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
   ```

2. **Create configuration file:**
   ```bash
   cp config.example.yaml config.yaml
   # Edit config.yaml with your settings
   ```

3. **Set up Google Calendar credentials** (see Google Calendar Setup section below)

4. **Run the service:**
   ```bash
   ./calendar-notifier --config config.yaml
   ```

## Calendar Setup

The calendar notifier supports multiple calendar providers through CalDAV (recommended) and Google Calendar API.

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

### Google Calendar API (Alternative)

If you prefer the full Google Calendar API (more complex but more features):

1. **Set up Google Cloud Project** and enable Calendar API
2. **Create OAuth2 credentials** for desktop application
3. **Configure:**
   ```yaml
   calendars:
     - name: "my-google-api"
       type: "google"
       credentials: "/path/to/oauth-credentials.json"
       calendar_ids: ["primary"]
   ```

## Configuration

The service uses YAML configuration. See `config.example.yaml` for a complete example or `config.minimal.yaml` for a minimal setup.

### Configuration Format

```yaml
nats:
  url: "nats://localhost:4222"      # NATS server URL
  subject: "calendar.notifications"  # Subject to publish to

calendars:
  - name: "my-calendar"                    # Unique name for this calendar
    type: "google"                         # Provider type (currently "google")
    credentials: "path/to/credentials.json" # Path to credentials file
    calendar_ids:                          # Optional: specific calendar IDs
      - "primary"
      - "work@company.com"
    poll_interval: "5m"                    # How often to check for events

defaults:
  notification_intervals: [15, 5]  # Minutes before event (for events without alarms)
  default_severity: "normal"       # Default severity: low, normal, high, critical

logging:
  level: "info"    # debug, info, warn, error
  format: "json"   # json (recommended) or text
```

## Troubleshooting

### Common Issues

**"calendar service not initialized"**
- Check that your credentials file exists and has correct permissions (`chmod 600`)
- Verify the service account has been shared access to your calendars
- Ensure the Google Calendar API is enabled in your Google Cloud project

**"failed to connect to NATS"**
- Make sure NATS server is running: `docker run -p 4222:4222 nats:latest`
- Check the NATS URL in your configuration

**"No calendars found for provider"**
- Verify calendar IDs are correct (use `"primary"` for primary calendar)
- Check that calendars are shared with the service account email
- Ensure service account has "See all event details" permission

### Testing Your Setup

Use dry-run mode to test without publishing:
```bash
./calendar-notifier --config config.yaml --dry-run --debug
```

This will show detailed logs without actually sending notifications.

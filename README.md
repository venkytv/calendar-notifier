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

## Google Calendar Setup

To access Google Calendar, you need to set up API credentials:

### 1. Create a Google Cloud Project

1. Go to the [Google Cloud Console](https://console.cloud.google.com/)
2. Click **"Create Project"** or select an existing project
3. Note your Project ID for reference

### 2. Enable Google Calendar API

1. In your Google Cloud Console, go to **APIs & Services > Library**
2. Search for **"Google Calendar API"**
3. Click on it and press **"Enable"**

### 3. Create Service Account Credentials

For server-to-server access (recommended for daemons):

1. Go to **APIs & Services > Credentials**
2. Click **"Create Credentials" > "Service Account"**
3. Fill in service account details:
   - **Name**: `calendar-notifier`
   - **Description**: `Service account for calendar notification daemon`
4. Click **"Create and Continue"**
5. Skip the optional steps and click **"Done"**

### 4. Generate and Download Credentials

1. In **APIs & Services > Credentials**, find your service account
2. Click on the service account name
3. Go to the **"Keys"** tab
4. Click **"Add Key" > "Create new key"**
5. Select **JSON** format and click **"Create"**
6. Save the downloaded JSON file securely (e.g., `google-calendar-credentials.json`)
7. Set appropriate file permissions: `chmod 600 google-calendar-credentials.json`

### 5. Share Calendars with Service Account

Since service accounts don't have their own calendars, you need to share your calendars:

1. Open [Google Calendar](https://calendar.google.com/)
2. Find the calendar you want to monitor in the left sidebar
3. Click the three dots next to the calendar name
4. Select **"Settings and sharing"**
5. Scroll to **"Share with specific people"**
6. Click **"Add people"**
7. Enter your service account email (found in the JSON file as `client_email`)
8. Set permission to **"See all event details"**
9. Click **"Send"**

### 6. Get Calendar IDs (Optional)

To monitor specific calendars, you need their IDs:

1. In Google Calendar settings, go to the calendar you want to monitor
2. Scroll to **"Integrate calendar"**
3. Copy the **Calendar ID** (looks like `abcd1234@group.calendar.google.com`)
4. For your primary calendar, you can use `"primary"` as the ID

### Alternative: OAuth2 Credentials (Interactive Setup)

If you prefer OAuth2 (requires initial interactive authentication):

1. In **APIs & Services > Credentials**, click **"Create Credentials" > "OAuth client ID"**
2. Select **"Desktop application"**
3. Name it `calendar-notifier-oauth`
4. Download the JSON file
5. The application will prompt for authentication on first run

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

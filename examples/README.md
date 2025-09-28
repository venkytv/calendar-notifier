# Configuration Examples

This directory contains example configuration files for the calendar-notifier service. Each example demonstrates different use cases and deployment scenarios.

## Configuration Files

### 1. `minimal-config.yaml`
The absolute minimum configuration required to run the service. Uses default values for most settings.

**Use case**: Quick testing or simple single-calendar setups

### 2. `basic-config.yaml`
A complete basic configuration with explicit settings and documentation.

**Use case**: Personal calendar monitoring, single iCal source

### 3. `caldav-config.yaml`
Demonstrates CalDAV server integration with authentication.

**Use case**: Corporate environments using CalDAV servers (Exchange, etc.)

### 4. `multi-source-config.yaml`
Shows how to monitor multiple calendar sources simultaneously with different poll intervals.

**Use case**: Complex environments with multiple calendar systems

### 5. `development-config.yaml`
Optimized for local development with verbose logging and frequent polling.

**Use case**: Local development and testing

### 6. `production-config.yaml`
Production-ready configuration with clustering, environment variables, and optimized settings.

**Use case**: Production deployments with high availability requirements

## Configuration Structure

### NATS Configuration

```yaml
nats:
  url: "nats://localhost:4222"           # NATS server URL(s), comma-separated for clusters
  subject: "calendar.notifications"      # NATS subject for publishing notifications
```

### Calendar Configuration

```yaml
calendars:
  - name: "calendar-name"               # Unique name for this calendar source
    type: "ical" | "caldav"             # Calendar provider type
    url: "https://example.com/cal.ics"  # Calendar URL
    poll_interval: "5m"                 # How often to check for updates (default: 5m)
    calendar_ids: []                    # Specific calendar IDs (empty = all)

    # CalDAV-specific fields (required for caldav type):
    username: "user@example.com"        # CalDAV username
    password: "password"                # CalDAV password (use env vars in production)
```

### Default Settings

```yaml
defaults:
  notification_intervals: [15, 30]     # Default notification times (minutes before event)
  default_severity: "normal"           # Default notification severity
```

### Logging Configuration

```yaml
logging:
  level: "info"                        # Log level: debug, info, warn, error
  format: "json"                       # Log format: json, text
```

## Calendar Provider Types

### iCal Provider (`type: "ical"`)

Supports public iCal URLs (.ics files). Perfect for:
- Google Calendar public URLs
- Apple iCloud shared calendars
- Any public iCal feed

**Required fields**: `url`

### CalDAV Provider (`type: "caldav"`)

Supports CalDAV protocol for authenticated calendar access. Perfect for:
- Microsoft Exchange servers
- Corporate calendar systems
- Private calendar servers

**Required fields**: `url`, `username`, `password`

## Environment Variables

For security in production environments, use environment variables for sensitive data:

```yaml
# Instead of hardcoding passwords:
password: "plaintext-password"

# Use environment variable substitution:
password: "${CALDAV_PASSWORD}"
```

Set the environment variable before running:
```bash
export CALDAV_PASSWORD="your-actual-password"
./calendar-notifier -config production-config.yaml
```

## Poll Intervals

Configure how frequently each calendar is checked:

- `30s` - Very frequent (for critical/urgent calendars)
- `1m` - High frequency (for important calendars)
- `5m` - Standard frequency (default, good for most use cases)
- `15m` - Low frequency (for less important calendars)
- `1h` - Very low frequency (for rarely changing calendars)

## Notification Intervals

Default notification intervals are applied to events that don't have their own alarms:

```yaml
defaults:
  notification_intervals: [5, 15, 30]  # Notify 5min, 15min, and 30min before events
```

Events with existing alarms will use those alarms instead of the defaults.

## NATS Message Format

The service publishes notifications in this format:

```json
{
  "title": "Meeting Title",
  "when": "2025-09-25T14:00:00+01:00",
  "lead": 10,
  "severity": "normal"
}
```

This format is compatible with [calendar-siren](https://github.com/venkytv/calendar-siren).

## Testing Your Configuration

1. Start with `minimal-config.yaml` to verify basic functionality
2. Use `development-config.yaml` for detailed debugging
3. Test with a small poll interval first, then increase for production
4. Verify NATS connectivity before adding multiple calendar sources

## Security Best Practices

1. **Never commit passwords to version control**
2. **Use environment variables for credentials in production**
3. **Restrict file permissions on configuration files** (`chmod 600`)
4. **Use app-specific passwords where available** (Google, Microsoft)
5. **Consider using secrets management systems** in production

## Troubleshooting

- **Connection issues**: Check URLs, usernames, and passwords
- **No events found**: Verify calendar URLs and date ranges
- **High CPU usage**: Increase poll intervals
- **Authentication failures**: Verify credentials and try app-specific passwords
- **NATS connection issues**: Check NATS server availability and URL format
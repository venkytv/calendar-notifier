# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a calendar notification daemon service that monitors multiple calendars and publishes meeting reminders as NATS messages. The service watches for upcoming meetings and sends notifications based on alarm triggers or default intervals.

## Architecture

### Implemented Components

1. **Calendar Integrations**
   - **Google Calendar API**: Full OAuth2-based integration (`pkg/calendar/google/`)
   - **CalDAV**: Universal CalDAV client for any compatible service (`pkg/calendar/caldav/`)
   - **iCal**: URL-based calendar feed parser (`pkg/calendar/ical/`)
   - Abstract calendar interface for extensibility (`pkg/calendar/interface.go`)
   - Provider factory pattern for dynamic provider registration (`pkg/calendar/providers/`)

2. **Message Publishing**
   - NATS client for publishing notifications (`pkg/nats/`)
   - Message formatting to match calendar-siren consumer format
   - Dry-run mode for testing without publishing

3. **Scheduling Engine**
   - Event monitoring and alarm trigger processing (`pkg/scheduler/`)
   - Default interval handling for events without alarms
   - Multi-calendar coordination and event deduplication
   - Time window filtering for relevant events

4. **Configuration Management**
   - YAML-based configuration with validation (`pkg/config/`)
   - Support for multiple calendar types with provider-specific settings
   - Default notification intervals and severity levels
   - NATS server configuration
   - Logging configuration (JSON and text formats)

## NATS Message Format

The service must publish messages to NATS in this JSON format for consumption by [calendar-siren](https://github.com/venkytv/calendar-siren):

```json
{
  "title": "Meeting Title",
  "when": "2025-09-25T14:00:00+01:00",
  "lead": 10,
  "severity": "normal"
}
```

Fields:
- `title`: Meeting name/description (string)
- `when`: Meeting timestamp in ISO 8601 format
- `lead`: Minutes before meeting to trigger alarm (integer)
- `severity`: Meeting priority level (string, optional)

## Key Design Considerations

- **Multiple Calendar Support**: Handle multiple calendar sources simultaneously
- **Alarm Processing**: Respect individual event alarm settings, fall back to defaults when none set
- **Event Filtering**: Skip events without alarms if no default intervals are configured
- **Daemon Operation**: Designed to run as a long-running background service
- **Extensible Calendar Support**: Architecture should allow easy addition of new calendar providers

## Development

This project is implemented in **Go**.

### Build Commands
```bash
go build -o calendar-notifier ./cmd/calendar-notifier
go run ./cmd/calendar-notifier
```

### Testing
```bash
go test ./...
go test -v ./...  # verbose output
go test ./pkg/calendar  # test specific package
```

### Dependencies
Use Go modules for dependency management:
```bash
go mod init github.com/venkytv/calendar-notifier
go mod tidy
```

### Project Structure
```
cmd/calendar-notifier/    # Main application entry point
pkg/
  calendar/              # Calendar integration packages
    google/             # Google Calendar OAuth2 provider
    caldav/             # CalDAV provider
    ical/               # iCal URL-based provider
    providers/          # Provider registration and factory
    interface.go        # Abstract calendar provider interface
  nats/                 # NATS publishing logic
  config/               # Configuration management
  scheduler/            # Event scheduling and alarm processing
internal/
  models/               # Internal data models
  logger/               # Structured logging setup
examples/               # Configuration examples and helper utilities
  google-calendar-setup.md        # Google Calendar setup guide
  google-auth-helper.go           # OAuth2 authentication helper
  list-google-calendars.go        # Calendar listing utility
  config-google.yaml              # Google Calendar config example
  caldav-config.yaml              # CalDAV config example
  minimal-config.yaml             # Minimal configuration
  multi-source-config.yaml        # Multi-provider example
  calendar-notifier.service       # Systemd service unit file
build/                  # Build artifacts (generated)
```

### Key Go Dependencies
- **Google Calendar API**: `google.golang.org/api/calendar/v3` - OAuth2-based calendar access
- **NATS Client**: `github.com/nats-io/nats.go` - Message publishing
- **iCal Parser**: `github.com/arran4/golang-ical` - RFC 5545 compliant iCalendar parsing
- **Configuration**: `gopkg.in/yaml.v3` - YAML configuration parsing
- **Logging**: `log/slog` (standard library) - Structured JSON and text logging
- **OAuth2**: `golang.org/x/oauth2` - OAuth2 authentication flow for Google Calendar

### Calendar Provider Types

1. **Google Calendar** (`type: "google"`):
   - OAuth2-based authentication
   - Full access to Google Calendar API features
   - Requires OAuth2 credentials JSON file
   - Token file for storing access/refresh tokens
   - Configuration fields: `credentials_file`, `token_file`, `calendar_ids`

2. **CalDAV** (`type: "caldav"`):
   - Universal support for CalDAV-compatible services
   - Username/password authentication (app passwords recommended)
   - Works with Google, Apple iCloud, Outlook, Nextcloud, etc.
   - Configuration fields: `url`, `username`, `password`

3. **iCal** (`type: "ical"`):
   - URL-based calendar feed (.ics files)
   - No authentication required for public feeds
   - Parses iCalendar format using RFC 5545 compliant parser
   - Configuration fields: `url`

### Development Workflow

When adding new features or fixing bugs:

1. **Use structured logging**: All logging should use `slog` with appropriate levels
2. **Add tests**: Unit tests should be added for new functionality
3. **Update examples**: Add example configurations if adding new features
4. **Consider all providers**: Changes to the calendar interface should work with all provider types
5. **Handle errors gracefully**: Use proper error wrapping with context
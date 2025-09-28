# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a calendar notification daemon service that monitors multiple calendars and publishes meeting reminders as NATS messages. The service watches for upcoming meetings and sends notifications based on alarm triggers or default intervals.

## Architecture

### Core Components to Implement

1. **Calendar Integrations**
   - Google Calendar API integration (priority)
   - Apple iCal support (future enhancement)
   - Abstract calendar interface for extensibility

2. **Message Publishing**
   - NATS client for publishing notifications
   - Message formatting to match calendar-siren consumer format

3. **Scheduling Engine**
   - Event monitoring and alarm trigger processing
   - Default interval handling for events without alarms
   - Multi-calendar coordination

4. **Configuration Management**
   - Calendar credentials and connection settings
   - Default notification intervals
   - NATS server configuration

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
cmd/calendar-notifier/  # Main application entry point
pkg/calendar/          # Calendar integration packages
pkg/nats/             # NATS publishing logic
pkg/config/           # Configuration management
pkg/scheduler/        # Event scheduling and alarm processing
internal/             # Internal packages
```

### Key Go Dependencies
- Google Calendar API: `google.golang.org/api/calendar/v3`
- NATS client: `github.com/nats-io/nats.go`
- Configuration: Consider `gopkg.in/yaml.v3` or `github.com/spf13/viper`
- Logging: `log/slog` (standard library) for structured JSON logging
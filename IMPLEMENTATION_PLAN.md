# Calendar Notifier Implementation Plan

## Overview
Build a Go-based calendar notification daemon that monitors multiple calendars and publishes meeting reminders as NATS messages for consumption by calendar-siren.

## Task Breakdown

### Phase 1: Foundation (Tasks 1-3)
- [x] **Set up Go module and project structure** - Initialize go.mod, create directory structure (cmd/, pkg/, internal/)
- [x] **Create configuration management package** - YAML/JSON config support for calendar credentials, NATS settings, default intervals
- [x] **Implement abstract calendar interface** - Define Calendar interface for extensible provider support

### Phase 2: Core Integrations (Tasks 4-5)
- [x] **Implement Google Calendar API integration** - Google Calendar v3 API client, authentication, event fetching
- [ ] **Create NATS message publishing package** - NATS client setup, message formatting to calendar-siren spec

### Phase 3: Business Logic (Tasks 6-8)
- [ ] **Implement scheduling engine for event monitoring** - Event polling, upcoming meeting detection, timer management
- [ ] **Create alarm processing logic** - Extract alarms from events, apply default intervals, calculate lead times
- [ ] **Implement multi-calendar coordination** - Merge events from multiple sources, deduplicate, prioritize

### Phase 4: Application Assembly (Tasks 9-10)
- [ ] **Create main application entry point** - CLI parsing, daemon setup, signal handling
- [ ] **Add comprehensive logging with structured JSON** - Structured logging with log/slog for debugging and monitoring

### Phase 5: Quality & Reliability (Tasks 11-15)
- [ ] **Create unit tests for all packages** - Test coverage for calendar, NATS, scheduler, config packages
- [ ] **Add integration tests for calendar providers** - End-to-end tests with mock calendar APIs
- [ ] **Create example configuration files** - Sample YAML configs for different deployment scenarios
- [ ] **Add graceful shutdown handling** - SIGTERM/SIGINT handling, cleanup routines
- [ ] **Implement error handling and retry logic** - Network failures, API rate limits, connection recovery

## NATS Message Contract
```json
{
  "title": "Meeting Title",
  "when": "2025-09-25T14:00:00+01:00",
  "lead": 10,
  "severity": "normal"
}
```

## Completion Tracking
Mark tasks as completed by changing `- [ ]` to `- [x]` in this document. Each task should result in:
- Working, tested code
- Documentation (inline comments)
- Unit tests where applicable
- Integration with existing components

## Key Dependencies
- `google.golang.org/api/calendar/v3` - Google Calendar API
- `github.com/nats-io/nats.go` - NATS client
- `gopkg.in/yaml.v3` or `github.com/spf13/viper` - Configuration
- `log/slog` - Structured logging

## Project Structure
```
cmd/calendar-notifier/  # Main application entry point
pkg/calendar/          # Calendar integration packages
pkg/nats/             # NATS publishing logic
pkg/config/           # Configuration management
pkg/scheduler/        # Event scheduling and alarm processing
internal/             # Internal packages
```

This plan provides a clear roadmap that can be handed off between sessions, with each task being independently completable and verifiable.
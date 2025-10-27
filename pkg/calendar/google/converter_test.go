package google

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"google.golang.org/api/calendar/v3"
)

func TestParseEventTime(t *testing.T) {
	tests := []struct {
		name        string
		eventTime   *calendar.EventDateTime
		wantErr     bool
		expectHour  int
		expectDay   int
	}{
		{
			name: "DateTime with timezone",
			eventTime: &calendar.EventDateTime{
				DateTime: "2025-10-27T14:00:00-07:00",
			},
			wantErr:    false,
			expectHour: 14,
			expectDay:  27,
		},
		{
			name: "All-day event with date only",
			eventTime: &calendar.EventDateTime{
				Date: "2025-10-27",
			},
			wantErr:   false,
			expectDay: 27,
		},
		{
			name:      "Nil event time",
			eventTime: nil,
			wantErr:   true,
		},
		{
			name:      "Empty event time",
			eventTime: &calendar.EventDateTime{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEventTime(tt.eventTime)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseEventTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Day() != tt.expectDay {
					t.Errorf("parseEventTime() day = %v, want %v", got.Day(), tt.expectDay)
				}
				if tt.expectHour > 0 && got.Hour() != tt.expectHour {
					t.Errorf("parseEventTime() hour = %v, want %v", got.Hour(), tt.expectHour)
				}
			}
		})
	}
}

func TestConvertReminderOverride(t *testing.T) {
	tests := []struct {
		name             string
		override         *calendar.EventReminder
		expectedLead     int
		expectedSeverity string
		expectedMethod   string
	}{
		{
			name: "Email reminder",
			override: &calendar.EventReminder{
				Method:  "email",
				Minutes: 30,
			},
			expectedLead:     30,
			expectedSeverity: "high",
			expectedMethod:   "email",
		},
		{
			name: "Popup reminder",
			override: &calendar.EventReminder{
				Method:  "popup",
				Minutes: 10,
			},
			expectedLead:     10,
			expectedSeverity: "normal",
			expectedMethod:   "popup",
		},
		{
			name: "Unknown method",
			override: &calendar.EventReminder{
				Method:  "custom",
				Minutes: 5,
			},
			expectedLead:     5,
			expectedSeverity: "normal",
			expectedMethod:   "custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alarm := convertReminderOverride(tt.override)

			if alarm.LeadTimeMinutes != tt.expectedLead {
				t.Errorf("convertReminderOverride() lead = %v, want %v", alarm.LeadTimeMinutes, tt.expectedLead)
			}
			if alarm.Severity != tt.expectedSeverity {
				t.Errorf("convertReminderOverride() severity = %v, want %v", alarm.Severity, tt.expectedSeverity)
			}
			if alarm.Method != tt.expectedMethod {
				t.Errorf("convertReminderOverride() method = %v, want %v", alarm.Method, tt.expectedMethod)
			}
		})
	}
}

func TestConvertReminders(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	provider := &Provider{logger: logger}

	tests := []struct {
		name          string
		event         *calendar.Event
		expectedCount int
	}{
		{
			name: "Event with no reminders",
			event: &calendar.Event{
				Id:        "test1",
				Summary:   "Test Event",
				Reminders: nil,
			},
			expectedCount: 0,
		},
		{
			name: "Event with default reminders",
			event: &calendar.Event{
				Id:      "test2",
				Summary: "Test Event",
				Reminders: &calendar.EventReminders{
					UseDefault: true,
				},
			},
			expectedCount: 1, // Should return default 10 min reminder
		},
		{
			name: "Event with override reminders",
			event: &calendar.Event{
				Id:      "test3",
				Summary: "Test Event",
				Reminders: &calendar.EventReminders{
					UseDefault: false,
					Overrides: []*calendar.EventReminder{
						{Method: "email", Minutes: 30},
						{Method: "popup", Minutes: 10},
					},
				},
			},
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alarms := provider.convertReminders(tt.event)

			if len(alarms) != tt.expectedCount {
				t.Errorf("convertReminders() count = %v, want %v", len(alarms), tt.expectedCount)
			}
		})
	}
}

func TestConvertEvent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	provider := &Provider{logger: logger}

	now := time.Now()
	nowStr := now.Format(time.RFC3339)
	laterStr := now.Add(1 * time.Hour).Format(time.RFC3339)

	tests := []struct {
		name       string
		event      *calendar.Event
		calendarID string
		wantErr    bool
	}{
		{
			name: "Valid event with all fields",
			event: &calendar.Event{
				Id:          "test-event-1",
				Summary:     "Test Meeting",
				Description: "Test description",
				Location:    "Test Location",
				Start:       &calendar.EventDateTime{DateTime: nowStr},
				End:         &calendar.EventDateTime{DateTime: laterStr},
				Created:     nowStr,
				Updated:     nowStr,
				Reminders: &calendar.EventReminders{
					UseDefault: false,
					Overrides: []*calendar.EventReminder{
						{Method: "popup", Minutes: 10},
					},
				},
			},
			calendarID: "primary",
			wantErr:    false,
		},
		{
			name: "Event with missing start time",
			event: &calendar.Event{
				Id:      "test-event-2",
				Summary: "Test Meeting",
				Start:   nil,
				End:     &calendar.EventDateTime{DateTime: laterStr},
			},
			calendarID: "primary",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := provider.convertEvent(tt.event, tt.calendarID)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if event.ID != tt.event.Id {
					t.Errorf("convertEvent() ID = %v, want %v", event.ID, tt.event.Id)
				}
				if event.Title != tt.event.Summary {
					t.Errorf("convertEvent() Title = %v, want %v", event.Title, tt.event.Summary)
				}
				if event.CalendarID != tt.calendarID {
					t.Errorf("convertEvent() CalendarID = %v, want %v", event.CalendarID, tt.calendarID)
				}
			}
		})
	}
}

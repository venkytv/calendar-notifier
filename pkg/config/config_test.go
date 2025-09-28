package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	configContent := `
nats:
  url: "nats://localhost:4222"
  subject: "calendar.notifications"

calendars:
  - name: "primary"
    type: "google"
    credentials: "/path/to/credentials.json"
    calendar_ids:
      - "primary"
    poll_interval: "5m"

defaults:
  notification_intervals: [10, 5]
  default_severity: "normal"

logging:
  level: "info"
  format: "json"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Validate loaded config
	if config.NATS.URL != "nats://localhost:4222" {
		t.Errorf("Expected NATS URL 'nats://localhost:4222', got '%s'", config.NATS.URL)
	}

	if config.NATS.Subject != "calendar.notifications" {
		t.Errorf("Expected NATS subject 'calendar.notifications', got '%s'", config.NATS.Subject)
	}

	if len(config.Calendars) != 1 {
		t.Errorf("Expected 1 calendar, got %d", len(config.Calendars))
	}

	if config.Calendars[0].Name != "primary" {
		t.Errorf("Expected calendar name 'primary', got '%s'", config.Calendars[0].Name)
	}

	if config.Calendars[0].PollInterval != 5*time.Minute {
		t.Errorf("Expected poll interval 5m, got %v", config.Calendars[0].PollInterval)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		expectErr bool
	}{
		{
			name: "valid config",
			config: Config{
				NATS: NATSConfig{
					URL:     "nats://localhost:4222",
					Subject: "test.subject",
				},
				Calendars: []CalendarConfig{
					{
						Name:        "test",
						Type:        "google",
						Credentials: "/path/to/creds",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "missing NATS URL",
			config: Config{
				NATS: NATSConfig{
					Subject: "test.subject",
				},
				Calendars: []CalendarConfig{
					{
						Name:        "test",
						Type:        "google",
						Credentials: "/path/to/creds",
					},
				},
			},
			expectErr: true,
		},
		{
			name: "missing calendars",
			config: Config{
				NATS: NATSConfig{
					URL:     "nats://localhost:4222",
					Subject: "test.subject",
				},
				Calendars: []CalendarConfig{},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			if tt.expectErr && err == nil {
				t.Error("Expected validation error, got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no validation error, got: %v", err)
			}
		})
	}
}
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	NATS      NATSConfig      `yaml:"nats"`
	Calendars []CalendarConfig `yaml:"calendars"`
	Defaults  DefaultsConfig   `yaml:"defaults"`
	Logging   LoggingConfig    `yaml:"logging"`
	Metrics   MetricsConfig    `yaml:"metrics"`
}

type NATSConfig struct {
	URL       string           `yaml:"url"`
	Subject   string           `yaml:"subject"`
	Heartbeat *HeartbeatConfig `yaml:"heartbeat"`
}

type HeartbeatConfig struct {
	Enabled      bool          `yaml:"enabled"`
	SubjectPrefix string       `yaml:"subject_prefix"`
	Service      string        `yaml:"service"`      // Service name for NATS subject (e.g., "calendar-notifier")
	Description  string        `yaml:"description"`  // Human-readable description (e.g., "Calendar Notifier")
	Interval     time.Duration `yaml:"interval"`
	GracePeriod  time.Duration `yaml:"grace_period"`
}

type CalendarConfig struct {
	Name         string        `yaml:"name"`
	Type         string        `yaml:"type"`
	CalendarIDs  []string      `yaml:"calendar_ids"`
	PollInterval time.Duration `yaml:"poll_interval"`

	// CalDAV/iCal-specific settings
	URL      string `yaml:"url"`      // CalDAV server URL or iCal URL
	Username string `yaml:"username"` // CalDAV username
	Password string `yaml:"password"` // CalDAV password

	// Event filtering
	ExcludePatterns []string `yaml:"exclude_patterns"` // Regex patterns to exclude events by title

	// Google Calendar-specific settings
	CredentialsFile string `yaml:"credentials_file"` // Path to OAuth2 credentials JSON
	TokenFile       string `yaml:"token_file"`       // Path to store OAuth2 tokens (optional)
}

type DefaultsConfig struct {
	NotificationIntervals    []int         `yaml:"notification_intervals"`
	DefaultSeverity          string        `yaml:"default_severity"`
	FinalReminderMinutes     *int          `yaml:"final_reminder_minutes"`     // If set, always send a notification this many minutes before each event
	NotificationGracePeriod  time.Duration `yaml:"notification_grace_period"`  // Grace period for late notifications (e.g., "60s")
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
}

func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

func (c *Config) validate() error {
	if c.NATS.URL == "" {
		return fmt.Errorf("NATS URL is required")
	}
	if c.NATS.Subject == "" {
		return fmt.Errorf("NATS subject is required")
	}
	if len(c.Calendars) == 0 {
		return fmt.Errorf("at least one calendar must be configured")
	}

	for i, cal := range c.Calendars {
		if cal.Name == "" {
			return fmt.Errorf("calendar[%d]: name is required", i)
		}
		if cal.Type == "" {
			return fmt.Errorf("calendar[%d]: type is required", i)
		}

		// Validate based on calendar type
		switch cal.Type {
		case "caldav":
			if cal.URL == "" {
				return fmt.Errorf("calendar[%d]: URL is required for CalDAV", i)
			}
			if cal.Username == "" {
				return fmt.Errorf("calendar[%d]: username is required for CalDAV", i)
			}
			if cal.Password == "" {
				return fmt.Errorf("calendar[%d]: password is required for CalDAV", i)
			}
		case "ical":
			if cal.URL == "" {
				return fmt.Errorf("calendar[%d]: URL is required for iCal", i)
			}
		case "google":
			if cal.CredentialsFile == "" {
				return fmt.Errorf("calendar[%d]: credentials_file is required for Google Calendar", i)
			}
			if len(cal.CalendarIDs) == 0 {
				return fmt.Errorf("calendar[%d]: at least one calendar_id is required for Google Calendar", i)
			}
		default:
			return fmt.Errorf("calendar[%d]: unsupported calendar type '%s'", i, cal.Type)
		}

		if cal.PollInterval == 0 {
			c.Calendars[i].PollInterval = 5 * time.Minute // default
		}
	}

	if c.Defaults.DefaultSeverity == "" {
		c.Defaults.DefaultSeverity = "normal"
	}

	// Set default grace period if not specified
	if c.Defaults.NotificationGracePeriod == 0 {
		c.Defaults.NotificationGracePeriod = 60 * time.Second
	}

	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}

	// Set metrics defaults
	if c.Metrics.Address == "" {
		c.Metrics.Address = "0.0.0.0"
	}
	if c.Metrics.Port == 0 {
		c.Metrics.Port = 9090
	}

	// Set heartbeat defaults if enabled
	if c.NATS.Heartbeat != nil && c.NATS.Heartbeat.Enabled {
		if c.NATS.Heartbeat.SubjectPrefix == "" {
			c.NATS.Heartbeat.SubjectPrefix = "heartbeat."
		}
		if c.NATS.Heartbeat.Service == "" {
			c.NATS.Heartbeat.Service = "calendar-notifier"
		}
		if c.NATS.Heartbeat.Description == "" {
			c.NATS.Heartbeat.Description = "Calendar Notifier"
		}
		if c.NATS.Heartbeat.Interval == 0 {
			c.NATS.Heartbeat.Interval = 1 * time.Minute
		}
		// Set grace period to 3x the interval if not specified
		if c.NATS.Heartbeat.GracePeriod == 0 {
			c.NATS.Heartbeat.GracePeriod = c.NATS.Heartbeat.Interval * 3
		}
	}

	return nil
}
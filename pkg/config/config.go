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
}

type NATSConfig struct {
	URL     string `yaml:"url"`
	Subject string `yaml:"subject"`
}

type CalendarConfig struct {
	Name         string        `yaml:"name"`
	Type         string        `yaml:"type"`
	CalendarIDs  []string      `yaml:"calendar_ids"`
	PollInterval time.Duration `yaml:"poll_interval"`

	// CalDAV-specific settings
	URL      string `yaml:"url"`      // CalDAV server URL
	Username string `yaml:"username"` // CalDAV username
	Password string `yaml:"password"` // CalDAV password
}

type DefaultsConfig struct {
	NotificationIntervals []int  `yaml:"notification_intervals"`
	DefaultSeverity      string `yaml:"default_severity"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
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

	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}

	return nil
}
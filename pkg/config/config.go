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
	Credentials  string        `yaml:"credentials"`
	CalendarIDs  []string      `yaml:"calendar_ids"`
	PollInterval time.Duration `yaml:"poll_interval"`
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
		if cal.Credentials == "" {
			return fmt.Errorf("calendar[%d]: credentials path is required", i)
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
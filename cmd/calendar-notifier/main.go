package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/venkytv/calendar-notifier/internal/models"
	"github.com/venkytv/calendar-notifier/pkg/calendar"
	"github.com/venkytv/calendar-notifier/pkg/calendar/caldav"
	"github.com/venkytv/calendar-notifier/pkg/calendar/providers"
	"github.com/venkytv/calendar-notifier/pkg/config"
	"github.com/venkytv/calendar-notifier/pkg/nats"
	"github.com/venkytv/calendar-notifier/pkg/scheduler"
)

const (
	defaultConfigPath = "config.yaml"
	gracefulTimeout   = 30 * time.Second
)

var (
	configPath = flag.String("config", defaultConfigPath, "Path to configuration file")
	version    = flag.Bool("version", false, "Print version information")
	debug      = flag.Bool("debug", false, "Enable debug logging")
	dryRun     = flag.Bool("dry-run", false, "Run without publishing notifications")
)

// Version information - can be set at build time
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func main() {
	flag.Parse()

	if *version {
		printVersion()
		os.Exit(0)
	}

	// Initialize application
	app, err := NewApp(*configPath, *debug, *dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize application: %v\n", err)
		os.Exit(1)
	}

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the application
	if err := app.Start(ctx); err != nil {
		app.logger.Error("Failed to start application", "error", err)
		os.Exit(1)
	}

	app.logger.Info("Calendar notifier started successfully")

	// Wait for shutdown signal
	sig := <-sigChan
	app.logger.Info("Received shutdown signal", "signal", sig)

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), gracefulTimeout)
	defer shutdownCancel()

	if err := app.Stop(shutdownCtx); err != nil {
		app.logger.Error("Error during shutdown", "error", err)
		os.Exit(1)
	}

	app.logger.Info("Calendar notifier stopped gracefully")
}

// App holds the main application components
type App struct {
	config           *config.Config
	logger           *slog.Logger
	calendarManager  *calendar.Manager
	natsPublisher    *nats.Publisher
	eventScheduler   *scheduler.EventScheduler
	dryRun          bool
}

// NewApp creates a new application instance
func NewApp(configPath string, debugMode, dryRun bool) (*App, error) {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Set up logger
	logger := setupLogger(cfg.Logging, debugMode)
	logger.Info("Starting calendar notifier",
		"version", Version,
		"commit", GitCommit,
		"build_time", BuildTime,
		"config_path", configPath,
		"dry_run", dryRun)

	// Create calendar manager
	factory := calendar.NewDefaultProviderFactory()
	providers.InitializeBuiltinProviders(factory)
	calendarManager := calendar.NewManagerWithCoordinator(factory, nil, logger)

	// Configure calendar providers
	for _, calendarCfg := range cfg.Calendars {
		provider, err := factory.CreateProvider(calendarCfg.Type)
		if err != nil {
			return nil, fmt.Errorf("failed to create %s calendar provider: %w", calendarCfg.Type, err)
		}

		// Set logger for the provider
		provider.SetLogger(logger)

		// Initialize provider based on type
		ctx := context.Background()
		switch calendarCfg.Type {
		case "google":
			if err := provider.Initialize(ctx, calendarCfg.Credentials); err != nil {
				return nil, fmt.Errorf("failed to initialize %s provider: %w", calendarCfg.Name, err)
			}

		case "caldav":
			// CalDAV providers need special initialization
			caldavProvider, ok := provider.(*caldav.SimpleProvider)
			if !ok {
				return nil, fmt.Errorf("failed to cast to CalDAV provider")
			}

			caldavConfig := &caldav.Config{
				URL:      calendarCfg.URL,
				Username: calendarCfg.Username,
				Password: calendarCfg.Password,
			}

			if err := caldavProvider.InitializeWithConfig(caldavConfig); err != nil {
				return nil, fmt.Errorf("failed to initialize %s CalDAV provider: %w", calendarCfg.Name, err)
			}

		case "ical":
			// iCal providers just need the URL
			if err := provider.Initialize(ctx, calendarCfg.URL); err != nil {
				return nil, fmt.Errorf("failed to initialize %s iCal provider: %w", calendarCfg.Name, err)
			}

		default:
			return nil, fmt.Errorf("unsupported provider type: %s", calendarCfg.Type)
		}

		calendarManager.AddProvider(calendarCfg.Name, provider)

		logger.Info("Configured calendar provider",
			"name", calendarCfg.Name,
			"type", calendarCfg.Type,
			"poll_interval", calendarCfg.PollInterval)
	}

	// Create NATS publisher (or mock for dry-run)
	var natsPublisher *nats.Publisher
	if !dryRun {
		natsConfig := &nats.Config{
			URL:     cfg.NATS.URL,
			Subject: cfg.NATS.Subject,
		}
		natsPublisher, err = nats.NewPublisher(natsConfig, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create NATS publisher: %w", err)
		}
	} else {
		// For dry-run, create a mock publisher that doesn't actually publish
		natsPublisher = &nats.Publisher{} // This won't work - need a proper mock
		logger.Info("Running in dry-run mode - notifications will not be published")
	}

	// Create scheduler configuration from app config
	schedulerConfig := &scheduler.Config{
		PollInterval:        5 * time.Minute, // Default - could be configurable
		LookaheadWindow:     24 * time.Hour,
		DefaultLeadTimes:    cfg.Defaults.NotificationIntervals,
		MaxConcurrentEvents: 1000,
		TimerBufferSize:     100,
	}

	// Create event scheduler
	var publisherInterface scheduler.Publisher
	if dryRun {
		publisherInterface = &DryRunPublisher{logger: logger}
	} else {
		publisherInterface = natsPublisher
	}

	eventScheduler := scheduler.NewEventScheduler(schedulerConfig, calendarManager, publisherInterface, logger)

	return &App{
		config:          cfg,
		logger:          logger,
		calendarManager: calendarManager,
		natsPublisher:   natsPublisher,
		eventScheduler:  eventScheduler,
		dryRun:          dryRun,
	}, nil
}

// Start starts the application services
func (a *App) Start(ctx context.Context) error {
	// Start event scheduler
	if err := a.eventScheduler.Start(); err != nil {
		return fmt.Errorf("failed to start event scheduler: %w", err)
	}

	// Start cleanup routine for old events
	go a.runCleanupRoutine(ctx)

	return nil
}

// Stop gracefully stops the application services
func (a *App) Stop(ctx context.Context) error {
	a.logger.Info("Shutting down application")

	// Stop event scheduler
	if err := a.eventScheduler.Stop(); err != nil {
		a.logger.Error("Error stopping event scheduler", "error", err)
	}

	// Close NATS publisher
	if a.natsPublisher != nil && !a.dryRun {
		if err := a.natsPublisher.Close(); err != nil {
			a.logger.Error("Error closing NATS publisher", "error", err)
		}
	}

	// Close calendar manager
	if err := a.calendarManager.Close(); err != nil {
		a.logger.Error("Error closing calendar manager", "error", err)
	}

	return nil
}

// runCleanupRoutine periodically cleans up old events
func (a *App) runCleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour) // Clean up every hour
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.eventScheduler.CleanupOldEvents()
		}
	}
}

// setupLogger configures the application logger
func setupLogger(cfg config.LoggingConfig, debugMode bool) *slog.Logger {
	var level slog.Level

	// Override config level if debug mode is enabled
	if debugMode {
		level = slog.LevelDebug
	} else {
		switch cfg.Level {
		case "debug":
			level = slog.LevelDebug
		case "info":
			level = slog.LevelInfo
		case "warn":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		default:
			level = slog.LevelInfo
		}
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}

	switch cfg.Format {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	case "text":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// printVersion prints version information
func printVersion() {
	fmt.Printf("Calendar Notifier %s\n", Version)
	fmt.Printf("Git Commit: %s\n", GitCommit)
	fmt.Printf("Build Time: %s\n", BuildTime)
}

// DryRunPublisher is a mock publisher for dry-run mode
type DryRunPublisher struct {
	logger *slog.Logger
}

// PublishNotification logs the notification instead of publishing it
func (p *DryRunPublisher) PublishNotification(ctx context.Context, notification *models.Notification) error {
	p.logger.Info("[DRY RUN] Would publish notification",
		"title", notification.Title,
		"when", notification.When.Format(time.RFC3339),
		"lead", notification.Lead,
		"severity", notification.Severity)
	return nil
}

// Close is a no-op for the dry-run publisher
func (p *DryRunPublisher) Close() error {
	return nil
}
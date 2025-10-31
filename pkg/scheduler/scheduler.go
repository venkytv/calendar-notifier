package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/venkytv/calendar-notifier/internal/models"
)

// CalendarManager defines the interface for calendar management
type CalendarManager interface {
	GetAllEvents(ctx context.Context, from, to time.Time) ([]*models.Event, error)
	Close() error
}

// Publisher defines the interface for notification publishing
type Publisher interface {
	PublishNotification(ctx context.Context, notification *models.Notification) error
	Close() error
}

// Config holds the scheduler configuration
type Config struct {
	PollInterval        time.Duration `yaml:"poll_interval"`
	LookaheadWindow     time.Duration `yaml:"lookahead_window"`
	DefaultLeadTimes    []int         `yaml:"default_lead_times"` // minutes
	FinalReminderMinutes *int         `yaml:"final_reminder_minutes"` // If set, always send this many minutes before event
	MaxConcurrentEvents int           `yaml:"max_concurrent_events"`
	TimerBufferSize     int           `yaml:"timer_buffer_size"`
}

// DefaultConfig returns a default scheduler configuration
func DefaultConfig() *Config {
	return &Config{
		PollInterval:        5 * time.Minute,
		LookaheadWindow:     24 * time.Hour,
		DefaultLeadTimes:    []int{15, 5}, // 15 and 5 minutes before
		MaxConcurrentEvents: 1000,
		TimerBufferSize:     100,
	}
}

// EventScheduler manages event monitoring and notification scheduling
type EventScheduler struct {
	config          *Config
	calendarManager CalendarManager
	publisher       Publisher
	logger          *slog.Logger

	// Internal state
	mu               sync.RWMutex
	scheduledEvents  map[string]*ScheduledEvent
	upcomingTimers   map[string]*time.Timer
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	running          bool

	// Channels for coordination
	eventChan     chan *models.Event
	timerChan     chan *TimerEvent
	shutdownChan  chan struct{}
}

// ScheduledEvent represents an event that has been scheduled for notifications
type ScheduledEvent struct {
	Event         *models.Event
	Notifications []*PendingNotification
	LastUpdated   time.Time
}

// PendingNotification represents a notification that is scheduled to be sent
type PendingNotification struct {
	Notification *models.Notification
	TriggerTime  time.Time
	Timer        *time.Timer
	Sent         bool
}

// TimerEvent represents a timer firing for a notification
type TimerEvent struct {
	EventID        string
	NotificationID string
	Notification   *models.Notification
}

// NewEventScheduler creates a new event scheduler
func NewEventScheduler(config *Config, calendarManager CalendarManager, publisher Publisher, logger *slog.Logger) *EventScheduler {
	if config == nil {
		config = DefaultConfig()
	}

	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &EventScheduler{
		config:          config,
		calendarManager: calendarManager,
		publisher:       publisher,
		logger:          logger,
		scheduledEvents: make(map[string]*ScheduledEvent),
		upcomingTimers:  make(map[string]*time.Timer),
		ctx:             ctx,
		cancel:          cancel,
		eventChan:       make(chan *models.Event, config.TimerBufferSize),
		timerChan:       make(chan *TimerEvent, config.TimerBufferSize),
		shutdownChan:    make(chan struct{}),
	}
}

// Start begins the event monitoring and scheduling process
func (s *EventScheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("scheduler is already running")
	}

	s.running = true
	s.logger.Info("Starting event scheduler",
		"poll_interval", s.config.PollInterval,
		"lookahead_window", s.config.LookaheadWindow)

	// Start the main polling goroutine
	s.wg.Add(1)
	go s.pollEvents()

	// Start the timer processing goroutine
	s.wg.Add(1)
	go s.processTimers()

	// Start the event processing goroutine
	s.wg.Add(1)
	go s.processEvents()

	return nil
}

// Stop gracefully stops the event scheduler
func (s *EventScheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.logger.Info("Stopping event scheduler")
	s.running = false

	// Cancel context to stop all goroutines
	s.cancel()

	// Close shutdown channel to signal immediate stop
	close(s.shutdownChan)

	// Cancel all pending timers
	for _, timer := range s.upcomingTimers {
		timer.Stop()
	}

	// Wait for all goroutines to finish
	s.wg.Wait()

	s.logger.Info("Event scheduler stopped")
	return nil
}

// pollEvents continuously polls for calendar events and schedules notifications
func (s *EventScheduler) pollEvents() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.PollInterval)
	defer ticker.Stop()

	// Initial poll
	s.performEventPoll()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.shutdownChan:
			return
		case <-ticker.C:
			s.performEventPoll()
		}
	}
}

// performEventPoll fetches events from calendars and schedules notifications
func (s *EventScheduler) performEventPoll() {
	now := time.Now()
	from := now
	to := now.Add(s.config.LookaheadWindow)

	s.logger.Debug("Polling for events",
		"from", from.Format(time.RFC3339),
		"to", to.Format(time.RFC3339))

	// Get all events from calendar manager
	events, err := s.calendarManager.GetAllEvents(s.ctx, from, to)
	if err != nil {
		s.logger.Error("Failed to fetch events", "error", err)
		return
	}

	s.logger.Debug("Fetched events", "count", len(events))

	// Process each event
	for _, event := range events {
		select {
		case s.eventChan <- event:
		case <-s.ctx.Done():
			return
		case <-s.shutdownChan:
			return
		default:
			s.logger.Warn("Event channel full, dropping event", "event_id", event.ID)
		}
	}
}

// processEvents handles incoming events and schedules notifications
func (s *EventScheduler) processEvents() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.shutdownChan:
			return
		case event := <-s.eventChan:
			s.scheduleEventNotifications(event)
		}
	}
}

// scheduleEventNotifications schedules notifications for a given event
func (s *EventScheduler) scheduleEventNotifications(event *models.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// Skip past events
	if !event.IsUpcoming(now) {
		s.logger.Debug("Skipping past event", "event_id", event.ID, "title", event.Title)
		return
	}

	// Skip events that have not been accepted
	if !event.IsAccepted() {
		s.logger.Debug("Skipping non-accepted event",
			"event_id", event.ID,
			"title", event.Title,
			"response_status", event.ResponseStatus)
		return
	}

	// Get or create scheduled event
	scheduledEvent, exists := s.scheduledEvents[event.ID]
	if !exists {
		scheduledEvent = &ScheduledEvent{
			Event:         event,
			Notifications: []*PendingNotification{},
			LastUpdated:   now,
		}
		s.scheduledEvents[event.ID] = scheduledEvent
	} else {
		// Update existing event
		scheduledEvent.Event = event
		scheduledEvent.LastUpdated = now
	}

	// Determine which alarms to use
	alarms := event.Alarms
	if len(alarms) == 0 && len(s.config.DefaultLeadTimes) > 0 {
		// Create default alarms
		for _, leadTime := range s.config.DefaultLeadTimes {
			alarm := models.Alarm{
				LeadTimeMinutes: leadTime,
				Method:          "popup",
				Severity:        "normal",
			}
			alarms = append(alarms, alarm)
		}
	}

	// Add final reminder if configured and not already present
	if s.config.FinalReminderMinutes != nil {
		finalMinutes := *s.config.FinalReminderMinutes
		hasFinalReminder := false
		for _, alarm := range alarms {
			if alarm.LeadTimeMinutes == finalMinutes {
				hasFinalReminder = true
				break
			}
		}
		if !hasFinalReminder {
			alarm := models.Alarm{
				LeadTimeMinutes: finalMinutes,
				Method:          "popup",
				Severity:        "normal",
			}
			alarms = append(alarms, alarm)
			s.logger.Debug("Added final reminder",
				"event_id", event.ID,
				"title", event.Title,
				"lead_time", finalMinutes)
		}
	}

	// Skip events with no alarms
	if len(alarms) == 0 {
		s.logger.Debug("Skipping event with no alarms", "event_id", event.ID, "title", event.Title)
		return
	}

	// Clear existing notifications for this event
	for _, pending := range scheduledEvent.Notifications {
		if pending.Timer != nil {
			pending.Timer.Stop()
		}
	}
	scheduledEvent.Notifications = []*PendingNotification{}

	// Schedule new notifications
	for i, alarm := range alarms {
		notification := models.NewNotification(event, &alarm)
		triggerTime := event.StartTime.Add(-time.Duration(alarm.LeadTimeMinutes) * time.Minute)

		// Skip notifications that should have already been sent
		if triggerTime.Before(now) || triggerTime.Equal(now) {
			s.logger.Debug("Skipping past notification",
				"event_id", event.ID,
				"trigger_time", triggerTime.Format(time.RFC3339),
				"lead_time", alarm.LeadTimeMinutes)
			continue
		}

		// Create timer for notification
		duration := triggerTime.Sub(now)
		notificationID := fmt.Sprintf("%s-%d", event.ID, i)

		timer := time.AfterFunc(duration, func() {
			timerEvent := &TimerEvent{
				EventID:        event.ID,
				NotificationID: notificationID,
				Notification:   notification,
			}

			select {
			case s.timerChan <- timerEvent:
			case <-s.ctx.Done():
			case <-s.shutdownChan:
			default:
				s.logger.Warn("Timer channel full, dropping notification",
					"event_id", event.ID,
					"notification_id", notificationID)
			}
		})

		pending := &PendingNotification{
			Notification: notification,
			TriggerTime:  triggerTime,
			Timer:        timer,
			Sent:         false,
		}

		scheduledEvent.Notifications = append(scheduledEvent.Notifications, pending)

		s.logger.Debug("Scheduled notification",
			"event_id", event.ID,
			"notification_id", notificationID,
			"title", event.Title,
			"trigger_time", triggerTime.Format(time.RFC3339),
			"lead_time", alarm.LeadTimeMinutes)
	}
}

// processTimers handles timer events and publishes notifications
func (s *EventScheduler) processTimers() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.shutdownChan:
			return
		case timerEvent := <-s.timerChan:
			s.handleTimerEvent(timerEvent)
		}
	}
}

// handleTimerEvent processes a timer event and publishes the notification
func (s *EventScheduler) handleTimerEvent(timerEvent *TimerEvent) {
	s.logger.Info("Processing timer event",
		"event_id", timerEvent.EventID,
		"notification_id", timerEvent.NotificationID,
		"title", timerEvent.Notification.Title)

	// Publish notification
	err := s.publisher.PublishNotification(s.ctx, timerEvent.Notification)
	if err != nil {
		s.logger.Error("Failed to publish notification",
			"error", err,
			"event_id", timerEvent.EventID,
			"title", timerEvent.Notification.Title)
		return
	}

	// Mark notification as sent
	s.mu.Lock()
	if scheduledEvent, exists := s.scheduledEvents[timerEvent.EventID]; exists {
		for _, pending := range scheduledEvent.Notifications {
			if pending.Notification == timerEvent.Notification {
				pending.Sent = true
				break
			}
		}
	}
	s.mu.Unlock()

	s.logger.Info("Notification published successfully",
		"event_id", timerEvent.EventID,
		"title", timerEvent.Notification.Title,
		"lead_time", timerEvent.Notification.Lead)
}

// GetScheduledEvents returns a copy of currently scheduled events
func (s *EventScheduler) GetScheduledEvents() map[string]*ScheduledEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*ScheduledEvent)
	for k, v := range s.scheduledEvents {
		// Create a shallow copy
		notifications := make([]*PendingNotification, len(v.Notifications))
		copy(notifications, v.Notifications)

		result[k] = &ScheduledEvent{
			Event:         v.Event,
			Notifications: notifications,
			LastUpdated:   v.LastUpdated,
		}
	}
	return result
}

// GetStats returns scheduler statistics
func (s *EventScheduler) GetStats() SchedulerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := SchedulerStats{
		TotalEvents:         len(s.scheduledEvents),
		PendingNotifications: 0,
		SentNotifications:   0,
		IsRunning:           s.running,
	}

	for _, scheduledEvent := range s.scheduledEvents {
		for _, notification := range scheduledEvent.Notifications {
			if notification.Sent {
				stats.SentNotifications++
			} else {
				stats.PendingNotifications++
			}
		}
	}

	return stats
}

// SchedulerStats holds statistics about the scheduler
type SchedulerStats struct {
	TotalEvents          int  `json:"total_events"`
	PendingNotifications int  `json:"pending_notifications"`
	SentNotifications    int  `json:"sent_notifications"`
	IsRunning            bool `json:"is_running"`
}

// CleanupOldEvents removes old events and their notifications from memory
func (s *EventScheduler) CleanupOldEvents() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-24 * time.Hour) // Keep events from last 24 hours

	var toDelete []string

	for eventID, scheduledEvent := range s.scheduledEvents {
		// Remove events that ended more than 24 hours ago
		if scheduledEvent.Event.EndTime.Before(cutoff) {
			// Cancel any remaining timers
			for _, notification := range scheduledEvent.Notifications {
				if notification.Timer != nil {
					notification.Timer.Stop()
				}
			}
			toDelete = append(toDelete, eventID)
		}
	}

	for _, eventID := range toDelete {
		delete(s.scheduledEvents, eventID)
		s.logger.Debug("Cleaned up old event", "event_id", eventID)
	}

	if len(toDelete) > 0 {
		s.logger.Info("Cleaned up old events", "count", len(toDelete))
	}
}
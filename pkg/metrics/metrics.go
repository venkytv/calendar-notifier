package metrics

import (
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Collector manages all Prometheus metrics for the calendar notifier
type Collector struct {
	// NATS metrics
	natsConnected   prometheus.Gauge
	natsPublished   prometheus.Counter
	natsPublishFailed prometheus.Counter
	natsPublishDuration prometheus.Histogram

	// Event processing metrics
	eventsProcessed prometheus.Counter
	eventsScheduled prometheus.Counter
	eventsSkipped   *prometheus.CounterVec
	eventsFetched   *prometheus.CounterVec
	eventsFetchFailed *prometheus.CounterVec
	eventsFetchDuration *prometheus.HistogramVec

	// Scheduler metrics
	schedulerRunning      prometheus.Gauge
	scheduledEvents       prometheus.Gauge
	pendingNotifications  prometheus.Gauge
	sentNotifications     prometheus.Gauge

	// Calendar health metrics
	calendarLastFetchTime *prometheus.GaugeVec
	calendarHealthy       *prometheus.GaugeVec

	logger *slog.Logger
}

// NewCollector creates a new metrics collector with Prometheus metrics
func NewCollector(logger *slog.Logger) *Collector {
	if logger == nil {
		logger = slog.Default()
	}

	collector := &Collector{
		// NATS metrics
		natsConnected: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "calendar_notifier_nats_connected",
			Help: "NATS connection status (1 = connected, 0 = disconnected)",
		}),
		natsPublished: promauto.NewCounter(prometheus.CounterOpts{
			Name: "calendar_notifier_nats_notifications_published_total",
			Help: "Total number of notifications successfully published to NATS",
		}),
		natsPublishFailed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "calendar_notifier_nats_notifications_failed_total",
			Help: "Total number of notification publish failures",
		}),
		natsPublishDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "calendar_notifier_nats_publish_duration_seconds",
			Help:    "Duration of NATS publish operations in seconds",
			Buckets: prometheus.DefBuckets,
		}),

		// Event processing metrics
		eventsProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "calendar_notifier_events_processed_total",
			Help: "Total number of calendar events processed",
		}),
		eventsScheduled: promauto.NewCounter(prometheus.CounterOpts{
			Name: "calendar_notifier_events_scheduled_total",
			Help: "Total number of events scheduled for notifications",
		}),
		eventsSkipped: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "calendar_notifier_events_skipped_total",
				Help: "Total number of events skipped, labeled by reason",
			},
			[]string{"reason"},
		),
		eventsFetched: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "calendar_notifier_events_fetched_total",
				Help: "Total number of events fetched from calendars, labeled by provider",
			},
			[]string{"provider", "calendar"},
		),
		eventsFetchFailed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "calendar_notifier_events_fetch_failed_total",
				Help: "Total number of failed event fetches, labeled by provider",
			},
			[]string{"provider", "calendar"},
		),
		eventsFetchDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "calendar_notifier_events_fetch_duration_seconds",
				Help:    "Duration of event fetch operations in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"provider", "calendar"},
		),

		// Scheduler metrics
		schedulerRunning: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "calendar_notifier_scheduler_running",
			Help: "Whether the event scheduler is running (1 = running, 0 = stopped)",
		}),
		scheduledEvents: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "calendar_notifier_scheduled_events",
			Help: "Current number of scheduled events being monitored",
		}),
		pendingNotifications: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "calendar_notifier_pending_notifications",
			Help: "Current number of pending notifications",
		}),
		sentNotifications: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "calendar_notifier_sent_notifications",
			Help: "Current number of sent notifications (within retention window)",
		}),

		// Calendar health metrics
		calendarLastFetchTime: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "calendar_notifier_calendar_last_fetch_timestamp_seconds",
				Help: "Unix timestamp of the last successful calendar fetch",
			},
			[]string{"provider", "calendar"},
		),
		calendarHealthy: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "calendar_notifier_calendar_healthy",
				Help: "Calendar provider health status (1 = healthy, 0 = unhealthy)",
			},
			[]string{"provider", "calendar"},
		),

		logger: logger,
	}

	logger.Info("Metrics collector initialized")
	return collector
}

// SetNATSConnected sets the NATS connection status
func (c *Collector) SetNATSConnected(connected bool) {
	if connected {
		c.natsConnected.Set(1)
		c.logger.Debug("NATS connection status: connected")
	} else {
		c.natsConnected.Set(0)
		c.logger.Debug("NATS connection status: disconnected")
	}
}

// IncNATSPublished increments the count of successfully published notifications
func (c *Collector) IncNATSPublished() {
	c.natsPublished.Inc()
}

// IncNATSPublishFailed increments the count of failed notification publishes
func (c *Collector) IncNATSPublishFailed() {
	c.natsPublishFailed.Inc()
}

// ObserveNATSPublishDuration records the duration of a NATS publish operation
func (c *Collector) ObserveNATSPublishDuration(duration time.Duration) {
	c.natsPublishDuration.Observe(duration.Seconds())
}

// IncEventsProcessed increments the count of processed events
func (c *Collector) IncEventsProcessed() {
	c.eventsProcessed.Inc()
}

// IncEventsScheduled increments the count of scheduled events
func (c *Collector) IncEventsScheduled() {
	c.eventsScheduled.Inc()
}

// IncEventsSkipped increments the count of skipped events with a reason
func (c *Collector) IncEventsSkipped(reason string) {
	c.eventsSkipped.WithLabelValues(reason).Inc()
}

// IncEventsFetched increments the count of fetched events for a calendar
func (c *Collector) IncEventsFetched(provider, calendar string, count int) {
	c.eventsFetched.WithLabelValues(provider, calendar).Add(float64(count))
}

// IncEventsFetchFailed increments the count of failed event fetches
func (c *Collector) IncEventsFetchFailed(provider, calendar string) {
	c.eventsFetchFailed.WithLabelValues(provider, calendar).Inc()
}

// ObserveEventsFetchDuration records the duration of an event fetch operation
func (c *Collector) ObserveEventsFetchDuration(provider, calendar string, duration time.Duration) {
	c.eventsFetchDuration.WithLabelValues(provider, calendar).Observe(duration.Seconds())
}

// SetSchedulerRunning sets the scheduler running status
func (c *Collector) SetSchedulerRunning(running bool) {
	if running {
		c.schedulerRunning.Set(1)
	} else {
		c.schedulerRunning.Set(0)
	}
}

// UpdateSchedulerStats updates all scheduler-related metrics
func (c *Collector) UpdateSchedulerStats(totalEvents, pendingNotifications, sentNotifications int) {
	c.scheduledEvents.Set(float64(totalEvents))
	c.pendingNotifications.Set(float64(pendingNotifications))
	c.sentNotifications.Set(float64(sentNotifications))
}

// SetCalendarLastFetchTime sets the last successful fetch time for a calendar
func (c *Collector) SetCalendarLastFetchTime(provider, calendar string, timestamp time.Time) {
	c.calendarLastFetchTime.WithLabelValues(provider, calendar).Set(float64(timestamp.Unix()))
}

// SetCalendarHealthy sets the health status for a calendar
func (c *Collector) SetCalendarHealthy(provider, calendar string, healthy bool) {
	if healthy {
		c.calendarHealthy.WithLabelValues(provider, calendar).Set(1)
	} else {
		c.calendarHealthy.WithLabelValues(provider, calendar).Set(0)
	}
}

// TimedOperation returns a function to be deferred that will record the duration
// Usage: defer c.TimedOperation(time.Now(), func(d time.Duration) { c.ObserveNATSPublishDuration(d) })()
func (c *Collector) TimedOperation(start time.Time, recordFunc func(time.Duration)) func() {
	return func() {
		duration := time.Since(start)
		recordFunc(duration)
	}
}

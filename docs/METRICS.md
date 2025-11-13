# Prometheus Metrics

The calendar-notifier service exposes Prometheus metrics on a configurable HTTP endpoint.

## Configuration

Enable metrics in your `config.yaml`:

```yaml
metrics:
  enabled: true      # Enable Prometheus metrics endpoint
  address: "0.0.0.0" # Listen address (default: 0.0.0.0)
  port: 9090         # HTTP port (default: 9090)
```

Once enabled, metrics are available at: `http://localhost:9090/metrics`

## Available Metrics

### NATS Connectivity

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `calendar_notifier_nats_connected` | Gauge | NATS connection status (1 = connected, 0 = disconnected) | - |
| `calendar_notifier_nats_notifications_published_total` | Counter | Total number of notifications successfully published | - |
| `calendar_notifier_nats_notifications_failed_total` | Counter | Total number of notification publish failures | - |
| `calendar_notifier_nats_publish_duration_seconds` | Histogram | Duration of NATS publish operations | - |

**Key metric for monitoring NATS health**: `calendar_notifier_nats_connected` will be 1 when connected, 0 when disconnected.

### Event Processing

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `calendar_notifier_events_processed_total` | Counter | Total number of calendar events processed | - |
| `calendar_notifier_events_scheduled_total` | Counter | Total number of events scheduled for notifications | - |
| `calendar_notifier_events_skipped_total` | Counter | Total number of events skipped | `reason` |
| `calendar_notifier_events_fetched_total` | Counter | Total number of events fetched from calendars | `provider`, `calendar` |
| `calendar_notifier_events_fetch_failed_total` | Counter | Total number of failed event fetches | `provider`, `calendar` |
| `calendar_notifier_events_fetch_duration_seconds` | Histogram | Duration of event fetch operations | `provider`, `calendar` |

**Skip reasons**:
- `past_event`: Event has already occurred
- `not_accepted`: Event invitation not accepted
- `no_alarms`: Event has no alarms and no default intervals configured

### Scheduler Status

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `calendar_notifier_scheduler_running` | Gauge | Whether the event scheduler is running (1 = running, 0 = stopped) | - |
| `calendar_notifier_scheduled_events` | Gauge | Current number of scheduled events being monitored | - |
| `calendar_notifier_pending_notifications` | Gauge | Current number of pending notifications | - |
| `calendar_notifier_sent_notifications` | Gauge | Current number of sent notifications (within retention window) | - |

### Calendar Health

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `calendar_notifier_calendar_last_fetch_timestamp_seconds` | Gauge | Unix timestamp of the last successful calendar fetch | `provider`, `calendar` |
| `calendar_notifier_calendar_healthy` | Gauge | Calendar provider health status (1 = healthy, 0 = unhealthy) | `provider`, `calendar` |

## Example Prometheus Queries

### Alert on NATS Disconnection
```promql
calendar_notifier_nats_connected == 0
```

### Alert on High Notification Failure Rate
```promql
rate(calendar_notifier_nats_notifications_failed_total[5m]) > 0.1
```

### Monitor Events Being Skipped
```promql
sum by (reason) (rate(calendar_notifier_events_skipped_total[5m]))
```

### Check Calendar Fetch Health
```promql
time() - calendar_notifier_calendar_last_fetch_timestamp_seconds{provider="ical",calendar="my-calendar"} > 600
```
This alerts if a specific calendar hasn't been fetched in the last 10 minutes. The metric includes labels for `provider` (e.g., "ical", "caldav", "google") and `calendar` (the calendar name from your config).

### Monitor Notification Throughput
```promql
rate(calendar_notifier_nats_notifications_published_total[5m])
```

### Track Scheduled Events Over Time
```promql
calendar_notifier_scheduled_events
```

### Monitor Calendar Fetch Latency
```promql
histogram_quantile(0.95, rate(calendar_notifier_events_fetch_duration_seconds_bucket[5m]))
```
This shows the 95th percentile of calendar fetch latency.

### Alert on Calendar Fetch Failures
```promql
rate(calendar_notifier_events_fetch_failed_total[5m]) > 0
```

### Track Events Fetched Per Calendar
```promql
sum by (provider, calendar) (rate(calendar_notifier_events_fetched_total[5m]))
```

### Check All Calendar Health Status
```promql
calendar_notifier_calendar_healthy
```
Shows 1 for healthy calendars, 0 for unhealthy ones.

## Example Grafana Dashboard

Here's a basic Grafana dashboard configuration:

### NATS Health Panel
- **Type**: Stat
- **Query**: `calendar_notifier_nats_connected`
- **Thresholds**: Red (0), Green (1)

### Notification Rate Panel
- **Type**: Graph
- **Query**: `rate(calendar_notifier_nats_notifications_published_total[5m])`
- **Unit**: ops/s

### Event Processing Panel
- **Type**: Graph
- **Queries**:
  - `rate(calendar_notifier_events_processed_total[5m])`
  - `rate(calendar_notifier_events_scheduled_total[5m])`
  - `rate(calendar_notifier_events_skipped_total[5m])`

### Scheduled Events Panel
- **Type**: Graph
- **Queries**:
  - `calendar_notifier_scheduled_events`
  - `calendar_notifier_pending_notifications`

## Testing the Metrics Endpoint

```bash
# Start the service with metrics enabled
./calendar-notifier -config config.yaml

# Check metrics endpoint
curl http://localhost:9090/metrics

# Filter for specific metrics
curl http://localhost:9090/metrics | grep calendar_notifier_nats_connected
```

## Integration with Prometheus

Add this job to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'calendar-notifier'
    static_configs:
      - targets: ['localhost:9090']
    scrape_interval: 30s
```

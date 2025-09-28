package calendar

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/venkytv/calendar-notifier/internal/models"
)

// CoordinatorConfig holds configuration for multi-calendar coordination
type CoordinatorConfig struct {
	DeduplicationEnabled bool              `yaml:"deduplication_enabled"`
	DeduplicationWindow  time.Duration     `yaml:"deduplication_window"`
	PriorityProviders    []string          `yaml:"priority_providers"`
	ProviderPriorities   map[string]int    `yaml:"provider_priorities"`
	MergeStrategies      map[string]string `yaml:"merge_strategies"` // "keep_first", "keep_last", "merge_alarms"
}

// DefaultCoordinatorConfig returns a default configuration for multi-calendar coordination
func DefaultCoordinatorConfig() *CoordinatorConfig {
	return &CoordinatorConfig{
		DeduplicationEnabled: true,
		DeduplicationWindow:  5 * time.Minute,
		PriorityProviders:    []string{"google", "outlook", "apple"},
		ProviderPriorities: map[string]int{
			"google":  1,
			"outlook": 2,
			"apple":   3,
		},
		MergeStrategies: map[string]string{
			"default": "keep_first", // Default strategy
			"alarms":  "merge_alarms",
		},
	}
}

// EventCoordinator handles multi-calendar coordination logic
type EventCoordinator struct {
	config *CoordinatorConfig
	logger *slog.Logger
}

// NewEventCoordinator creates a new event coordinator
func NewEventCoordinator(config *CoordinatorConfig, logger *slog.Logger) *EventCoordinator {
	if config == nil {
		config = DefaultCoordinatorConfig()
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &EventCoordinator{
		config: config,
		logger: logger,
	}
}

// CoordinateEvents processes events from multiple calendars with deduplication and prioritization
func (c *EventCoordinator) CoordinateEvents(events []*models.Event) ([]*models.Event, error) {
	if len(events) == 0 {
		return events, nil
	}

	c.logger.Debug("Coordinating events", "input_count", len(events))

	// Step 1: Sort events by provider priority
	c.prioritizeEventsByProvider(events)

	// Step 2: Deduplicate events if enabled
	var coordinatedEvents []*models.Event
	if c.config.DeduplicationEnabled {
		coordinatedEvents = c.deduplicateEvents(events)
	} else {
		coordinatedEvents = events
	}

	// Step 3: Sort final events by start time
	sort.Slice(coordinatedEvents, func(i, j int) bool {
		return coordinatedEvents[i].StartTime.Before(coordinatedEvents[j].StartTime)
	})

	c.logger.Info("Event coordination complete",
		"input_count", len(events),
		"output_count", len(coordinatedEvents),
		"duplicates_removed", len(events)-len(coordinatedEvents))

	return coordinatedEvents, nil
}

// prioritizeEventsByProvider sorts events based on provider priority
func (c *EventCoordinator) prioritizeEventsByProvider(events []*models.Event) {
	sort.Slice(events, func(i, j int) bool {
		providerA := strings.ToLower(events[i].CalendarName)
		providerB := strings.ToLower(events[j].CalendarName)

		priorityA, okA := c.config.ProviderPriorities[providerA]
		priorityB, okB := c.config.ProviderPriorities[providerB]

		// If both providers have priorities, use them (lower number = higher priority)
		if okA && okB {
			return priorityA < priorityB
		}

		// If only one has priority, it comes first
		if okA && !okB {
			return true
		}
		if !okA && okB {
			return false
		}

		// If neither has priority, sort alphabetically
		return providerA < providerB
	})
}

// deduplicateEvents removes duplicate events based on similarity and time window
func (c *EventCoordinator) deduplicateEvents(events []*models.Event) []*models.Event {
	if len(events) <= 1 {
		return events
	}

	var deduplicated []*models.Event
	processed := make(map[string]bool)

	for _, event := range events {
		if processed[event.ID] {
			continue
		}

		// Find all similar events within the deduplication window
		similar := c.findSimilarEvents(event, events)
		if len(similar) == 1 {
			// No duplicates found
			deduplicated = append(deduplicated, event)
			processed[event.ID] = true
		} else {
			// Merge similar events
			merged := c.mergeEvents(similar)
			deduplicated = append(deduplicated, merged)

			// Mark all similar events as processed
			for _, similarEvent := range similar {
				processed[similarEvent.ID] = true
			}

			c.logger.Debug("Merged duplicate events",
				"primary_event", event.ID,
				"total_duplicates", len(similar),
				"merged_title", merged.Title)
		}
	}

	return deduplicated
}

// findSimilarEvents finds events that are likely duplicates
func (c *EventCoordinator) findSimilarEvents(target *models.Event, allEvents []*models.Event) []*models.Event {
	var similar []*models.Event

	for _, event := range allEvents {
		if c.areEventsSimilar(target, event) {
			similar = append(similar, event)
		}
	}

	return similar
}

// areEventsSimilar determines if two events are likely the same event from different calendars
func (c *EventCoordinator) areEventsSimilar(a, b *models.Event) bool {
	// Same exact event ID
	if a.ID == b.ID {
		return true
	}

	// Check time overlap within deduplication window
	timeDiff := a.StartTime.Sub(b.StartTime)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	if timeDiff > c.config.DeduplicationWindow {
		return false
	}

	// Check title similarity (case-insensitive, trimmed)
	titleA := strings.ToLower(strings.TrimSpace(a.Title))
	titleB := strings.ToLower(strings.TrimSpace(b.Title))

	// Exact title match
	if titleA == titleB {
		return true
	}

	// Similar title (basic similarity check)
	if c.areTitlesSimilar(titleA, titleB) {
		return true
	}

	return false
}

// areTitlesSimilar performs basic string similarity check
func (c *EventCoordinator) areTitlesSimilar(a, b string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}

	// Normalize strings (lowercase, trim spaces)
	normA := strings.ToLower(strings.TrimSpace(a))
	normB := strings.ToLower(strings.TrimSpace(b))

	// Exact match after normalization
	if normA == normB {
		return true
	}

	// Check if one title contains the other (for cases like "Meeting" vs "Weekly Meeting")
	if strings.Contains(normA, normB) || strings.Contains(normB, normA) {
		return true
	}

	// Check for significant word overlap
	wordsA := strings.Fields(normA)
	wordsB := strings.Fields(normB)

	if len(wordsA) == 0 || len(wordsB) == 0 {
		return false
	}

	// Filter out common stopwords that shouldn't count toward similarity
	stopwords := map[string]bool{
		"meeting": true, "call": true, "sync": true, "standup": true,
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"with": true, "for": true, "of": true, "in": true, "on": true,
	}

	// Count meaningful common words
	commonWords := 0
	for _, wordA := range wordsA {
		if len(wordA) <= 2 || stopwords[wordA] {
			continue // Skip short words and stopwords
		}
		for _, wordB := range wordsB {
			if wordA == wordB {
				commonWords++
				break
			}
		}
	}

	// Count meaningful words in each title
	meaningfulWordsA := 0
	for _, word := range wordsA {
		if len(word) > 2 && !stopwords[word] {
			meaningfulWordsA++
		}
	}

	meaningfulWordsB := 0
	for _, word := range wordsB {
		if len(word) > 2 && !stopwords[word] {
			meaningfulWordsB++
		}
	}

	// Need meaningful words to compare
	if meaningfulWordsA == 0 || meaningfulWordsB == 0 {
		return false
	}

	// Need at least one common meaningful word
	if commonWords == 0 {
		return false
	}

	// Calculate similarity based on meaningful words
	minMeaningfulWords := meaningfulWordsA
	if meaningfulWordsB < minMeaningfulWords {
		minMeaningfulWords = meaningfulWordsB
	}

	similarity := float64(commonWords) / float64(minMeaningfulWords)
	return similarity >= 0.7 // Higher threshold for better precision
}

// mergeEvents combines multiple similar events into one
func (c *EventCoordinator) mergeEvents(events []*models.Event) *models.Event {
	if len(events) == 0 {
		return nil
	}

	if len(events) == 1 {
		return events[0]
	}

	// Use the first event as the base (highest priority due to prior sorting)
	merged := &models.Event{
		ID:           events[0].ID,
		Title:        events[0].Title,
		Description:  events[0].Description,
		StartTime:    events[0].StartTime,
		EndTime:      events[0].EndTime,
		CalendarID:   events[0].CalendarID,
		CalendarName: events[0].CalendarName,
		Location:     events[0].Location,
		CreatedAt:    events[0].CreatedAt,
		ModifiedAt:   events[0].ModifiedAt,
		Alarms:       make([]models.Alarm, 0),
	}

	// Determine merge strategy
	strategy := c.config.MergeStrategies["default"]
	if strategy == "" {
		strategy = "keep_first"
	}

	switch strategy {
	case "keep_first":
		merged.Alarms = append(merged.Alarms, events[0].Alarms...)

	case "keep_last":
		merged.Alarms = append(merged.Alarms, events[len(events)-1].Alarms...)

	case "merge_alarms":
		// Collect all unique alarms from all events
		alarmMap := make(map[string]models.Alarm)
		for _, event := range events {
			for _, alarm := range event.Alarms {
				key := fmt.Sprintf("%d-%s-%s", alarm.LeadTimeMinutes, alarm.Method, alarm.Severity)
				if _, exists := alarmMap[key]; !exists {
					alarmMap[key] = alarm
				}
			}
		}
		// Convert map back to slice
		for _, alarm := range alarmMap {
			merged.Alarms = append(merged.Alarms, alarm)
		}
		// Sort alarms by lead time (descending)
		sort.Slice(merged.Alarms, func(i, j int) bool {
			return merged.Alarms[i].LeadTimeMinutes > merged.Alarms[j].LeadTimeMinutes
		})

	default:
		// Default to keep_first
		merged.Alarms = append(merged.Alarms, events[0].Alarms...)
	}

	// Merge additional fields with preference for non-empty values
	for _, event := range events {
		if merged.Description == "" && event.Description != "" {
			merged.Description = event.Description
		}
		if merged.Location == "" && event.Location != "" {
			merged.Location = event.Location
		}
		// Use the latest modification time
		if event.ModifiedAt.After(merged.ModifiedAt) {
			merged.ModifiedAt = event.ModifiedAt
		}
	}

	// Create a combined ID for traceability
	var sourceIDs []string
	for _, event := range events {
		sourceIDs = append(sourceIDs, event.ID)
	}
	merged.ID = fmt.Sprintf("merged-%s", strings.Join(sourceIDs, "-"))

	return merged
}

// GetCoordinationStats returns statistics about the coordination process
func (c *EventCoordinator) GetCoordinationStats(originalEvents, coordinatedEvents []*models.Event) CoordinationStats {
	stats := CoordinationStats{
		OriginalCount:    len(originalEvents),
		CoordinatedCount: len(coordinatedEvents),
		DuplicatesRemoved: len(originalEvents) - len(coordinatedEvents),
		ProviderCounts:   make(map[string]int),
	}

	// Count events by provider
	for _, event := range coordinatedEvents {
		stats.ProviderCounts[event.CalendarName]++
	}

	return stats
}

// CoordinationStats holds statistics about the coordination process
type CoordinationStats struct {
	OriginalCount     int            `json:"original_count"`
	CoordinatedCount  int            `json:"coordinated_count"`
	DuplicatesRemoved int            `json:"duplicates_removed"`
	ProviderCounts    map[string]int `json:"provider_counts"`
}
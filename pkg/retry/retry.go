package retry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Config holds retry configuration
type Config struct {
	MaxAttempts      int           `yaml:"max_attempts"`
	InitialDelay     time.Duration `yaml:"initial_delay"`
	MaxDelay         time.Duration `yaml:"max_delay"`
	BackoffFactor    float64       `yaml:"backoff_factor"`
	Jitter           bool          `yaml:"jitter"`
	RetriableErrors  []string      `yaml:"retriable_errors"`
	RetriableStatuses []int         `yaml:"retriable_statuses"`
}

// DefaultConfig returns a sensible default retry configuration
func DefaultConfig() *Config {
	return &Config{
		MaxAttempts:   3,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
		RetriableErrors: []string{
			"connection refused",
			"timeout",
			"temporary failure",
			"network unreachable",
			"no such host",
			"connection reset",
		},
		RetriableStatuses: []int{
			http.StatusRequestTimeout,      // 408
			http.StatusTooManyRequests,     // 429
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout,      // 504
		},
	}
}

// Operation represents a retriable operation
type Operation func() error

// OperationWithResult represents a retriable operation that returns a result
type OperationWithResult func() (interface{}, error)

// AttemptInfo provides information about the current retry attempt
type AttemptInfo struct {
	Attempt   int
	Elapsed   time.Duration
	LastError error
}

// Retryer handles retry logic with exponential backoff
type Retryer struct {
	config *Config
	logger *slog.Logger
}

// NewRetryer creates a new Retryer with the given configuration
func NewRetryer(config *Config, logger *slog.Logger) *Retryer {
	if config == nil {
		config = DefaultConfig()
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &Retryer{
		config: config,
		logger: logger,
	}
}

// Do executes an operation with retry logic
func (r *Retryer) Do(ctx context.Context, operation Operation) error {
	var lastErr error
	start := time.Now()

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		if attempt > 1 {
			delay := r.calculateDelay(attempt - 1)
			r.logger.Debug("Retrying after delay",
				"attempt", attempt,
				"max_attempts", r.config.MaxAttempts,
				"delay", delay,
				"last_error", lastErr)

			select {
			case <-ctx.Done():
				return fmt.Errorf("retry cancelled by context: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		err := operation()
		if err == nil {
			if attempt > 1 {
				r.logger.Info("Operation succeeded after retry",
					"attempt", attempt,
					"elapsed", time.Since(start))
			}
			return nil
		}

		lastErr = err

		if !r.isRetriable(err) {
			r.logger.Debug("Error is not retriable, stopping retries",
				"attempt", attempt,
				"error", err)
			return fmt.Errorf("non-retriable error: %w", err)
		}

		if attempt == r.config.MaxAttempts {
			r.logger.Warn("Max retry attempts reached",
				"attempts", r.config.MaxAttempts,
				"elapsed", time.Since(start),
				"last_error", lastErr)
			break
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", r.config.MaxAttempts, lastErr)
}

// DoWithResult executes an operation that returns a result with retry logic
func (r *Retryer) DoWithResult(ctx context.Context, operation OperationWithResult) (interface{}, error) {
	var lastErr error
	start := time.Now()

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		if attempt > 1 {
			delay := r.calculateDelay(attempt - 1)
			r.logger.Debug("Retrying after delay",
				"attempt", attempt,
				"max_attempts", r.config.MaxAttempts,
				"delay", delay,
				"last_error", lastErr)

			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("retry cancelled by context: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		result, err := operation()
		if err == nil {
			if attempt > 1 {
				r.logger.Info("Operation succeeded after retry",
					"attempt", attempt,
					"elapsed", time.Since(start))
			}
			return result, nil
		}

		lastErr = err

		if !r.isRetriable(err) {
			r.logger.Debug("Error is not retriable, stopping retries",
				"attempt", attempt,
				"error", err)
			return nil, fmt.Errorf("non-retriable error: %w", err)
		}

		if attempt == r.config.MaxAttempts {
			r.logger.Warn("Max retry attempts reached",
				"attempts", r.config.MaxAttempts,
				"elapsed", time.Since(start),
				"last_error", lastErr)
			break
		}
	}

	return nil, fmt.Errorf("operation failed after %d attempts: %w", r.config.MaxAttempts, lastErr)
}

// calculateDelay calculates the delay before the next retry attempt
func (r *Retryer) calculateDelay(attemptNumber int) time.Duration {
	delay := float64(r.config.InitialDelay) * math.Pow(r.config.BackoffFactor, float64(attemptNumber))

	// Cap at max delay
	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}

	// Add jitter to prevent thundering herd
	if r.config.Jitter {
		jitter := rand.Float64() * 0.1 * delay // 10% jitter
		delay = delay + jitter
	}

	return time.Duration(delay)
}

// isRetriable determines if an error is retriable based on configuration
func (r *Retryer) isRetriable(err error) bool {
	if err == nil {
		return false
	}

	// Check for context cancellation/timeout - not retriable
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check for HTTP status codes
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		for _, status := range r.config.RetriableStatuses {
			if httpErr.StatusCode == status {
				return true
			}
		}
		return false
	}

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Temporary() || netErr.Timeout()
	}

	// Check for URL errors
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return r.isRetriable(urlErr.Err)
	}

	// Check error message for known retriable patterns
	errMsg := err.Error()
	for _, pattern := range r.config.RetriableErrors {
		if containsIgnoreCase(errMsg, pattern) {
			return true
		}
	}

	return false
}

// HTTPError represents an HTTP error with status code
type HTTPError struct {
	StatusCode int
	Status     string
	URL        string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s (URL: %s)", e.StatusCode, e.Status, e.URL)
}

// NewHTTPError creates a new HTTPError
func NewHTTPError(statusCode int, status, url string) *HTTPError {
	return &HTTPError{
		StatusCode: statusCode,
		Status:     status,
		URL:        url,
	}
}

// containsIgnoreCase checks if a string contains a substring (case insensitive)
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		   (s == substr ||
		    (len(s) > len(substr) &&
		     stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if toLower(s[i+j]) != toLower(substr[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func toLower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

// WithCallback wraps an operation with attempt information callback
func (r *Retryer) WithCallback(operation Operation, callback func(AttemptInfo)) Operation {
	return func() error {
		start := time.Now()
		err := operation()
		if callback != nil {
			callback(AttemptInfo{
				Attempt:   1, // This will be updated by the retry loop
				Elapsed:   time.Since(start),
				LastError: err,
			})
		}
		return err
	}
}

// Circuit breaker functionality

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	CircuitClosed CircuitBreakerState = iota
	CircuitOpen
	CircuitHalfOpen
)

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	FailureThreshold int           `yaml:"failure_threshold"`
	OpenTimeout      time.Duration `yaml:"open_timeout"`
	SuccessThreshold int           `yaml:"success_threshold"`
}

// DefaultCircuitBreakerConfig returns default circuit breaker configuration
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		FailureThreshold: 5,
		OpenTimeout:      60 * time.Second,
		SuccessThreshold: 2,
	}
}

// CircuitBreaker implements circuit breaker pattern
type CircuitBreaker struct {
	config       *CircuitBreakerConfig
	state        CircuitBreakerState
	failures     int
	successes    int
	lastFailure  time.Time
	logger       *slog.Logger
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig, logger *slog.Logger) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &CircuitBreaker{
		config: config,
		state:  CircuitClosed,
		logger: logger,
	}
}

// Execute executes an operation through the circuit breaker
func (cb *CircuitBreaker) Execute(operation Operation) error {
	// Check if circuit should transition from open to half-open
	if cb.state == CircuitOpen && time.Since(cb.lastFailure) > cb.config.OpenTimeout {
		cb.state = CircuitHalfOpen
		cb.successes = 0
		cb.logger.Info("Circuit breaker transitioning to half-open")
	}

	// Reject if circuit is open
	if cb.state == CircuitOpen {
		return fmt.Errorf("circuit breaker is open")
	}

	err := operation()

	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()

		if cb.state == CircuitHalfOpen {
			cb.state = CircuitOpen
			cb.logger.Warn("Circuit breaker opening due to failure in half-open state")
		} else if cb.failures >= cb.config.FailureThreshold {
			cb.state = CircuitOpen
			cb.logger.Warn("Circuit breaker opening due to failure threshold",
				"failures", cb.failures,
				"threshold", cb.config.FailureThreshold)
		}

		return err
	}

	// Success
	cb.failures = 0
	cb.successes++

	if cb.state == CircuitHalfOpen && cb.successes >= cb.config.SuccessThreshold {
		cb.state = CircuitClosed
		cb.logger.Info("Circuit breaker closing after successful operations",
			"successes", cb.successes)
	}

	return nil
}
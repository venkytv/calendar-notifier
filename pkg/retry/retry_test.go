package retry

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"testing"
	"time"
)

func TestNewRetryer(t *testing.T) {
	retryer := NewRetryer(nil, nil)
	if retryer == nil {
		t.Fatal("Expected non-nil retryer")
	}
	if retryer.config == nil {
		t.Error("Expected default config when nil provided")
	}
	if retryer.logger == nil {
		t.Error("Expected default logger when nil provided")
	}
}

func TestRetryer_Do_Success(t *testing.T) {
	retryer := NewRetryer(DefaultConfig(), slog.Default())

	called := 0
	operation := func() error {
		called++
		return nil
	}

	err := retryer.Do(context.Background(), operation)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if called != 1 {
		t.Errorf("Expected operation to be called once, got %d", called)
	}
}

func TestRetryer_Do_SuccessAfterRetry(t *testing.T) {
	config := &Config{
		MaxAttempts:   3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
		Jitter:        false,
		RetriableStatuses: []int{500},
	}
	retryer := NewRetryer(config, slog.Default())

	called := 0
	operation := func() error {
		called++
		if called < 3 {
			return NewHTTPError(500, "Internal Server Error", "http://test.com")
		}
		return nil
	}

	start := time.Now()
	err := retryer.Do(context.Background(), operation)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if called != 3 {
		t.Errorf("Expected operation to be called 3 times, got %d", called)
	}

	// Should have some delay due to retries
	expectedMinDelay := config.InitialDelay + config.InitialDelay*time.Duration(config.BackoffFactor)
	if elapsed < expectedMinDelay {
		t.Errorf("Expected elapsed time to be at least %v, got %v", expectedMinDelay, elapsed)
	}
}

func TestRetryer_Do_MaxAttemptsReached(t *testing.T) {
	config := &Config{
		MaxAttempts:   2,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
		Jitter:        false,
		RetriableStatuses: []int{500},
	}
	retryer := NewRetryer(config, slog.Default())

	called := 0
	operation := func() error {
		called++
		return NewHTTPError(500, "Internal Server Error", "http://test.com")
	}

	err := retryer.Do(context.Background(), operation)
	if err == nil {
		t.Error("Expected error after max attempts")
	}
	if called != 2 {
		t.Errorf("Expected operation to be called 2 times, got %d", called)
	}
}

func TestRetryer_Do_NonRetriableError(t *testing.T) {
	config := &Config{
		MaxAttempts:   3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
		RetriableStatuses: []int{500},
	}
	retryer := NewRetryer(config, slog.Default())

	called := 0
	operation := func() error {
		called++
		return NewHTTPError(404, "Not Found", "http://test.com")
	}

	err := retryer.Do(context.Background(), operation)
	if err == nil {
		t.Error("Expected error for non-retriable error")
	}
	if called != 1 {
		t.Errorf("Expected operation to be called once for non-retriable error, got %d", called)
	}
}

func TestRetryer_Do_ContextCancellation(t *testing.T) {
	config := &Config{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      1000 * time.Millisecond,
		BackoffFactor: 2.0,
		RetriableStatuses: []int{500},
	}
	retryer := NewRetryer(config, slog.Default())

	called := 0
	operation := func() error {
		called++
		return NewHTTPError(500, "Internal Server Error", "http://test.com")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := retryer.Do(ctx, operation)
	if err == nil {
		t.Error("Expected context cancellation error")
	}
	if called > 2 {
		t.Errorf("Expected operation to be called at most 2 times due to context cancellation, got %d", called)
	}
}

func TestRetryer_DoWithResult_Success(t *testing.T) {
	retryer := NewRetryer(DefaultConfig(), slog.Default())

	called := 0
	operation := func() (interface{}, error) {
		called++
		return "success", nil
	}

	result, err := retryer.DoWithResult(context.Background(), operation)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if result.(string) != "success" {
		t.Errorf("Expected result 'success', got '%s'", result.(string))
	}
	if called != 1 {
		t.Errorf("Expected operation to be called once, got %d", called)
	}
}

func TestRetryer_DoWithResult_SuccessAfterRetry(t *testing.T) {
	config := &Config{
		MaxAttempts:   3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
		Jitter:        false,
		RetriableStatuses: []int{500},
	}
	retryer := NewRetryer(config, slog.Default())

	called := 0
	operation := func() (interface{}, error) {
		called++
		if called < 3 {
			return nil, NewHTTPError(500, "Internal Server Error", "http://test.com")
		}
		return "success", nil
	}

	result, err := retryer.DoWithResult(context.Background(), operation)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if result.(string) != "success" {
		t.Errorf("Expected result 'success', got '%s'", result.(string))
	}
	if called != 3 {
		t.Errorf("Expected operation to be called 3 times, got %d", called)
	}
}

func TestIsRetriable(t *testing.T) {
	retryer := NewRetryer(DefaultConfig(), slog.Default())

	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			expected: false,
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: false,
		},
		{
			name:     "HTTP 500",
			err:      NewHTTPError(500, "Internal Server Error", "http://test.com"),
			expected: true,
		},
		{
			name:     "HTTP 404",
			err:      NewHTTPError(404, "Not Found", "http://test.com"),
			expected: false,
		},
		{
			name:     "HTTP 429",
			err:      NewHTTPError(429, "Too Many Requests", "http://test.com"),
			expected: true,
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "timeout",
			err:      errors.New("request timeout"),
			expected: true,
		},
		{
			name:     "generic error",
			err:      errors.New("some other error"),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := retryer.isRetriable(tc.err)
			if result != tc.expected {
				t.Errorf("Expected isRetriable(%v) to be %t, got %t", tc.err, tc.expected, result)
			}
		})
	}
}

func TestCalculateDelay(t *testing.T) {
	config := &Config{
		InitialDelay:  1 * time.Second,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        false,
	}
	retryer := NewRetryer(config, slog.Default())

	testCases := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 2 * time.Second},  // 1 * 2^1
		{2, 4 * time.Second},  // 1 * 2^2
		{3, 8 * time.Second},  // 1 * 2^3
		{4, 10 * time.Second}, // capped at MaxDelay
		{5, 10 * time.Second}, // capped at MaxDelay
	}

	for _, tc := range testCases {
		result := retryer.calculateDelay(tc.attempt)
		if result != tc.expected {
			t.Errorf("Expected delay for attempt %d to be %v, got %v", tc.attempt, tc.expected, result)
		}
	}
}

func TestCircuitBreaker_Execute_Success(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig(), slog.Default())

	called := 0
	operation := func() error {
		called++
		return nil
	}

	err := cb.Execute(operation)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if called != 1 {
		t.Errorf("Expected operation to be called once, got %d", called)
	}
	if cb.state != CircuitClosed {
		t.Errorf("Expected circuit to remain closed, got state %v", cb.state)
	}
}

func TestCircuitBreaker_Execute_FailureThresholdReached(t *testing.T) {
	config := &CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenTimeout:      100 * time.Millisecond,
		SuccessThreshold: 1,
	}
	cb := NewCircuitBreaker(config, slog.Default())

	operation := func() error {
		return errors.New("test error")
	}

	// First failure - should remain closed
	err := cb.Execute(operation)
	if err == nil {
		t.Error("Expected error")
	}
	if cb.state != CircuitClosed {
		t.Errorf("Expected circuit to remain closed after first failure, got state %v", cb.state)
	}

	// Second failure - should open
	err = cb.Execute(operation)
	if err == nil {
		t.Error("Expected error")
	}
	if cb.state != CircuitOpen {
		t.Errorf("Expected circuit to open after reaching failure threshold, got state %v", cb.state)
	}

	// Third attempt - should be rejected immediately
	err = cb.Execute(operation)
	if err == nil {
		t.Error("Expected error")
	}
	if err.Error() != "circuit breaker is open" {
		t.Errorf("Expected circuit breaker rejection error, got: %v", err)
	}
}

func TestCircuitBreaker_Execute_HalfOpenTransition(t *testing.T) {
	config := &CircuitBreakerConfig{
		FailureThreshold: 1,
		OpenTimeout:      50 * time.Millisecond,
		SuccessThreshold: 1,
	}
	cb := NewCircuitBreaker(config, slog.Default())

	// Cause failure to open circuit
	failureOp := func() error {
		return errors.New("test error")
	}
	cb.Execute(failureOp)

	if cb.state != CircuitOpen {
		t.Errorf("Expected circuit to be open, got state %v", cb.state)
	}

	// Wait for open timeout
	time.Sleep(60 * time.Millisecond)

	// Next call should transition to half-open
	successOp := func() error {
		return nil
	}

	err := cb.Execute(successOp)
	if err != nil {
		t.Errorf("Expected no error in half-open state, got: %v", err)
	}
	if cb.state != CircuitClosed {
		t.Errorf("Expected circuit to close after success in half-open, got state %v", cb.state)
	}
}

func TestHTTPError(t *testing.T) {
	err := NewHTTPError(404, "Not Found", "http://test.com")

	if err.StatusCode != 404 {
		t.Errorf("Expected status code 404, got %d", err.StatusCode)
	}
	if err.Status != "Not Found" {
		t.Errorf("Expected status 'Not Found', got '%s'", err.Status)
	}
	if err.URL != "http://test.com" {
		t.Errorf("Expected URL 'http://test.com', got '%s'", err.URL)
	}

	expectedError := "HTTP 404: Not Found (URL: http://test.com)"
	if err.Error() != expectedError {
		t.Errorf("Expected error message '%s', got '%s'", expectedError, err.Error())
	}
}

// Mock network error for testing
type mockNetError struct {
	temporary bool
	timeout   bool
	msg       string
}

func (e *mockNetError) Error() string   { return e.msg }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return e.temporary }

func TestRetryer_NetworkErrors(t *testing.T) {
	retryer := NewRetryer(DefaultConfig(), slog.Default())

	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "temporary network error",
			err:      &mockNetError{temporary: true, timeout: false, msg: "temporary error"},
			expected: true,
		},
		{
			name:     "timeout network error",
			err:      &mockNetError{temporary: false, timeout: true, msg: "timeout error"},
			expected: true,
		},
		{
			name:     "permanent network error",
			err:      &mockNetError{temporary: false, timeout: false, msg: "permanent error"},
			expected: false,
		},
		{
			name:     "url error with temporary underlying error",
			err:      &url.Error{Op: "Get", URL: "http://test.com", Err: &mockNetError{temporary: true, timeout: false, msg: "temporary"}},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := retryer.isRetriable(tc.err)
			if result != tc.expected {
				t.Errorf("Expected isRetriable(%v) to be %t, got %t", tc.err, tc.expected, result)
			}
		})
	}
}
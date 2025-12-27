package errors

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// RetryConfig defines the configuration for retry operations
type RetryConfig struct {
	// MaxAttempts is the maximum number of retry attempts
	MaxAttempts int
	// InitialBackoff is the initial backoff duration
	InitialBackoff time.Duration
	// MaxBackoff is the maximum backoff duration
	MaxBackoff time.Duration
	// BackoffFactor is the factor by which the backoff increases
	BackoffFactor float64
	// Jitter is the randomness factor for backoff (0.0 to 1.0)
	Jitter float64
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:    3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     60 * time.Second,
		BackoffFactor:  2.0,
		Jitter:         0.2,
	}
}

// NetworkRetryConfig returns a retry configuration optimized for network operations
func NetworkRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:    5,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     60 * time.Second,
		BackoffFactor:  2.0,
		Jitter:         0.2,
	}
}

// APIRetryConfig returns a retry configuration optimized for API calls
func APIRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:    3,
		InitialBackoff: 2 * time.Second,
		MaxBackoff:     30 * time.Second,
		BackoffFactor:  2.0,
		Jitter:         0.1,
	}
}

// SSHRetryConfig returns a retry configuration optimized for SSH operations
func SSHRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:    4,
		InitialBackoff: 5 * time.Second,
		MaxBackoff:     60 * time.Second,
		BackoffFactor:  2.0,
		Jitter:         0.1,
	}
}

// RetryFunc is a function that will be retried
type RetryFunc func() error

// RetryWithContextFunc is a function that will be retried with context
type RetryWithContextFunc func(ctx context.Context) error

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	var dockErr *DockBridgeError
	if As(err, &dockErr) {
		return dockErr.IsRetryable()
	}

	return false
}

// Retry executes the given function with retries based on the provided configuration
func Retry(fn RetryFunc, config *RetryConfig) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var err error
	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}

		if !IsRetryableError(err) {
			return err
		}

		if attempt == config.MaxAttempts-1 {
			break
		}

		backoff := calculateBackoff(attempt, config)
		time.Sleep(backoff)
	}

	return NewError(
		ErrCategoryNetwork,
		"MAX_RETRIES_EXCEEDED",
		"Maximum retry attempts exceeded",
		err,
		false,
	)
}

// RetryWithContext executes the given function with retries based on the provided configuration
// and respects context cancellation
func RetryWithContext(ctx context.Context, fn RetryWithContextFunc, config *RetryConfig) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var err error
	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return NewError(
				ErrCategoryNetwork,
				"CONTEXT_CANCELED",
				"Operation canceled",
				ctx.Err(),
				false,
			)
		default:
			err = fn(ctx)
			if err == nil {
				return nil
			}

			if !IsRetryableError(err) {
				return err
			}

			if attempt == config.MaxAttempts-1 {
				break
			}

			backoff := calculateBackoff(attempt, config)
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return NewError(
					ErrCategoryNetwork,
					"CONTEXT_CANCELED",
					"Operation canceled during backoff",
					ctx.Err(),
					false,
				)
			case <-timer.C:
				// Continue with next attempt
			}
		}
	}

	return NewError(
		ErrCategoryNetwork,
		"MAX_RETRIES_EXCEEDED",
		"Maximum retry attempts exceeded",
		err,
		false,
	)
}

// calculateBackoff calculates the backoff duration with jitter
func calculateBackoff(attempt int, config *RetryConfig) time.Duration {
	backoff := float64(config.InitialBackoff) * math.Pow(config.BackoffFactor, float64(attempt))
	if backoff > float64(config.MaxBackoff) {
		backoff = float64(config.MaxBackoff)
	}

	// Apply jitter
	if config.Jitter > 0 {
		jitter := rand.Float64() * config.Jitter * backoff // #nosec G404
		backoff = backoff - (config.Jitter * backoff / 2) + jitter
	}

	return time.Duration(backoff)
}

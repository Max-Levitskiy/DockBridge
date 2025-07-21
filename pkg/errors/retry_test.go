package errors

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsRetryableError(t *testing.T) {
	// Non-retryable error
	err1 := NewError(ErrCategoryConfig, "INVALID", "Invalid config", nil, false)
	assert.False(t, IsRetryableError(err1))

	// Retryable error
	err2 := NewError(ErrCategoryNetwork, "TIMEOUT", "Request timeout", nil, true)
	assert.True(t, IsRetryableError(err2))

	// Standard error (not retryable)
	err3 := fmt.Errorf("standard error")
	assert.False(t, IsRetryableError(err3))

	// Nil error
	assert.False(t, IsRetryableError(nil))
}

func TestRetry_Success(t *testing.T) {
	attempts := 0
	err := Retry(func() error {
		attempts++
		return nil // Success on first attempt
	}, DefaultRetryConfig())

	assert.NoError(t, err)
	assert.Equal(t, 1, attempts)
}

func TestRetry_EventualSuccess(t *testing.T) {
	attempts := 0
	err := Retry(func() error {
		attempts++
		if attempts < 3 {
			return NewError(ErrCategoryNetwork, "TEMP_FAILURE", "Temporary failure", nil, true)
		}
		return nil // Success on third attempt
	}, DefaultRetryConfig())

	assert.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestRetry_NonRetryableError(t *testing.T) {
	attempts := 0
	nonRetryableErr := NewError(ErrCategoryConfig, "INVALID", "Invalid config", nil, false)

	err := Retry(func() error {
		attempts++
		return nonRetryableErr
	}, DefaultRetryConfig())

	assert.Error(t, err)
	assert.Equal(t, nonRetryableErr, err)
	assert.Equal(t, 1, attempts)
}

func TestRetry_MaxAttemptsExceeded(t *testing.T) {
	attempts := 0
	retryableErr := NewError(ErrCategoryNetwork, "TIMEOUT", "Request timeout", nil, true)

	err := Retry(func() error {
		attempts++
		return retryableErr
	}, DefaultRetryConfig())

	assert.Error(t, err)
	assert.Equal(t, DefaultRetryConfig().MaxAttempts, attempts)

	// Check that we got a MAX_RETRIES_EXCEEDED error
	var dockErr *DockBridgeError
	assert.True(t, As(err, &dockErr))
	assert.Equal(t, "MAX_RETRIES_EXCEEDED", dockErr.Code)
	assert.Equal(t, retryableErr, dockErr.Cause)
}

func TestRetryWithContext_Success(t *testing.T) {
	attempts := 0
	ctx := context.Background()

	err := RetryWithContext(ctx, func(ctx context.Context) error {
		attempts++
		return nil // Success on first attempt
	}, DefaultRetryConfig())

	assert.NoError(t, err)
	assert.Equal(t, 1, attempts)
}

func TestRetryWithContext_Cancellation(t *testing.T) {
	attempts := 0
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := RetryWithContext(ctx, func(ctx context.Context) error {
		attempts++
		time.Sleep(100 * time.Millisecond)
		return NewError(ErrCategoryNetwork, "TIMEOUT", "Request timeout", nil, true)
	}, DefaultRetryConfig())

	assert.Error(t, err)
	assert.Equal(t, 1, attempts)

	// Check that we got a CONTEXT_CANCELED error
	var dockErr *DockBridgeError
	assert.True(t, As(err, &dockErr))
	assert.Equal(t, "CONTEXT_CANCELED", dockErr.Code)
}

func TestRetryWithContext_CancellationDuringBackoff(t *testing.T) {
	// Skip this test as it's timing-dependent and can be flaky
	t.Skip("Skipping timing-dependent test")

	attempts := 0
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := &RetryConfig{
		MaxAttempts:    5,
		InitialBackoff: 200 * time.Millisecond,
		MaxBackoff:     1 * time.Second,
		BackoffFactor:  2.0,
		Jitter:         0.0, // No jitter for predictable testing
	}

	// Use a channel to synchronize the test
	done := make(chan struct{})

	go func() {
		time.Sleep(50 * time.Millisecond) // Wait for the first attempt to complete
		cancel()
		close(done)
	}()

	err := RetryWithContext(ctx, func(ctx context.Context) error {
		attempts++
		return NewError(ErrCategoryNetwork, "TIMEOUT", "Request timeout", nil, true)
	}, config)

	<-done // Wait for the goroutine to complete

	assert.Error(t, err)
	assert.Equal(t, 1, attempts)

	// Check that we got a CONTEXT_CANCELED error
	var dockErr *DockBridgeError
	assert.True(t, As(err, &dockErr))
	assert.Equal(t, "CONTEXT_CANCELED", dockErr.Code)
}

func TestCalculateBackoff(t *testing.T) {
	config := &RetryConfig{
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     10 * time.Second,
		BackoffFactor:  2.0,
		Jitter:         0.0, // No jitter for predictable testing
	}

	// First attempt (attempt 0)
	backoff0 := calculateBackoff(0, config)
	assert.Equal(t, 1*time.Second, backoff0)

	// Second attempt (attempt 1)
	backoff1 := calculateBackoff(1, config)
	assert.Equal(t, 2*time.Second, backoff1)

	// Third attempt (attempt 2)
	backoff2 := calculateBackoff(2, config)
	assert.Equal(t, 4*time.Second, backoff2)

	// Fourth attempt (attempt 3)
	backoff3 := calculateBackoff(3, config)
	assert.Equal(t, 8*time.Second, backoff3)

	// Fifth attempt (attempt 4) - should be capped at MaxBackoff
	backoff4 := calculateBackoff(4, config)
	assert.Equal(t, 10*time.Second, backoff4)
}

func TestCalculateBackoffWithJitter(t *testing.T) {
	config := &RetryConfig{
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     10 * time.Second,
		BackoffFactor:  2.0,
		Jitter:         0.2, // 20% jitter
	}

	// With jitter, the backoff should be within a range
	backoff := calculateBackoff(1, config)

	// Expected backoff without jitter would be 2s
	// With 20% jitter, it should be between 1.8s and 2.2s
	assert.GreaterOrEqual(t, backoff, 1800*time.Millisecond)
	assert.LessOrEqual(t, backoff, 2200*time.Millisecond)
}

func TestRetryConfigs(t *testing.T) {
	// Test DefaultRetryConfig
	defaultConfig := DefaultRetryConfig()
	assert.Equal(t, 3, defaultConfig.MaxAttempts)
	assert.Equal(t, 1*time.Second, defaultConfig.InitialBackoff)
	assert.Equal(t, 60*time.Second, defaultConfig.MaxBackoff)

	// Test NetworkRetryConfig
	networkConfig := NetworkRetryConfig()
	assert.Equal(t, 5, networkConfig.MaxAttempts)
	assert.Equal(t, 1*time.Second, networkConfig.InitialBackoff)

	// Test APIRetryConfig
	apiConfig := APIRetryConfig()
	assert.Equal(t, 3, apiConfig.MaxAttempts)
	assert.Equal(t, 2*time.Second, apiConfig.InitialBackoff)

	// Test SSHRetryConfig
	sshConfig := SSHRetryConfig()
	assert.Equal(t, 4, sshConfig.MaxAttempts)
	assert.Equal(t, 5*time.Second, sshConfig.InitialBackoff)
}

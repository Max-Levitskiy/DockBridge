package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewError(t *testing.T) {
	cause := errors.New("underlying error")
	err := NewError(ErrCategoryNetwork, "CONN_FAILED", "Connection failed", cause, true)

	assert.Equal(t, ErrCategoryNetwork, err.Category)
	assert.Equal(t, "CONN_FAILED", err.Code)
	assert.Equal(t, "Connection failed", err.Message)
	assert.Equal(t, cause, err.Cause)
	assert.True(t, err.Retryable)
	assert.False(t, err.Timestamp.IsZero())
}

func TestDockBridgeError_Error(t *testing.T) {
	// Test error without cause
	err1 := NewError(ErrCategoryDocker, "CMD_FAILED", "Docker command failed", nil, false)
	expected1 := "[docker:CMD_FAILED] Docker command failed"
	assert.Equal(t, expected1, err1.Error())

	// Test error with cause
	cause := errors.New("connection refused")
	err2 := NewError(ErrCategoryNetwork, "CONN_REFUSED", "Network error", cause, true)
	expected2 := "[network:CONN_REFUSED] Network error: connection refused"
	assert.Equal(t, expected2, err2.Error())
}

func TestDockBridgeError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := NewError(ErrCategorySSH, "AUTH_FAILED", "Authentication failed", cause, false)

	assert.Equal(t, cause, err.Unwrap())
}

func TestDockBridgeError_IsRetryable(t *testing.T) {
	retryableErr := NewError(ErrCategoryNetwork, "TIMEOUT", "Request timeout", nil, true)
	nonRetryableErr := NewError(ErrCategoryConfig, "INVALID", "Invalid config", nil, false)

	assert.True(t, retryableErr.IsRetryable())
	assert.False(t, nonRetryableErr.IsRetryable())
}

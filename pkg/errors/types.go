package errors

import (
	"fmt"
	"time"
)

// ErrorCategory represents different categories of errors in the system
type ErrorCategory string

const (
	ErrCategoryNetwork    ErrorCategory = "network"
	ErrCategoryHetzner    ErrorCategory = "hetzner"
	ErrCategoryDocker     ErrorCategory = "docker"
	ErrCategorySSH        ErrorCategory = "ssh"
	ErrCategoryConfig     ErrorCategory = "config"
	ErrCategoryLockDetect ErrorCategory = "lock_detection"
)

// DockBridgeError represents a structured error with category and retry information
type DockBridgeError struct {
	Category  ErrorCategory `json:"category"`
	Code      string        `json:"code"`
	Message   string        `json:"message"`
	Cause     error         `json:"cause,omitempty"`
	Retryable bool          `json:"retryable"`
	Timestamp time.Time     `json:"timestamp"`
}

// Error implements the error interface
func (e *DockBridgeError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s:%s] %s: %v", e.Category, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s:%s] %s", e.Category, e.Code, e.Message)
}

// Unwrap returns the underlying cause error
func (e *DockBridgeError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns whether the error is retryable
func (e *DockBridgeError) IsRetryable() bool {
	return e.Retryable
}

// NewError creates a new DockBridgeError
func NewError(category ErrorCategory, code, message string, cause error, retryable bool) *DockBridgeError {
	return &DockBridgeError{
		Category:  category,
		Code:      code,
		Message:   message,
		Cause:     cause,
		Retryable: retryable,
		Timestamp: time.Now(),
	}
}

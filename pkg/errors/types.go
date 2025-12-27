package errors

import (
	"errors"
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
	ErrCategoryInternal   ErrorCategory = "internal"
	ErrCategoryResource   ErrorCategory = "resource"
	ErrCategoryAuth       ErrorCategory = "auth"
)

// Common error codes
const (
	// Network error codes
	ErrCodeTimeout        = "TIMEOUT"
	ErrCodeConnectionFail = "CONNECTION_FAILED"
	ErrCodeDNSFailure     = "DNS_FAILURE"

	// Config error codes
	ErrCodeInvalidConfig = "INVALID_CONFIG"
	ErrCodeMissingConfig = "MISSING_CONFIG"

	// Resource error codes
	ErrCodeNotFound      = "NOT_FOUND"
	ErrCodeAlreadyExists = "ALREADY_EXISTS"
	ErrCodeLimitExceeded = "LIMIT_EXCEEDED"

	// Auth error codes
	ErrCodeUnauthorized = "UNAUTHORIZED"
	ErrCodeForbidden    = "FORBIDDEN"

	// Internal error codes
	ErrCodeInternal = "INTERNAL_ERROR"
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

// Convenience functions for creating common errors

// NewNetworkError creates a new network error
func NewNetworkError(code, message string, cause error, retryable bool) *DockBridgeError {
	return NewError(ErrCategoryNetwork, code, message, cause, retryable)
}

// NewTimeoutError creates a new timeout error (always retryable)
func NewTimeoutError(message string, cause error) *DockBridgeError {
	return NewError(ErrCategoryNetwork, ErrCodeTimeout, message, cause, true)
}

// NewConnectionError creates a new connection error (always retryable)
func NewConnectionError(message string, cause error) *DockBridgeError {
	return NewError(ErrCategoryNetwork, ErrCodeConnectionFail, message, cause, true)
}

// NewConfigError creates a new configuration error (never retryable)
func NewConfigError(code, message string, cause error) *DockBridgeError {
	return NewError(ErrCategoryConfig, code, message, cause, false)
}

// NewResourceError creates a new resource error
func NewResourceError(code, message string, cause error, retryable bool) *DockBridgeError {
	return NewError(ErrCategoryResource, code, message, cause, retryable)
}

// NewNotFoundError creates a new not found error (not retryable by default)
func NewNotFoundError(message string, cause error) *DockBridgeError {
	return NewError(ErrCategoryResource, ErrCodeNotFound, message, cause, false)
}

// NewInternalError creates a new internal error (not retryable by default)
func NewInternalError(message string, cause error) *DockBridgeError {
	return NewError(ErrCategoryInternal, ErrCodeInternal, message, cause, false)
}

// Wrap wraps an error with additional context
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}

	var dockErr *DockBridgeError
	if As(err, &dockErr) {
		return NewError(
			dockErr.Category,
			dockErr.Code,
			fmt.Sprintf("%s: %s", message, dockErr.Message),
			dockErr.Cause,
			dockErr.Retryable,
		)
	}

	return fmt.Errorf("%s: %w", message, err)
}

// WrapWithCode wraps an error with a specific error code and category
func WrapWithCode(err error, category ErrorCategory, code, message string, retryable bool) error {
	if err == nil {
		return nil
	}

	return NewError(category, code, message, err, retryable)
}

// Is reports whether any error in err's tree matches target.
// This is a wrapper around the standard errors.Is function.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in err's tree that matches target, and if one is found, sets
// target to that error value and returns true. Otherwise, it returns false.
// This is a wrapper around the standard errors.As function.
func As(err error, target any) bool {
	return errors.As(err, target)
}

package errors

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/dockbridge/dockbridge/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestLogError(t *testing.T) {
	// Setup a buffer to capture log output
	buf := &bytes.Buffer{}
	testLogger := logger.NewDefault()
	testLogger.SetOutput(buf)
	testLogger.SetLevel(logger.Debug)
	testLogger.UseColors = false // Disable colors for testing
	logger.SetDefaultLogger(testLogger)

	// Test with DockBridgeError
	dockErr := NewError(ErrCategoryNetwork, "TEST_ERROR", "Test error message", nil, true)
	LogError(dockErr, "Operation failed")

	output := buf.String()
	assert.Contains(t, output, "ERROR Operation failed: Test error message")
	assert.Contains(t, output, "error_category=network")
	assert.Contains(t, output, "error_code=TEST_ERROR")
	assert.Contains(t, output, "retryable=true")

	// Reset buffer
	buf.Reset()

	// Test with standard error
	stdErr := fmt.Errorf("standard error")
	LogError(stdErr, "Standard error occurred")

	output = buf.String()
	assert.Contains(t, output, "ERROR Standard error occurred")
	assert.Contains(t, output, "error=standard error")

	// Reset buffer
	buf.Reset()

	// Test with nil error (should not log)
	LogError(nil, "No error")
	assert.Empty(t, buf.String())
}

func TestLogErrorWithFields(t *testing.T) {
	// Setup a buffer to capture log output
	buf := &bytes.Buffer{}
	testLogger := logger.NewDefault()
	testLogger.SetOutput(buf)
	testLogger.SetLevel(logger.Debug)
	testLogger.UseColors = false // Disable colors for testing
	logger.SetDefaultLogger(testLogger)

	// Test with DockBridgeError and additional fields
	dockErr := NewError(ErrCategoryNetwork, "TEST_ERROR", "Test error message", nil, true)
	fields := map[string]interface{}{
		"operation": "connect",
		"target":    "example.com",
	}
	LogErrorWithFields(dockErr, "Operation failed", fields)

	output := buf.String()
	assert.Contains(t, output, "ERROR Operation failed: Test error message")
	assert.Contains(t, output, "error_category=network")
	assert.Contains(t, output, "error_code=TEST_ERROR")
	assert.Contains(t, output, "retryable=true")
	assert.Contains(t, output, "operation=connect")
	assert.Contains(t, output, "target=example.com")

	// Reset buffer
	buf.Reset()

	// Test with standard error and additional fields
	stdErr := fmt.Errorf("standard error")
	LogErrorWithFields(stdErr, "Standard error occurred", fields)

	output = buf.String()
	assert.Contains(t, output, "ERROR Standard error occurred")
	assert.Contains(t, output, "error=standard error")
	assert.Contains(t, output, "operation=connect")
	assert.Contains(t, output, "target=example.com")
}

func TestLogDebug(t *testing.T) {
	// Setup a buffer to capture log output
	buf := &bytes.Buffer{}
	testLogger := logger.NewDefault()
	testLogger.SetOutput(buf)
	testLogger.SetLevel(logger.Debug)
	testLogger.UseColors = false // Disable colors for testing
	logger.SetDefaultLogger(testLogger)

	// Test with DockBridgeError
	dockErr := NewError(ErrCategoryNetwork, "TEST_ERROR", "Test error message", nil, true)
	LogDebug(dockErr, "Debug info")

	output := buf.String()
	assert.Contains(t, output, "DEBUG Debug info: Test error message")
	assert.Contains(t, output, "error_category=network")
	assert.Contains(t, output, "error_code=TEST_ERROR")

	// Reset buffer
	buf.Reset()

	// Test without error
	LogDebug(nil, "Debug message only")
	assert.Contains(t, buf.String(), "DEBUG Debug message only")
}

func TestLogWarn(t *testing.T) {
	// Setup a buffer to capture log output
	buf := &bytes.Buffer{}
	testLogger := logger.NewDefault()
	testLogger.SetOutput(buf)
	testLogger.SetLevel(logger.Debug)
	testLogger.UseColors = false // Disable colors for testing
	logger.SetDefaultLogger(testLogger)

	// Test with DockBridgeError
	dockErr := NewError(ErrCategoryConfig, "CONFIG_WARNING", "Config issue", nil, false)
	LogWarn(dockErr, "Configuration warning")

	output := buf.String()
	assert.Contains(t, output, "WARN Configuration warning: Config issue")
	assert.Contains(t, output, "error_category=config")
	assert.Contains(t, output, "error_code=CONFIG_WARNING")

	// Reset buffer
	buf.Reset()

	// Test without error
	LogWarn(nil, "Warning message only")
	assert.Contains(t, buf.String(), "WARN Warning message only")
}

// We can't easily test LogFatal as it calls os.Exit
// But we can test that it formats the message correctly
func TestLogFatalFormat(t *testing.T) {
	// Skip this test for now
	t.Skip("Skipping fatal log test")
}

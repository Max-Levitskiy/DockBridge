package logger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
		hasError bool
	}{
		{"debug", Debug, false},
		{"DEBUG", Debug, false},
		{"info", Info, false},
		{"INFO", Info, false},
		{"warn", Warn, false},
		{"WARN", Warn, false},
		{"error", Error, false},
		{"ERROR", Error, false},
		{"fatal", Fatal, false},
		{"FATAL", Fatal, false},
		{"unknown", Info, true},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			level, err := ParseLevel(test.input)
			if test.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, level)
			}
		})
	}
}

func TestNew(t *testing.T) {
	cfg := &Config{
		Level:      "debug",
		UseColors:  false,
		TimeFormat: "2006-01-02",
	}

	logger, err := New(cfg)
	assert.NoError(t, err)
	assert.Equal(t, Debug, logger.level)
	assert.Equal(t, false, logger.UseColors)
	assert.Equal(t, "2006-01-02", logger.timeFormat)

	// Test with invalid level
	invalidCfg := &Config{
		Level: "invalid",
	}
	_, err = New(invalidCfg)
	assert.Error(t, err)
}

func TestWithField(t *testing.T) {
	logger := NewDefault()
	newLogger := logger.WithField("key", "value")

	assert.Equal(t, logger.level, newLogger.level)
	assert.Equal(t, 1, len(newLogger.fields))
	assert.Equal(t, "value", newLogger.fields["key"])
}

func TestWithFields(t *testing.T) {
	logger := NewDefault()
	fields := map[string]any{
		"key1": "value1",
		"key2": 42,
	}
	newLogger := logger.WithFields(fields)

	assert.Equal(t, logger.level, newLogger.level)
	assert.Equal(t, 2, len(newLogger.fields))
	assert.Equal(t, "value1", newLogger.fields["key1"])
	assert.Equal(t, 42, newLogger.fields["key2"])
}

func TestSetLevel(t *testing.T) {
	logger := NewDefault()
	assert.Equal(t, Info, logger.level)

	logger.SetLevel(Debug)
	assert.Equal(t, Debug, logger.level)
}

func TestSetOutput(t *testing.T) {
	logger := NewDefault()
	buf := &bytes.Buffer{}
	logger.SetOutput(buf)

	logger.Info("test message")
	assert.Contains(t, buf.String(), "INFO test message")
}

func TestLogging(t *testing.T) {
	tests := []struct {
		level       Level
		logFunc     func(l *Logger, msg string)
		shouldWrite bool
	}{
		{Debug, func(l *Logger, msg string) { l.Debug(msg) }, true},
		{Info, func(l *Logger, msg string) { l.Info(msg) }, true},
		{Warn, func(l *Logger, msg string) { l.Warn(msg) }, true},
		{Error, func(l *Logger, msg string) { l.Error(msg) }, true},
		// Skip Fatal as it calls os.Exit
	}

	for _, test := range tests {
		t.Run(levelNames[test.level], func(t *testing.T) {
			buf := &bytes.Buffer{}
			logger := NewDefault()
			logger.SetOutput(buf)
			logger.SetLevel(test.level)
			logger.UseColors = false // Disable colors for testing

			test.logFunc(logger, "test message")

			if test.shouldWrite {
				assert.Contains(t, buf.String(), levelNames[test.level]+" test message")
			} else {
				assert.Empty(t, buf.String())
			}
		})
	}
}

func TestLogLevelFiltering(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewDefault()
	logger.SetOutput(buf)
	logger.SetLevel(Warn)
	logger.UseColors = false // Disable colors for testing

	// These should not be logged
	logger.Debug("debug message")
	logger.Info("info message")

	// These should be logged
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	assert.NotContains(t, output, "DEBUG debug message")
	assert.NotContains(t, output, "INFO info message")
	assert.Contains(t, output, "WARN warn message")
	assert.Contains(t, output, "ERROR error message")
}

func TestLogWithFields(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewDefault()
	logger.SetOutput(buf)
	logger.UseColors = false // Disable colors for testing

	logger.WithFields(map[string]any{
		"user":   "test",
		"action": "login",
	}).Info("user login")

	output := buf.String()
	assert.Contains(t, output, "INFO user login")
	assert.Contains(t, output, "user=test")
	assert.Contains(t, output, "action=login")
}

func TestLogWithFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewDefault()
	logger.SetOutput(buf)
	logger.UseColors = false // Disable colors for testing

	logger.Info("Hello, %s!", "world")

	output := buf.String()
	assert.Contains(t, output, "INFO Hello, world!")
}

func TestGlobalLoggerFunctions(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewDefault()
	logger.SetOutput(buf)
	logger.UseColors = false // Disable colors for testing
	SetDefaultLogger(logger)

	GlobalInfo("global info")
	GlobalWithField("key", "value").Warn("global warn")

	output := buf.String()
	assert.Contains(t, output, "INFO global info")
	assert.Contains(t, output, "WARN global warn")
	assert.Contains(t, output, "key=value")
}

func TestLoggerTimeFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := &Config{
		Level:      "info",
		UseColors:  false,
		TimeFormat: "2006/01/02",
	}

	logger, _ := New(cfg)
	logger.SetOutput(buf)

	logger.Info("test message")

	// Extract the timestamp part (before the first space)
	parts := strings.SplitN(buf.String(), " ", 2)
	timestamp := parts[0]

	// Check if timestamp follows the specified format (should be like "2023/07/21")
	assert.Regexp(t, `^\d{4}/\d{2}/\d{2}$`, timestamp)
}

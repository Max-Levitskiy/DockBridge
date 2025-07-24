package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

// LoggerInterface defines the interface for logging operations
type LoggerInterface interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Fatal(msg string, args ...interface{})
	WithFields(fields map[string]interface{}) *Logger
}

// Level represents the logging level
type Level int

const (
	// Debug level for detailed troubleshooting
	Debug Level = iota
	// Info level for general operational information
	Info
	// Warn level for potentially harmful situations
	Warn
	// Error level for error events that might still allow the application to continue
	Error
	// Fatal level for severe error events that will lead the application to abort
	Fatal
)

var levelNames = map[Level]string{
	Debug: "DEBUG",
	Info:  "INFO",
	Warn:  "WARN",
	Error: "ERROR",
	Fatal: "FATAL",
}

var levelColors = map[Level]*color.Color{
	Debug: color.New(color.FgCyan),
	Info:  color.New(color.FgGreen),
	Warn:  color.New(color.FgYellow),
	Error: color.New(color.FgRed),
	Fatal: color.New(color.FgHiRed, color.Bold),
}

// ParseLevel parses a level string into a Level
func ParseLevel(level string) (Level, error) {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return Debug, nil
	case "INFO":
		return Info, nil
	case "WARN":
		return Warn, nil
	case "ERROR":
		return Error, nil
	case "FATAL":
		return Fatal, nil
	default:
		return Info, fmt.Errorf("unknown log level: %s", level)
	}
}

// Logger represents a structured logger
type Logger struct {
	level      Level
	out        io.Writer
	fields     map[string]interface{}
	mu         sync.Mutex
	UseColors  bool
	timeFormat string
}

// Config represents logger configuration
type Config struct {
	Level      string `yaml:"level" default:"info"`
	UseColors  bool   `yaml:"use_colors" default:"true"`
	TimeFormat string `yaml:"time_format" default:"2006-01-02T15:04:05.000Z07:00"`
}

// New creates a new logger with the given configuration
func New(cfg *Config) (*Logger, error) {
	level, err := ParseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	timeFormat := cfg.TimeFormat
	if timeFormat == "" {
		timeFormat = "2006-01-02T15:04:05.000Z07:00"
	}

	return &Logger{
		level:      level,
		out:        os.Stdout,
		fields:     make(map[string]interface{}),
		UseColors:  cfg.UseColors,
		timeFormat: timeFormat,
	}, nil
}

// NewDefault creates a new logger with default configuration
func NewDefault() *Logger {
	return &Logger{
		level:      Info,
		out:        os.Stdout,
		fields:     make(map[string]interface{}),
		UseColors:  true,
		timeFormat: "2006-01-02T15:04:05.000Z07:00",
	}
}

// SetOutput sets the output writer
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out = w
}

// SetLevel sets the logging level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// WithField returns a new logger with the field added to the context
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return l.WithFields(map[string]interface{}{key: value})
}

// WithFields returns a new logger with the fields added to the context
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := &Logger{
		level:      l.level,
		out:        l.out,
		UseColors:  l.UseColors,
		timeFormat: l.timeFormat,
		fields:     make(map[string]interface{}, len(l.fields)+len(fields)),
	}

	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// Add new fields
	for k, v := range fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

// log logs a message at the specified level
func (l *Logger) log(level Level, msg string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Format the message if args are provided
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}

	timestamp := time.Now().Format(l.timeFormat)
	levelName := levelNames[level]

	// Build the log entry
	var builder strings.Builder

	// Add timestamp
	builder.WriteString(timestamp)
	builder.WriteString(" ")

	// Add level
	if l.UseColors {
		levelColor := levelColors[level]
		builder.WriteString(levelColor.Sprint(levelName))
	} else {
		builder.WriteString(levelName)
	}
	builder.WriteString(" ")

	// Add message
	builder.WriteString(msg)

	// Add fields if any
	if len(l.fields) > 0 {
		builder.WriteString(" ")
		first := true
		for k, v := range l.fields {
			if !first {
				builder.WriteString(", ")
			}
			builder.WriteString(k)
			builder.WriteString("=")
			builder.WriteString(fmt.Sprintf("%v", v))
			first = false
		}
	}

	builder.WriteString("\n")

	// Write to output
	fmt.Fprint(l.out, builder.String())

	// Exit on fatal
	if level == Fatal {
		os.Exit(1)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.log(Debug, msg, args...)
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...interface{}) {
	l.log(Info, msg, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...interface{}) {
	l.log(Warn, msg, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...interface{}) {
	l.log(Error, msg, args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, args ...interface{}) {
	l.log(Fatal, msg, args...)
}

// Global logger instance
var (
	defaultLogger = NewDefault()
	once          sync.Once
)

// SetDefaultLogger sets the global default logger
func SetDefaultLogger(logger *Logger) {
	defaultLogger = logger
}

// Global functions that use the default logger

// GlobalDebug logs a debug message to the default logger
func GlobalDebug(msg string, args ...interface{}) {
	defaultLogger.Debug(msg, args...)
}

// GlobalInfo logs an info message to the default logger
func GlobalInfo(msg string, args ...interface{}) {
	defaultLogger.Info(msg, args...)
}

// GlobalWarn logs a warning message to the default logger
func GlobalWarn(msg string, args ...interface{}) {
	defaultLogger.Warn(msg, args...)
}

// GlobalError logs an error message to the default logger
func GlobalError(msg string, args ...interface{}) {
	defaultLogger.Error(msg, args...)
}

// GlobalFatal logs a fatal message to the default logger and exits
func GlobalFatal(msg string, args ...interface{}) {
	defaultLogger.Fatal(msg, args...)
}

// GlobalWithField returns a new logger with the field added to the context
func GlobalWithField(key string, value interface{}) *Logger {
	return defaultLogger.WithField(key, value)
}

// GlobalWithFields returns a new logger with the fields added to the context
func GlobalWithFields(fields map[string]interface{}) *Logger {
	return defaultLogger.WithFields(fields)
}

// Interface implementation methods

// DebugWithFields logs a debug message with fields
func (l *Logger) DebugWithFields(msg string, fields map[string]interface{}) {
	if fields != nil {
		l.WithFields(fields).Debug(msg)
	} else {
		l.Debug(msg)
	}
}

// InfoWithFields logs an info message with fields
func (l *Logger) InfoWithFields(msg string, fields map[string]interface{}) {
	if fields != nil {
		l.WithFields(fields).Info(msg)
	} else {
		l.Info(msg)
	}
}

// WarnWithFields logs a warning message with fields
func (l *Logger) WarnWithFields(msg string, fields map[string]interface{}) {
	if fields != nil {
		l.WithFields(fields).Warn(msg)
	} else {
		l.Warn(msg)
	}
}

// ErrorWithFields logs an error message with fields
func (l *Logger) ErrorWithFields(msg string, err error, fields map[string]interface{}) {
	combinedFields := make(map[string]interface{})
	if fields != nil {
		for k, v := range fields {
			combinedFields[k] = v
		}
	}
	if err != nil {
		combinedFields["error"] = err.Error()
	}

	if len(combinedFields) > 0 {
		l.WithFields(combinedFields).Error(msg)
	} else {
		l.Error(msg)
	}
}

// FatalWithFields logs a fatal message with fields
func (l *Logger) FatalWithFields(msg string, err error, fields map[string]interface{}) {
	combinedFields := make(map[string]interface{})
	if fields != nil {
		for k, v := range fields {
			combinedFields[k] = v
		}
	}
	if err != nil {
		combinedFields["error"] = err.Error()
	}

	if len(combinedFields) > 0 {
		l.WithFields(combinedFields).Fatal(msg)
	} else {
		l.Fatal(msg)
	}
}

// Ensure Logger implements LoggerInterface
var _ LoggerInterface = (*Logger)(nil)

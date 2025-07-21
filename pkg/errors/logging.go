package errors

import (
	"github.com/dockbridge/dockbridge/pkg/logger"
)

// LogError logs an error with the appropriate level and context
func LogError(err error, msg string) {
	if err == nil {
		return
	}

	var dockErr *DockBridgeError
	if As(err, &dockErr) {
		fields := map[string]interface{}{
			"error_category": dockErr.Category,
			"error_code":     dockErr.Code,
			"retryable":      dockErr.Retryable,
			"timestamp":      dockErr.Timestamp,
		}

		if dockErr.Cause != nil {
			fields["cause"] = dockErr.Cause.Error()
		}

		logger.GlobalWithFields(fields).Error("%s: %s", msg, dockErr.Message)
	} else {
		logger.GlobalWithField("error", err.Error()).Error(msg)
	}
}

// LogErrorWithFields logs an error with additional context fields
func LogErrorWithFields(err error, msg string, fields map[string]interface{}) {
	if err == nil {
		return
	}

	logFields := make(map[string]interface{})
	for k, v := range fields {
		logFields[k] = v
	}

	var dockErr *DockBridgeError
	if As(err, &dockErr) {
		logFields["error_category"] = dockErr.Category
		logFields["error_code"] = dockErr.Code
		logFields["retryable"] = dockErr.Retryable
		logFields["timestamp"] = dockErr.Timestamp

		if dockErr.Cause != nil {
			logFields["cause"] = dockErr.Cause.Error()
		}

		logger.GlobalWithFields(logFields).Error("%s: %s", msg, dockErr.Message)
	} else {
		logFields["error"] = err.Error()
		logger.GlobalWithFields(logFields).Error(msg)
	}
}

// LogDebug logs a debug message with error context if an error is provided
func LogDebug(err error, msg string) {
	if err == nil {
		logger.GlobalDebug(msg)
		return
	}

	var dockErr *DockBridgeError
	if As(err, &dockErr) {
		fields := map[string]interface{}{
			"error_category": dockErr.Category,
			"error_code":     dockErr.Code,
			"retryable":      dockErr.Retryable,
		}

		if dockErr.Cause != nil {
			fields["cause"] = dockErr.Cause.Error()
		}

		logger.GlobalWithFields(fields).Debug("%s: %s", msg, dockErr.Message)
	} else {
		logger.GlobalWithField("error", err.Error()).Debug(msg)
	}
}

// LogWarn logs a warning message with error context if an error is provided
func LogWarn(err error, msg string) {
	if err == nil {
		logger.GlobalWarn(msg)
		return
	}

	var dockErr *DockBridgeError
	if As(err, &dockErr) {
		fields := map[string]interface{}{
			"error_category": dockErr.Category,
			"error_code":     dockErr.Code,
			"retryable":      dockErr.Retryable,
		}

		if dockErr.Cause != nil {
			fields["cause"] = dockErr.Cause.Error()
		}

		logger.GlobalWithFields(fields).Warn("%s: %s", msg, dockErr.Message)
	} else {
		logger.GlobalWithField("error", err.Error()).Warn(msg)
	}
}

// LogFatal logs a fatal message with error context and exits the application
func LogFatal(err error, msg string) {
	if err == nil {
		logger.GlobalFatal(msg)
		return
	}

	var dockErr *DockBridgeError
	if As(err, &dockErr) {
		fields := map[string]interface{}{
			"error_category": dockErr.Category,
			"error_code":     dockErr.Code,
			"retryable":      dockErr.Retryable,
			"timestamp":      dockErr.Timestamp,
		}

		if dockErr.Cause != nil {
			fields["cause"] = dockErr.Cause.Error()
		}

		logger.GlobalWithFields(fields).Fatal("%s: %s", msg, dockErr.Message)
	} else {
		logger.GlobalWithField("error", err.Error()).Fatal(msg)
	}
}

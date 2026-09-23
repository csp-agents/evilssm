// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package logger

import (
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/telemetry"
	telemetryLog "github.com/aws/amazon-ssm-agent/common/telemetry/telemetrylog"
)

// DelegateLogger holds the base logger for logging
type DelegateLogger struct {
	BaseLoggerInstance log.BasicT
}

// TelemetryLogger holds the telemetry namespace
type TelemetryLogger struct {
	Log telemetryLog.Log
}

// Wrapper is a logger that can modify the format of a log message before delegating to another logger.
type Wrapper struct {
	Format          FormatFilter
	TelemetryLogger *TelemetryLogger
	M               *sync.RWMutex
	Delegate        *DelegateLogger
	EventLogger     *EventLog
}

// FormatFilter can modify the format and or parameters to be passed to a logger.
type FormatFilter interface {
	// Filter modifies parameters that will be passed to log.Debug, log.Info, etc.
	Filter(params ...interface{}) (newParams []interface{})

	// Filter modifies format and/or parameter strings that will be passed to log.Debugf, log.Infof, etc.
	Filterf(format string, params ...interface{}) (newFormat string, newParams []interface{})
}

// WithContext creates a wrapper logger with context
func (w *Wrapper) WithContext(context ...string) (contextLogger log.T) {
	formatFilter := &ContextFormatFilter{Context: context}
	contextLogger = &Wrapper{TelemetryLogger: w.TelemetryLogger, Format: formatFilter, M: w.M, Delegate: w.Delegate, EventLogger: w.EventLogger}
	return contextLogger
}

// WithTelemetryNamespace creates a wrapper logger with the specified telemetry namespace
func (w *Wrapper) WithTelemetryNamespace(namespace string) (contextLogger log.T) {
	telemetryLogger := telemetry.GetLogger(namespace)
	telemetryNamespace := &TelemetryLogger{Log: telemetryLogger}
	contextLogger = &Wrapper{TelemetryLogger: telemetryNamespace, Format: w.Format, M: w.M, Delegate: w.Delegate, EventLogger: w.EventLogger}
	return contextLogger
}

// WriteEvent creates event in audit log.
// When blank value passed, will use the version number generated in version package
func (w *Wrapper) WriteEvent(eventType string, agentVersion string, event string) {
	if w.EventLogger == nil {
		return
	}
	w.EventLogger.loadEvent(eventType, agentVersion, event)
}

// Tracef formats message according to format specifier
// and writes to log with level = Trace.
func (w *Wrapper) Tracef(format string, params ...interface{}) {
	format, params = w.Format.Filterf(format, params...)
	w.M.RLock()
	defer w.M.RUnlock()
	w.Delegate.BaseLoggerInstance.Tracef(format, params...)
}

// Debugf formats message according to format specifier
// and writes to log with level = Debug.
func (w *Wrapper) Debugf(format string, params ...interface{}) {
	format, params = w.Format.Filterf(format, params...)

	w.M.RLock()
	defer w.M.RUnlock()
	w.Delegate.BaseLoggerInstance.Debugf(format, params...)
}

// Infof formats message according to format specifier
// and writes to log with level = Info.
func (w *Wrapper) Infof(format string, params ...interface{}) {
	format, params = w.Format.Filterf(format, params...)

	w.M.RLock()
	defer w.M.RUnlock()
	w.Delegate.BaseLoggerInstance.Infof(format, params...)
}

// Warnf formats message according to format specifier
// and writes to log with level = Warn.
func (w *Wrapper) Warnf(format string, params ...interface{}) error {
	format, params = w.Format.Filterf(format, params...)

	w.M.RLock()
	defer w.M.RUnlock()
	return w.Delegate.BaseLoggerInstance.Warnf(format, params...)
}

// TelemetryWarnf emits log telemetry and formats message according to format specifier
// and writes to log with level = Warn.
func (w *Wrapper) TelemetryWarnf(format string, params ...interface{}) error {
	format, params = w.Format.Filterf(format, params...)

	w.M.RLock()
	defer w.M.RUnlock()

	w.emitTelemetryLogf(telemetryLog.WARN, format, params...)

	return w.Delegate.BaseLoggerInstance.Warnf(format, params...)
}

// Errorf formats message according to format specifier
// and writes to log with level = Error.
func (w *Wrapper) Errorf(format string, params ...interface{}) error {
	format, params = w.Format.Filterf(format, params...)

	w.M.RLock()
	defer w.M.RUnlock()
	return w.Delegate.BaseLoggerInstance.Errorf(format, params...)
}

// TelemetryErrorf emits log telemetry and formats message according to format specifier
// and writes to log with level = Error.
func (w *Wrapper) TelemetryErrorf(format string, params ...interface{}) error {
	format, params = w.Format.Filterf(format, params...)

	w.M.RLock()
	defer w.M.RUnlock()

	w.emitTelemetryLogf(telemetryLog.ERROR, format, params...)

	return w.Delegate.BaseLoggerInstance.Errorf(format, params...)
}

// Criticalf formats message according to format specifier
// and writes to log with level = Critical.
func (w *Wrapper) Criticalf(format string, params ...interface{}) error {
	format, params = w.Format.Filterf(format, params...)

	w.M.RLock()
	defer w.M.RUnlock()
	return w.Delegate.BaseLoggerInstance.Criticalf(format, params...)
}

// TelemetryCriticalf emits log telemetry and formats message according to format specifier
// and writes to log with level Critical.
func (w *Wrapper) TelemetryCriticalf(format string, params ...interface{}) error {
	format, params = w.Format.Filterf(format, params...)

	w.M.RLock()
	defer w.M.RUnlock()

	w.emitTelemetryLogf(telemetryLog.CRITICAL, format, params...)

	return w.Delegate.BaseLoggerInstance.Criticalf(format, params...)
}

// Trace formats message using the default formats for its operands
// and writes to log with level = Trace
func (w *Wrapper) Trace(v ...interface{}) {
	v = w.Format.Filter(v...)
	w.M.RLock()
	defer w.M.RUnlock()
	w.Delegate.BaseLoggerInstance.Trace(v...)
}

// Debug formats message using the default formats for its operands
// and writes to log with level = Debug
func (w *Wrapper) Debug(v ...interface{}) {
	v = w.Format.Filter(v...)

	w.M.RLock()
	defer w.M.RUnlock()
	w.Delegate.BaseLoggerInstance.Debug(v...)
}

// Info formats message using the default formats for its operands
// and writes to log with level = Info
func (w *Wrapper) Info(v ...interface{}) {
	v = w.Format.Filter(v...)

	w.M.RLock()
	defer w.M.RUnlock()
	w.Delegate.BaseLoggerInstance.Info(v...)
}

// Warn formats message using the default formats for its operands
// and writes to log with level = Warn
func (w *Wrapper) Warn(v ...interface{}) error {
	v = w.Format.Filter(v...)

	w.M.RLock()
	defer w.M.RUnlock()
	return w.Delegate.BaseLoggerInstance.Warn(v...)
}

// TelemetryWarn emits log telemetry and formats message using the default formats for its operands
// and writes to log with level Warn.
func (w *Wrapper) TelemetryWarn(v ...interface{}) error {
	v = w.Format.Filter(v...)

	w.M.RLock()
	defer w.M.RUnlock()

	w.emitTelemetryLog(telemetryLog.WARN, v...)

	return w.Delegate.BaseLoggerInstance.Warn(v...)
}

// Error formats message using the default formats for its operands
// and writes to log with level = Error
func (w *Wrapper) Error(v ...interface{}) error {
	v = w.Format.Filter(v...)

	w.M.RLock()
	defer w.M.RUnlock()
	return w.Delegate.BaseLoggerInstance.Error(v...)
}

// TelemetryError emits log telemetry and formats message using the default formats for its operands
// and writes to log with level Error.
func (w *Wrapper) TelemetryError(v ...interface{}) error {
	v = w.Format.Filter(v...)

	w.M.RLock()
	defer w.M.RUnlock()

	w.emitTelemetryLog(telemetryLog.ERROR, v...)

	return w.Delegate.BaseLoggerInstance.Error(v...)
}

// Critical formats message using the default formats for its operands
// and writes to log with level = Critical
func (w *Wrapper) Critical(v ...interface{}) error {
	v = w.Format.Filter(v...)

	w.M.RLock()
	defer w.M.RUnlock()
	return w.Delegate.BaseLoggerInstance.Critical(v...)
}

// TelemetryCritical emits log telemetry and formats message using the default formats for its operands
// and writes to log with level Critical.
func (w *Wrapper) TelemetryCritical(v ...interface{}) error {
	v = w.Format.Filter(v...)

	w.M.RLock()
	defer w.M.RUnlock()

	w.emitTelemetryLog(telemetryLog.CRITICAL, v...)

	return w.Delegate.BaseLoggerInstance.Critical(v...)
}

// emitTelemetryLogf emits log telemetry and formats message according to format specifier
func (w *Wrapper) emitTelemetryLogf(severity telemetryLog.Severity, format string, params ...interface{}) {
	if w.TelemetryLogger != nil {
		err := w.TelemetryLogger.Log.EmitLogf(severity, format, params...)
		if err != nil {
			format1, params1 := w.Format.Filterf("Error emitting log telemetry: %v", err)
			w.Delegate.BaseLoggerInstance.Warnf(format1, params1...)
		}
	}
}

// emitTelemetryLogf emits log telemetry and formats message using the default formats for its operands
func (w *Wrapper) emitTelemetryLog(severity telemetryLog.Severity, v ...interface{}) {
	if w.TelemetryLogger != nil {
		err := w.TelemetryLogger.Log.EmitLog(severity, v...)
		if err != nil {
			format, params := w.Format.Filterf("Error emitting log telemetry: %v", err)
			w.Delegate.BaseLoggerInstance.Warnf(format, params...)
		}
	}
}

// Flush flushes all the messages in the logger.
func (w *Wrapper) Flush() {
	w.M.Lock()
	defer w.M.Unlock()
	w.Delegate.BaseLoggerInstance.Flush()
}

// Close flushes all the messages in the logger and closes it. It cannot be used after this operation.
func (w *Wrapper) Close() {
	w.M.Lock()
	defer w.M.Unlock()
	w.Delegate.BaseLoggerInstance.Close()
	if w.EventLogger == nil {
		return
	}
	// Will revisit later
	// w.EventLogger.Close()
}

// Closed checks if logger is closed
func (w *Wrapper) Closed() bool {
	w.M.Lock()
	defer w.M.Unlock()
	return w.Delegate.BaseLoggerInstance.Closed()
}

func (w *Wrapper) Log(i ...interface{}) {
	w.Info(i)
}

// ReplaceDelegate replaces the delegate logger with a new logger
func (w *Wrapper) ReplaceDelegate(newLogger log.BasicT) {
	w.M.Lock()
	defer w.M.Unlock()
	w.Delegate.BaseLoggerInstance.Flush()
	w.Delegate.BaseLoggerInstance.Close()
	w.Delegate.BaseLoggerInstance = newLogger
	w.Delegate.BaseLoggerInstance.Info("Logger Replaced. New Logger Used to log the message")
}

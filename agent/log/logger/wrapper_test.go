package logger

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	telemetryLog "github.com/aws/amazon-ssm-agent/common/telemetry/telemetrylog"
	telemetryLogMocks "github.com/aws/amazon-ssm-agent/common/telemetry/telemetrylog/mocks"
	seelog "github.com/cihub/seelog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type LogLevel uint8

// Log levels
const (
	TraceLvl = iota
	DebugLvl
	InfoLvl
	WarnLvl
	ErrorLvl
	TelemetryErrorLvl
	CriticalLvl
	TelemetryCriticalLvl
	Off
)

type TestCase struct {
	TelemetryLog *telemetryLogMocks.Mock
	Context      string
	LogFormat    string
	Level        LogLevel
	Message      string
	Params       []interface{}
	Output       string
}

func generateTelemetryTestCase(t *testing.T, level LogLevel, telemetryEmitError error, message string, params ...interface{}) TestCase {
	testCase := TestCase{
		Context:   "<some context>",
		LogFormat: "[%Level] %Msg%n",
		Level:     level,
		Message:   message,
		Params:    params,
	}
	levelStr := getLevelStr(t, level)

	msg := fmt.Sprintf(testCase.Message, testCase.Params...)

	testCase.Output = ""
	if telemetryEmitError != nil {
		switch level {
		case TelemetryErrorLvl, TelemetryCriticalLvl:
			testCase.Output = fmt.Sprintf("[%v] %v %v\n", "Warn", testCase.Context, "Error emitting log telemetry: telemetry is not initialized")
		}
	}

	testCase.Output += fmt.Sprintf("[%v] %v %v\n", levelStr, testCase.Context, msg)

	mockLog := telemetryLogMocks.NewMockDefault()
	mockLog.On("EmitLog", mock.Anything, mock.Anything).Return(telemetryEmitError)
	mockLog.On("EmitLogf", mock.Anything, mock.Anything, mock.Anything).Return(telemetryEmitError)

	testCase.TelemetryLog = mockLog
	return testCase
}

func getLevelStr(t *testing.T, level LogLevel) string {
	var levelStr string

	switch level {
	case CriticalLvl:
		levelStr = "Critical"
	case TelemetryCriticalLvl:
		levelStr = "Critical"
	case ErrorLvl:
		levelStr = "Error"
	case TelemetryErrorLvl:
		levelStr = "Error"
	case WarnLvl:
		levelStr = "Warn"
	case InfoLvl:
		levelStr = "Info"
	case DebugLvl:
		levelStr = "Debug"
	case TraceLvl:
		levelStr = "Trace"

	default:
		assert.Fail(t, "Unexpected log level", level)
	}
	return levelStr
}

// TestWithContextRetainsTelemetryNamespace ensures that telemetry namespace is retained after WithContext creates a new logger
func TestWithContextRetainsTelemetryNamespace(t *testing.T) {
	var out bytes.Buffer
	seelogger, err := seelog.LoggerFromWriterWithMinLevelAndFormat(&out, seelog.TraceLvl, "[%Level] %Msg%n")
	assert.Nil(t, err)

	loggerInstance := &DelegateLogger{}
	loggerInstance.BaseLoggerInstance = seelogger

	logger := &Wrapper{M: PkgMutex, Delegate: loggerInstance}
	assert.Nil(t, logger.TelemetryLogger)
	logger = logger.WithTelemetryNamespace("testNamespace").(*Wrapper)
	assert.NotNil(t, logger.TelemetryLogger)
	before := logger.TelemetryLogger

	logger = logger.WithContext("<some context>").(*Wrapper)
	assert.Equal(t, before, logger.TelemetryLogger)
}

// TestLoggerWithoutTelemetry tests that there are no failures when telemetry namespace is not set
func TestLoggerWithoutTelemetry(t *testing.T) {
	var out bytes.Buffer
	seelogger, err := seelog.LoggerFromWriterWithMinLevelAndFormat(&out, seelog.TraceLvl, "[%Level] %Msg%n")
	assert.Nil(t, err)

	loggerInstance := &DelegateLogger{}
	loggerInstance.BaseLoggerInstance = seelogger

	logger := &Wrapper{M: PkgMutex, Delegate: loggerInstance}
	logger = logger.WithContext("<some context>").(*Wrapper)
	logger.TelemetryLogger = nil

	logger.Error("(some message without parameters)")
	logger.Flush()

	assert.Equal(t, "[Error] <some context> (some message without parameters)\n", out.String())
}

// TestLoggerWithoutTelemetry tests that ERROR and CRITICAL logs are emitted to telemetry
func TestLoggerWithTelemetryNamespace(t *testing.T) {
	var testCases []TestCase

	for _, logLevel := range []LogLevel{TraceLvl, DebugLvl, InfoLvl, ErrorLvl, TelemetryErrorLvl, CriticalLvl, TelemetryCriticalLvl} {
		testCases = append(testCases, generateTelemetryTestCase(t, logLevel, nil, "(some message without parameters)"))
		testCases = append(testCases, generateTelemetryTestCase(t, logLevel, nil, "(some message with %v as param)", []interface{}{"|a param|"}))
	}

	for _, testCase := range testCases {
		testLoggerWithTelemetry(t, testCase)
	}
}

// TestLoggerWithTelemetryNamespaceNotInitialized tests that a warning is logged if telemetry is not initialized
func TestLoggerWithTelemetryNamespaceNotInitialized(t *testing.T) {
	var testCases []TestCase

	telemetryEmitError := errors.New("telemetry is not initialized")

	for _, logLevel := range []LogLevel{TraceLvl, DebugLvl, InfoLvl, ErrorLvl, TelemetryErrorLvl, CriticalLvl, TelemetryCriticalLvl} {
		testCases = append(testCases, generateTelemetryTestCase(t, logLevel, telemetryEmitError, "(some message without parameters)"))
		testCases = append(testCases, generateTelemetryTestCase(t, logLevel, telemetryEmitError, "(some message with %v as param)", []interface{}{"|a param|"}))
	}

	for _, testCase := range testCases {
		testLoggerWithTelemetry(t, testCase)
	}
}

func testLoggerWithTelemetry(t *testing.T, testCase TestCase) {
	// create seelog logger that outputs to buffer
	var out bytes.Buffer
	seelogger, err := seelog.LoggerFromWriterWithMinLevelAndFormat(&out, seelog.TraceLvl, testCase.LogFormat)
	assert.Nil(t, err)

	loggerInstance := &DelegateLogger{}
	loggerInstance.BaseLoggerInstance = seelogger

	logger := &Wrapper{M: PkgMutex, Delegate: loggerInstance}
	logger = logger.WithContext(testCase.Context).(*Wrapper)
	logger.TelemetryLogger = &TelemetryLogger{Log: testCase.TelemetryLog}

	// exercise logger
	switch testCase.Level {
	case CriticalLvl:
		if len(testCase.Params) > 0 {
			logger.Criticalf(testCase.Message, testCase.Params...)
		} else {
			logger.Critical(testCase.Message)
		}
	case TelemetryCriticalLvl:
		if len(testCase.Params) > 0 {
			logger.TelemetryCriticalf(testCase.Message, testCase.Params...)
		} else {
			logger.TelemetryCritical(testCase.Message)
		}
	case ErrorLvl:
		if len(testCase.Params) > 0 {
			logger.Errorf(testCase.Message, testCase.Params...)
		} else {
			logger.Error(testCase.Message)
		}
	case TelemetryErrorLvl:
		if len(testCase.Params) > 0 {
			logger.TelemetryErrorf(testCase.Message, testCase.Params...)
		} else {
			logger.TelemetryError(testCase.Message)
		}
	case InfoLvl:
		if len(testCase.Params) > 0 {
			logger.Infof(testCase.Message, testCase.Params...)
		} else {
			logger.Info(testCase.Message)
		}
	case DebugLvl:
		if len(testCase.Params) > 0 {
			logger.Debugf(testCase.Message, testCase.Params...)
		} else {
			logger.Debug(testCase.Message)
		}
	case TraceLvl:
		if len(testCase.Params) > 0 {
			logger.Tracef(testCase.Message, testCase.Params...)
		} else {
			logger.Trace(testCase.Message)
		}

	default:
		assert.Fail(t, "Unexpected log level", testCase.Level)
	}
	logger.Flush()

	switch testCase.Level {
	case TelemetryErrorLvl, TelemetryCriticalLvl:

		var telemetrySeverity telemetryLog.Severity

		switch testCase.Level {
		case TelemetryCriticalLvl:
			telemetrySeverity = telemetryLog.CRITICAL
		case TelemetryErrorLvl:
			telemetrySeverity = telemetryLog.ERROR
		default:
			assert.Fail(t, "Unexpected log level", testCase.Level)
		}

		if len(testCase.Params) > 0 {
			testCase.TelemetryLog.AssertCalled(t, "EmitLogf", telemetrySeverity, fmt.Sprintf("%v %v", testCase.Context, testCase.Message), testCase.Params)
		} else {
			testCase.TelemetryLog.AssertCalled(t, "EmitLog", telemetrySeverity, []interface{}([]interface{}{testCase.Context + " ", testCase.Message}))
		}
	default:
		// no other level should emit telemetry
		testCase.TelemetryLog.AssertNumberOfCalls(t, "EmitLog", 0)
		testCase.TelemetryLog.AssertNumberOfCalls(t, "EmitLogf", 0)
	}

	// check result
	assert.Equal(t, testCase.Output, out.String())
}

// Copyright 2025 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
package telemetry

import (
	"encoding/json"
	"math/rand"
	"sort"
	"strings"
	"testing"
	"time"

	telemetryContextMocks "github.com/aws/amazon-ssm-agent/common/telemetry/context/mocks"
	"github.com/aws/amazon-ssm-agent/common/telemetry/emitter"
	emitterMock "github.com/aws/amazon-ssm-agent/common/telemetry/emitter/mocks"
	"github.com/aws/amazon-ssm-agent/common/telemetry/metric"
	"github.com/aws/amazon-ssm-agent/common/telemetry/telemetrylog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// TelemetryTestSuite define agent test suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TelemetryTestSuite struct {
	suite.Suite
	originalPreingestionDir string
	mockContext             *telemetryContextMocks.Mock
}

// TestChannelSuite executes test suite
func TestTelemetrySuite(t *testing.T) {
	suite.Run(t, new(TelemetryTestSuite))
}

// SetupTest makes sure that all the components referenced in the test case are initialized
// before each test
func (suite *TelemetryTestSuite) SetupTest() {
	suite.mockContext = telemetryContextMocks.NewMockDefault()

	// Temporarily override the TelemetryPreIngestionDir
	suite.originalPreingestionDir = emitter.TelemetryPreIngestionDir
	emitter.TelemetryPreIngestionDir = suite.T().TempDir()
}

func (suite *TelemetryTestSuite) TearDownTest() {
	Shutdown()
}

func (suite *TelemetryTestSuite) TestInitialize() {
	telemetryInstance, _ := getTelemetry()
	assert.Nil(suite.T(), telemetryInstance)

	Initialize(suite.mockContext)

	telemetryInstance, _ = getTelemetry()
	assert.NotNil(suite.T(), telemetryInstance)
	assert.NotNil(suite.T(), telemetryInstance.emitter)
}

// TestTelemetryAlreadyInitialized verifies that the telemetry initialization
// can only occur once
func (suite *TelemetryTestSuite) TestTelemetryAlreadyInitialized() {
	telemetryInstance, _ := getTelemetry()
	assert.Nil(suite.T(), telemetryInstance)

	Initialize(suite.mockContext)

	telemetryInstance, _ = getTelemetry()

	assert.NotNil(suite.T(), telemetryInstance)
	err := Initialize(suite.mockContext)

	assert.Equal(suite.T(), "telemetry is already initialized", err.Error())
}

func (suite *TelemetryTestSuite) TestShutdown() {
	Initialize(suite.mockContext)

	telemetryInstance, _ := getTelemetry()
	assert.NotNil(suite.T(), telemetryInstance.emitter)

	me := emitterMock.NewMockEmitter()
	telemetryInstance.emitter = me

	Shutdown()
	assert.True(suite.T(), me.CloseCalled)

	telemetryInstance, _ = getTelemetry()
	assert.Nil(suite.T(), telemetryInstance)
}

func (suite *TelemetryTestSuite) Test_emitLog() {
	Initialize(suite.mockContext)
	telemetryInstance, _ := getTelemetry()
	me := emitterMock.NewMockEmitter()
	telemetryInstance.emitter = me

	now := time.Now()

	err := telemetryInstance.emitLog("testNamespace", now, telemetrylog.ERROR, "This is a test message")
	assert.Nil(suite.T(), err)

	expectedLogEntry := &telemetrylog.Entry{
		Time:     now,
		Severity: telemetrylog.ERROR,
		Body:     "This is a test message",
	}

	entryJson, err := json.Marshal(expectedLogEntry)
	assert.Nil(suite.T(), err)

	expectedMessage := emitter.Message{
		Type:    emitter.LOG,
		Payload: string(entryJson),
	}

	assert.EventuallyWithT(suite.T(), func(c *assert.CollectT) {
		assert.Len(c, me.GetAllMessages(), 1)
		if len(me.GetMessages("testNamespace")) == 0 {
			assert.Fail(c, "no messages yet")
			return
		}

		var actualMessage emitter.Message = me.GetMessages("testNamespace")[0]

		// Check that correct message was emitted
		assert.Equal(c, expectedMessage, actualMessage)
	}, 2*time.Second, 5*time.Millisecond)
}

func (suite *TelemetryTestSuite) Test_emitLogTruncates() {
	Initialize(suite.mockContext)

	telemetryInstance, _ := getTelemetry()
	me := emitterMock.NewMockEmitter()
	telemetryInstance.emitter = me

	now := time.Now()

	err := telemetryInstance.emitLog("testNamespace", now, telemetrylog.ERROR, strings.Repeat("🙂", 550))
	assert.Nil(suite.T(), err)

	expectedLogEntry := &telemetrylog.Entry{
		Time:     now,
		Severity: telemetrylog.ERROR,
		Body:     strings.Repeat("🙂", 400),
	}

	entryJson, err := json.Marshal(expectedLogEntry)
	assert.Nil(suite.T(), err)

	expectedMessage := emitter.Message{
		Type:    emitter.LOG,
		Payload: string(entryJson),
	}

	assert.EventuallyWithT(suite.T(), func(c *assert.CollectT) {
		assert.Len(c, me.GetAllMessages(), 1)
		if len(me.GetMessages("testNamespace")) == 0 {
			assert.Fail(c, "no messages yet")
			return
		}

		var actualMessage emitter.Message = me.Messages["testNamespace"][0]

		// Check that correct message was received on the other side
		assert.Equal(c, expectedMessage, actualMessage)
	}, 2*time.Second, 5*time.Millisecond)
}

func (suite *TelemetryTestSuite) TestEmitLog() {
	Initialize(suite.mockContext)
	telemetryInstance, _ := getTelemetry()
	me := emitterMock.NewMockEmitter()
	telemetryInstance.emitter = me

	logger := GetLogger("testNamespace")
	logger.EmitLog(telemetrylog.ERROR, "This is a test message")

	assert.EventuallyWithT(suite.T(), func(c *assert.CollectT) {
		assert.Len(c, me.GetAllMessages(), 1)
		if len(me.GetMessages("testNamespace")) == 0 {
			assert.Fail(c, "no messages yet")
			return
		}

		var actualMessage emitter.Message = me.Messages["testNamespace"][0]
		assert.Equal(c, emitter.LOG, actualMessage.Type)

		var actualLogEntry *telemetrylog.Entry
		err := json.Unmarshal([]byte(actualMessage.Payload), &actualLogEntry)
		assert.Nil(c, err)

		// Check that correct message was emitted
		assert.Equal(c, "This is a test message", actualLogEntry.Body)
		assert.Equal(c, telemetrylog.ERROR, actualLogEntry.Severity)
	}, 2*time.Second, 5*time.Millisecond)
}

func (suite *TelemetryTestSuite) TestEmitLogf() {
	Initialize(suite.mockContext)
	telemetryInstance, _ := getTelemetry()
	me := emitterMock.NewMockEmitter()
	telemetryInstance.emitter = me

	logger := GetLogger("testNamespace")
	logger.EmitLogf(telemetrylog.ERROR, "This is a test message %v, %v", 1, "hi")

	assert.EventuallyWithT(suite.T(), func(c *assert.CollectT) {
		assert.Len(c, me.GetAllMessages(), 1)
		if len(me.GetMessages("testNamespace")) == 0 {
			assert.Fail(c, "no messages yet")
			return
		}

		var actualMessage emitter.Message = me.Messages["testNamespace"][0]
		assert.Equal(c, emitter.LOG, actualMessage.Type)

		var actualLogEntry *telemetrylog.Entry
		err := json.Unmarshal([]byte(actualMessage.Payload), &actualLogEntry)
		assert.Nil(c, err)

		// Check that correct message was emitted
		assert.Equal(c, "This is a test message 1, hi", actualLogEntry.Body)
		assert.Equal(c, telemetrylog.ERROR, actualLogEntry.Severity)
	}, 2*time.Second, 5*time.Millisecond)
}

func (suite *TelemetryTestSuite) Test_emitIntegerMetric() {
	Initialize(suite.mockContext)
	telemetryInstance, _ := getTelemetry()
	me := emitterMock.NewMockEmitter()
	telemetryInstance.emitter = me

	now := time.Now()

	err := telemetryInstance.emitIntegerMetric("testNamespace", "testMetric", "event", metric.Sum, now, 100)
	assert.Nil(suite.T(), err)

	expectedMetric := &metric.Metric[int64]{
		Name:       "testMetric",
		Unit:       "event",
		Kind:       metric.Sum,
		DataPoints: []metric.DataPoint[int64]{{StartTime: now, EndTime: now, Value: 100}},
	}

	metricJson, err := json.Marshal(expectedMetric)
	assert.Nil(suite.T(), err)

	expectedMessage := emitter.Message{
		Type:    emitter.METRIC,
		Payload: string(metricJson),
	}

	assert.EventuallyWithT(suite.T(), func(c *assert.CollectT) {
		assert.Len(c, me.GetAllMessages(), 1)
		if len(me.GetMessages("testNamespace")) == 0 {
			assert.Fail(c, "no messages yet")
			return
		}

		var actualMessage emitter.Message = me.Messages["testNamespace"][0]

		// Check that correct message was emitted
		assert.Equal(c, expectedMessage, actualMessage)
	}, 2*time.Second, 5*time.Millisecond)
}

func (suite *TelemetryTestSuite) TestInt64Counter() {
	Initialize(suite.mockContext)
	telemetryInstance, _ := getTelemetry()
	me := emitterMock.NewMockEmitter()
	telemetryInstance.emitter = me

	meter := GetMeter("testNamespace")
	counter := meter.Int64Counter("testCounter", "event")

	metrics := make([]metric.Metric[int64], 0)

	for range [10]int{} {
		val := rand.Int63()

		counter.Add(val)

		now := time.Now()
		// timestamps cannot be compared since we use time.Now() in actual code which cannot be mocked
		expectedMetric := metric.Metric[int64]{
			Name:       "testMetric",
			Unit:       "event",
			DataPoints: []metric.DataPoint[int64]{{StartTime: now, EndTime: now, Value: val}},
		}

		metrics = append(metrics, expectedMetric)

		// add some delay to ensure that the timestamps are distinct enough to be sorted
		time.Sleep(2 * time.Millisecond)
	}
	assert.EventuallyWithT(suite.T(), func(c *assert.CollectT) {
		assert.Len(c, me.GetAllMessages(), 1)
		assert.Len(c, me.GetMessages("testNamespace"), len(metrics))

		receivedMetrics := make([]metric.Metric[int64], 0)
		for i := range metrics {
			msg := me.GetMessages("testNamespace")[i]
			suite.T().Logf("TestInt64Counter: received message: %v", msg)

			assert.Equal(c, emitter.METRIC, msg.Type)

			var actualMetric *metric.Metric[int64]
			err := json.Unmarshal([]byte(msg.Payload), &actualMetric)
			assert.Nil(c, err)

			receivedMetrics = append(receivedMetrics, *actualMetric)
		}

		assert.Len(c, receivedMetrics, len(metrics))

		// the metrics may be received in any order, so we sort them before comparing
		// to make the test deterministic
		sort.Slice(receivedMetrics, func(i, j int) bool {
			return receivedMetrics[i].DataPoints[0].StartTime.Before(receivedMetrics[j].DataPoints[0].StartTime)
		})

		for i, expectedMetric := range metrics {
			assert.Equal(c, "testCounter", receivedMetrics[i].Name)
			assert.Len(c, receivedMetrics[i].DataPoints, 1)
			dataPoint := receivedMetrics[i].DataPoints[0]
			assert.Equal(c, expectedMetric.DataPoints[0].Value, dataPoint.Value)
		}
	}, 2*time.Second, 5*time.Millisecond)

}

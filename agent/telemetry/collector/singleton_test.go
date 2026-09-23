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
package collector

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/mocks/context"

	collectorMocks "github.com/aws/amazon-ssm-agent/agent/telemetry/collector/mocks"
	dynamicconfiguration "github.com/aws/amazon-ssm-agent/agent/telemetry/dynamic_configuration"
	exporterMocks "github.com/aws/amazon-ssm-agent/agent/telemetry/exporter/mocks"

	"github.com/aws/amazon-ssm-agent/common/telemetry/emitter"
	"github.com/aws/amazon-ssm-agent/common/telemetry/metric"
	"github.com/aws/amazon-ssm-agent/common/telemetry/telemetrylog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type singletonTestSuite struct {
	suite.Suite
	originalPreingestionDir string
	ctx                     *context.Mock
}

// TestSingletonSuite executes test suite
func TestSingletonSuite(t *testing.T) {
	suite.Run(t, new(singletonTestSuite))
}

// SetupTest makes sure that all the components referenced in the
// test case are initialized before each test
func (suite *singletonTestSuite) SetupTest() {
	suite.ctx = context.NewMockDefault()
	// Temporarily override the TelemetryPreIngestionDir
	suite.originalPreingestionDir = emitter.TelemetryPreIngestionDir
	emitter.TelemetryPreIngestionDir = suite.T().TempDir()
}

func (suite *singletonTestSuite) TearDownTest() {
	StopCollection(suite.ctx.Log())
	singleton = nil
	defer func() { emitter.TelemetryPreIngestionDir = suite.originalPreingestionDir }()
}

func (suite *singletonTestSuite) TestStartCollection() {
	dynamicconfiguration.MaxRolls = func(string) int { return 10 }
	dynamicconfiguration.MaxRollSize = func(string) int64 { return 1024 * 1024 }

	defer func() {
		dynamicconfiguration.MaxRolls = dynamicconfiguration.GetMaxRolls
		dynamicconfiguration.MaxRollSize = dynamicconfiguration.GetMaxRollSize
	}()

	// create sender side of the telemetry
	e := emitter.NewEmitter(suite.ctx.Log())
	defer e.Close()

	// start telemetry collection
	err := StartCollection(suite.ctx)
	assert.NoError(suite.T(), err)

	// replace the collector in the singleton with mock
	collectorMock := collectorMocks.NewCollectorMock()
	// set expectations
	collectorMock.On("CollectLog", mock.Anything, mock.Anything).Return(nil)
	collectorMock.On("CollectMetric", mock.Anything, mock.Anything).Return(nil)
	collectorMock.On("Close").Return(nil).Once()
	singleton.collector = collectorMock

	logCounts := 10
	metricCounts := 8

	expectedLogEntries := make([]telemetrylog.Entry, 0, logCounts)
	expectedMetrics := make([]metric.Metric[float64], 0, metricCounts)

	now := time.Now()

	// emit logs
	for i := range logCounts {
		expectedLogEntry := &telemetrylog.Entry{
			Time:     now.UTC(),
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message : %v", i),
		}
		entryJson, err := json.Marshal(expectedLogEntry)
		assert.NoError(suite.T(), err)

		expectedLogEntries = append(expectedLogEntries, *expectedLogEntry)

		// send telemetry
		err = e.Emit("testNamespace", emitter.Message{
			Type:    emitter.LOG,
			Payload: string(entryJson),
		})
		assert.NoError(suite.T(), err)
	}

	// emit metrics
	for i := range metricCounts {
		expectedMetric := &metric.Metric[float64]{
			Name: fmt.Sprintf("testMetric%v", i),
			Unit: "1",
			Kind: metric.Sum,
			DataPoints: []metric.DataPoint[float64]{
				{
					StartTime: now.UTC(),
					EndTime:   now.UTC(),
					Value:     float64(i),
				},
			},
		}
		entryJson, err := json.Marshal(expectedMetric)
		assert.NoError(suite.T(), err)

		expectedMetrics = append(expectedMetrics, *expectedMetric)

		// send telemetry
		err = e.Emit("testNamespace", emitter.Message{
			Type:    emitter.METRIC,
			Payload: string(entryJson),
		})
		assert.NoError(suite.T(), err)
	}

	err = e.Flush()
	assert.NoError(suite.T(), err)

	// skip the wait
	singleton.consumer.consumerJob.SkipWait <- true

	// assert that they were collected
	assert.EventuallyWithT(suite.T(), func(c *assert.CollectT) {
		ct := NewCommonT(c)

		collectorMock.AssertNumberOfCalls(ct, "CollectLog", logCounts)
		collectorMock.AssertNumberOfCalls(ct, "CollectMetric", metricCounts)

		for _, e := range expectedLogEntries {
			collectorMock.AssertCalled(ct, "CollectLog", "testNamespace", e)
		}

		for _, e := range expectedMetrics {
			collectorMock.AssertCalled(ct, "CollectMetric", "testNamespace", e)
		}
	}, 30*time.Second, 100*time.Millisecond)
}

func (suite *singletonTestSuite) TestLogsAreTruncated() {
	// start telemetry collection
	err := StartCollection(suite.ctx)
	assert.NoError(suite.T(), err)

	// replace the collector in the singleton with mock
	collectorMock := collectorMocks.NewCollectorMock()
	// set expectations
	collectorMock.On("CollectLog", mock.Anything, mock.Anything).Return(nil)
	collectorMock.On("CollectMetric", mock.Anything, mock.Anything).Return(nil)
	collectorMock.On("Close").Return(nil).Once()
	singleton.collector = collectorMock

	// create sender side of the telemetry
	e := emitter.NewEmitter(suite.ctx.Log())
	defer e.Close()

	// emit telemetry
	now := time.Now()
	sentLogEntry := &telemetrylog.Entry{
		Time:     now.UTC(),
		Severity: telemetrylog.ERROR,
		Body:     strings.Repeat("A🙂", 400),
	}
	entryJson, err := json.Marshal(sentLogEntry)
	assert.NoError(suite.T(), err)

	// send log
	err = e.Emit("testNamespace", emitter.Message{
		Type:    emitter.LOG,
		Payload: string(entryJson),
	})
	assert.NoError(suite.T(), err)

	err = e.Flush()
	assert.NoError(suite.T(), err)

	expectedLogEntry := *sentLogEntry                 // make a copy
	expectedLogEntry.Body = strings.Repeat("A🙂", 200) // 2 * 200 characters = 400 expected characters

	// skip the wait
	singleton.consumer.consumerJob.SkipWait <- true
	// assert that they were collected
	assert.EventuallyWithT(suite.T(), func(c *assert.CollectT) {
		ct := NewCommonT(c)

		collectorMock.AssertNumberOfCalls(ct, "CollectLog", 1)
		collectorMock.AssertNumberOfCalls(ct, "CollectMetric", 0)

		collectorMock.AssertCalled(ct, "CollectLog", "testNamespace", expectedLogEntry)
	}, 20*time.Second, 100*time.Millisecond)
}

func (suite *singletonTestSuite) TestStopCollection_CollectionNotStarted() {
	err := StopCollection(suite.ctx.Log())
	assert.ErrorContains(suite.T(), err, "telemetry collector is not started")
}

func (suite *singletonTestSuite) TestStopCollection() {
	// start telemetry collection
	err := StartCollection(suite.ctx)
	assert.NoError(suite.T(), err)

	consumer := singleton.consumer

	originalQuitChannel := singleton.consumer.consumerJob.Quit
	quitChannel := make(chan bool)
	singleton.consumer.consumerJob.Quit = quitChannel

	go func() {
		err = StopCollection(suite.ctx.Log())
		assert.NoError(suite.T(), err)
	}()

	<-quitChannel // wait for signal

	originalQuitChannel <- true
	<-consumer.getMessage()

	assert.EventuallyWithT(suite.T(), func(c *assert.CollectT) {
		assert.Nil(c, singleton)
	}, 2*time.Second, 10*time.Millisecond)
}

func (suite *singletonTestSuite) TestStopCollectionPanic() {
	backupPkgMutex := singletonMutex
	defer func() {
		singletonMutex = backupPkgMutex
	}()
	singletonMutex = nil

	err := StopCollection(suite.ctx.Log())

	assert.ErrorContains(suite.T(), err, "panic in telemetry collector StopCollection")
}

func (suite *singletonTestSuite) TestAddRemoveExporter_CollectionNotStarted() {
	exporterMock := exporterMocks.NewExporterMock()
	err := AddExporter(suite.ctx.Log(), exporterMock)
	assert.ErrorContains(suite.T(), err, "telemetry collector not initialized")

	err = RemoveExporter(suite.ctx.Log(), exporterMock)
	assert.ErrorContains(suite.T(), err, "telemetry collector not initialized")
}

func (suite *singletonTestSuite) TestAddRemoveExporter() {
	err := StartCollection(suite.ctx)
	assert.NoError(suite.T(), err)

	// replace the collector with mock
	collectorMock := collectorMocks.NewCollectorMock()
	singleton.collector = collectorMock
	collectorMock.On("Close").Return(nil).Once()

	exporterMock := exporterMocks.NewExporterMock()

	collectorMock.On("AddExporter", mock.Anything).Return(nil).Once()
	collectorMock.On("RemoveExporter", mock.Anything).Return(nil).Once()

	err = AddExporter(suite.ctx.Log(), exporterMock)
	assert.NoError(suite.T(), err)
	collectorMock.AssertNumberOfCalls(suite.T(), "AddExporter", 1)

	err = RemoveExporter(suite.ctx.Log(), exporterMock)
	assert.NoError(suite.T(), err)

	collectorMock.AssertNumberOfCalls(suite.T(), "RemoveExporter", 1)
}

func (suite *singletonTestSuite) TestAddExporterPanic() {
	err := StartCollection(suite.ctx)
	assert.NoError(suite.T(), err)

	// replace the collector with mock
	collectorMock := collectorMocks.NewCollectorMock()
	singleton.collector = collectorMock
	collectorMock.On("Close").Return(nil).Once()

	exporterMock := exporterMocks.NewExporterMock()

	collectorMock.On("AddExporter", mock.Anything).Panic("panic")

	err = AddExporter(suite.ctx.Log(), exporterMock)
	assert.ErrorContains(suite.T(), err, "panic in singleton collector AddExporter")
}

func (suite *singletonTestSuite) TestRemoveExporterPanic() {
	err := StartCollection(suite.ctx)
	assert.NoError(suite.T(), err)

	// replace the collector with mock
	collectorMock := collectorMocks.NewCollectorMock()
	singleton.collector = collectorMock
	collectorMock.On("Close").Return(nil).Once()

	collectorMock.On("Close").Return(nil).Once()
	collectorMock.On("RemoveExporter", mock.Anything).Panic("panic")

	err = RemoveExporter(suite.ctx.Log(), nil)
	assert.ErrorContains(suite.T(), err, "panic in singleton collector RemoveExporter")
}

// interface which allows us to use assert.CollectT as testing.T
// open issue in testify: https://github.com/stretchr/testify/issues/1414
type commonT struct {
	c *assert.CollectT
}

func (c *commonT) FailNow() {
	c.c.FailNow()
}

func (c *commonT) Errorf(format string, args ...interface{}) {
	c.c.Errorf(format, args...)
}

func (c *commonT) Logf(format string, args ...interface{}) {
	c.c.Errorf(format, args...)
}

func NewCommonT(c *assert.CollectT) *commonT {
	return &commonT{
		c: c,
	}
}

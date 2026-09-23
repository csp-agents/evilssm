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
package control_channel_exporter

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	logMock "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	commMock "github.com/aws/amazon-ssm-agent/agent/session/communicator/mocks"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/telemetry/collector"
	"github.com/aws/amazon-ssm-agent/agent/telemetry/datastores"
	dynamicconfiguration "github.com/aws/amazon-ssm-agent/agent/telemetry/dynamic_configuration"
	"github.com/aws/amazon-ssm-agent/common/telemetry/metric"
	"github.com/aws/amazon-ssm-agent/common/telemetry/telemetrylog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type controlChannelExporterTestSuite struct {
	suite.Suite

	ctx           *context.Mock
	mockWsChannel commMock.IWebSocketChannel
	exporter      *controlChannelTelemetryExporter
}

// Helper methods used by tests

func getLowerThanConfiguredPercentageLimit() float64 {
	return 22 // lower than configured percentage limit
}

func getHigherThanConfiguredPercentageLimit() float64 {
	return 82 // higher than configured percentage limit
}

// TestControlChannelExporterSuite executes test suite
func TestControlChannelExporterSuite(t *testing.T) {
	suite.Run(t, new(controlChannelExporterTestSuite))
}

// SetupTest makes sure that all the components referenced in the
// test case are initialized before each test
func (suite *controlChannelExporterTestSuite) SetupTest() {
	suite.ctx = context.NewMockDefault()
	suite.mockWsChannel = commMock.IWebSocketChannel{}

	suite.exporter = GetControlChannelTelemetryExporter(suite.ctx, &suite.mockWsChannel)
}

func (suite *controlChannelExporterTestSuite) TearDownTest() {
	suite.exporter.StopExporter()
}

func (suite *controlChannelExporterTestSuite) TestStartExporter() {
	collector.StartCollection(suite.ctx)
	defer collector.StopCollection(suite.ctx.Log())

	suite.exporter.StartExporter()

	log := suite.ctx.Log().(*logMock.Mock)

	log.AssertNotCalled(suite.T(), "Errorf", mock.Anything, mock.Anything)
	log.AssertNotCalled(suite.T(), "Error", mock.Anything)
}

func (suite *controlChannelExporterTestSuite) TestStartExporterNotInitialized() {
	suite.exporter.StartExporter()

	log := suite.exporter.ctx.Log().(*logMock.Mock)

	log.AssertCalled(suite.T(), "Errorf", mock.MatchedBy(func(msg string) bool {
		return strings.Contains(msg, "Error while adding telemetry exporter")
	}), mock.Anything)
}

func (suite *controlChannelExporterTestSuite) TestStopExporter() {
	collector.StartCollection(suite.ctx)
	defer collector.StopCollection(suite.ctx.Log())

	suite.exporter.StopExporter()

	log := suite.ctx.Log().(*logMock.Mock)

	log.AssertNotCalled(suite.T(), "Errorf", mock.Anything, mock.Anything)
	log.AssertNotCalled(suite.T(), "Error", mock.Anything)
}

func (suite *controlChannelExporterTestSuite) TestStopExporterNotInitialized() {
	suite.exporter.StopExporter()

	log := suite.exporter.ctx.Log().(*logMock.Mock)

	log.AssertCalled(suite.T(), "Errorf", mock.MatchedBy(func(msg string) bool {
		return strings.Contains(msg, "Error while removing telemetry exporter")
	}), mock.Anything)
}

func (suite *controlChannelExporterTestSuite) TestExportEmptyTelemetry() {
	// //Mocking this helper as this test does not have to know about all the implementation details
	controlChannelCheckTelemetryExportLuck = func(log log.T, namespace string) bool {
		return true
	}
	controlChannelIsTelemetryEnabled = func(log log.T, namespace string) bool {
		return true
	}
	controlChannelTooSoontoExportTelemetry = func(log log.T, namespace string) bool {
		return false
	}

	defer func() {
		controlChannelCheckTelemetryExportLuck = checkTelemetryExportLuck
	}()
	defer func() {
		controlChannelIsTelemetryEnabled = isTelemetryEnabled
	}()
	defer func() {
		controlChannelTooSoontoExportTelemetry = tooSoontoExportTelemetry
	}()
	err := suite.exporter.Export("testNamespace", []metric.Metric[float64]{}, []telemetrylog.Entry{})
	assert.NoError(suite.T(), err)

	suite.mockWsChannel.AssertNotCalled(suite.T(), "SendMessage")
}

func (suite *controlChannelExporterTestSuite) TestCheckTelemetryExportLucky() {
	randomPercentage = getLowerThanConfiguredPercentageLimit
	defer func() {
		randomPercentage = getRandomPercentage
	}()

	dynamicconfiguration.PercentageLimit = func(namespace string) float64 { return 50 }
	defer func() {
		dynamicconfiguration.PercentageLimit = dynamicconfiguration.GetPercentageLimit
	}()

	assert.Equal(suite.T(), true, controlChannelCheckTelemetryExportLuck(suite.ctx.Log(), "testNamespace"))
}

func (suite *controlChannelExporterTestSuite) TestCheckTelemetryExportUnlucky() {
	randomPercentage = getHigherThanConfiguredPercentageLimit
	defer func() {
		randomPercentage = getRandomPercentage
	}()

	dynamicconfiguration.PercentageLimit = func(namespace string) float64 { return 50 }
	defer func() {
		dynamicconfiguration.PercentageLimit = dynamicconfiguration.GetPercentageLimit
	}()

	assert.Equal(suite.T(), false, controlChannelCheckTelemetryExportLuck(suite.ctx.Log(), "testNamespace"))
}

func (suite *controlChannelExporterTestSuite) TestIsTelemetryDisabled() {
	dynamicconfiguration.TelemetryDisabledTill = func(string) int64 {
		return 2 * time.Now().Unix()
	}
	defer func() {
		dynamicconfiguration.TelemetryDisabledTill = dynamicconfiguration.GetTelemetryDisabledTill
	}()

	assert.Equal(suite.T(), false, controlChannelIsTelemetryEnabled(suite.ctx.Log(), "testNamespace"))
}

func (suite *controlChannelExporterTestSuite) TestIsTelemetryEnabled() {
	dynamicconfiguration.TelemetryDisabledTill = func(string) int64 {
		return 0
	}
	defer func() {
		dynamicconfiguration.TelemetryDisabledTill = dynamicconfiguration.GetTelemetryDisabledTill
	}()

	assert.Equal(suite.T(), true, controlChannelIsTelemetryEnabled(suite.ctx.Log(), "testNamespace"))
}

func (suite *controlChannelExporterTestSuite) TestTooSoonToExportTelemetry() {
	dynamicconfiguration.ExportPeriod = func(string) int {
		return 15
	}
	defer func() {
		dynamicconfiguration.ExportPeriod = dynamicconfiguration.GetExportPeriod
	}()
	datastores.TelemetryLastEmittedDataStore = func() *datastores.LastEmittedDataStore {
		lsd := make(datastores.LastEmittedDataStore)
		lsd["testNamespace"] = time.Now().Unix()
		return &lsd
	}
	defer func() {
		datastores.TelemetryLastEmittedDataStore = datastores.GetLastEmittedDataStore
	}()
	assert.Equal(suite.T(), true, controlChannelTooSoontoExportTelemetry(suite.ctx.Log(), "testNamespace"))
}

func (suite *controlChannelExporterTestSuite) TestNotTooSoonToExportTelemetry() {
	dynamicconfiguration.ExportPeriod = func(string) int {
		return 15
	}
	defer func() {
		dynamicconfiguration.ExportPeriod = dynamicconfiguration.GetExportPeriod
	}()
	datastores.TelemetryLastEmittedDataStore = func() *datastores.LastEmittedDataStore {
		lsd := make(datastores.LastEmittedDataStore)
		lsd["testNamespace"] = 0
		return &lsd
	}
	defer func() {
		datastores.TelemetryLastEmittedDataStore = datastores.GetLastEmittedDataStore
	}()
	assert.Equal(suite.T(), false, controlChannelTooSoontoExportTelemetry(suite.ctx.Log(), "testNamespace"))
}

func (suite *controlChannelExporterTestSuite) TestUpdateLastEmittedDataStore() {
	lsd := make(datastores.LastEmittedDataStore)
	datastores.TelemetryLastEmittedDataStore = func() *datastores.LastEmittedDataStore {
		return &lsd
	}
	defer func() {
		datastores.TelemetryLastEmittedDataStore = datastores.GetLastEmittedDataStore
	}()
	updateLastEmittedTimestamp("testNamespace", 12)
	assert.Equal(suite.T(), int64(12), lsd.Read("testNamespace"))
}

func (suite *controlChannelExporterTestSuite) TestExportTelemetryExportsWhenLucky() {
	// ARRANGE
	now := time.Now().UTC()

	// prepare test telemetry
	namespace := "testNamespace"

	sentMetrics := make([]metric.Metric[float64], 0)
	sentLogs := make([]telemetrylog.Entry, 0)

	expectedMetrics := make([]Metric, 0)
	expectedLogs := make([]LogEntry, 0)

	for j := range 10 {
		metricName := fmt.Sprintf("testMetric%v", j)
		sentMetrics = append(sentMetrics, metric.Metric[float64]{
			Name:       metricName,
			Unit:       "1",
			Kind:       metric.Sum,
			DataPoints: []metric.DataPoint[float64]{{StartTime: now, EndTime: now.Add(time.Second), Value: 100}},
		})
		expectedMetrics = append(expectedMetrics, Metric{
			Name:       metricName,
			Unit:       "1",
			DataPoints: []DataPoint{{Time: now, Value: 100}},
		})

		sentLogs = append(sentLogs, telemetrylog.Entry{
			Time:     now,
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message %v", j),
		})
		expectedLogs = append(expectedLogs, LogEntry{
			Time:     now,
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message %v", j),
		})
	}

	// set expectations
	receivedMessage := &mgsContracts.AgentMessage{}

	suite.mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		// verify telemetry is sent
		message := args.Get(1).([]byte)
		assert.NotEmpty(suite.T(), message)

		err := receivedMessage.Deserialize(suite.ctx.Log(), message)
		require.NoError(suite.T(), err)
	}).Return(nil)

	randomPercentage = getLowerThanConfiguredPercentageLimit
	defer func() {
		randomPercentage = getRandomPercentage
	}()

	dynamicconfiguration.PercentageLimit = func(namespace string) float64 { return 50 }
	defer func() {
		dynamicconfiguration.PercentageLimit = dynamicconfiguration.GetPercentageLimit
	}()

	controlChannelIsTelemetryEnabled = func(log log.T, namespace string) bool {
		return true
	}
	controlChannelTooSoontoExportTelemetry = func(log log.T, namespace string) bool {
		return false
	}
	defer func() {
		controlChannelIsTelemetryEnabled = isTelemetryEnabled
	}()
	defer func() {
		controlChannelTooSoontoExportTelemetry = tooSoontoExportTelemetry
	}()

	// ACT
	err := suite.exporter.Export(namespace, sentMetrics, sentLogs)

	// ASSERT
	assert.NoError(suite.T(), err)

	suite.mockWsChannel.AssertNumberOfCalls(suite.T(), "SendMessage", 1)

	payload := receivedMessage.Payload

	agentTelemetryV2 := AgentTelemetryV2{}
	err = json.Unmarshal(payload, &agentTelemetryV2)
	assert.NoError(suite.T(), err)

	receivedInnerPayload := AgentTelemetryV2Payload{}
	err = json.Unmarshal([]byte(agentTelemetryV2.Payload), &receivedInnerPayload)
	assert.NoError(suite.T(), err)

	assert.Equal(suite.T(), uint32(1), agentTelemetryV2.SchemaVersion, "Schema version changed! Is this expected? If yes, only then update this test")

	assert.Equal(suite.T(), expectedMetrics, receivedInnerPayload.Metrics)
	assert.Equal(suite.T(), expectedLogs, receivedInnerPayload.Logs)
}

func (suite *controlChannelExporterTestSuite) TestExportTelemetryExportsWhenLuckyForFloatingPercentage() {
	// ARRANGE
	now := time.Now().UTC()

	// prepare test telemetry
	namespace := "testNamespace"

	sentMetrics := make([]metric.Metric[float64], 0)
	sentLogs := make([]telemetrylog.Entry, 0)

	expectedMetrics := make([]Metric, 0)
	expectedLogs := make([]LogEntry, 0)

	for j := range 10 {
		metricName := fmt.Sprintf("testMetric%v", j)
		sentMetrics = append(sentMetrics, metric.Metric[float64]{
			Name:       metricName,
			Unit:       "1",
			Kind:       metric.Sum,
			DataPoints: []metric.DataPoint[float64]{{StartTime: now, EndTime: now.Add(time.Second), Value: 100}},
		})
		expectedMetrics = append(expectedMetrics, Metric{
			Name:       metricName,
			Unit:       "1",
			DataPoints: []DataPoint{{Time: now, Value: 100}},
		})

		sentLogs = append(sentLogs, telemetrylog.Entry{
			Time:     now,
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message %v", j),
		})
		expectedLogs = append(expectedLogs, LogEntry{
			Time:     now,
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message %v", j),
		})
	}

	// set expectations
	receivedMessage := &mgsContracts.AgentMessage{}

	suite.mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		// verify telemetry is sent
		message := args.Get(1).([]byte)
		assert.NotEmpty(suite.T(), message)

		err := receivedMessage.Deserialize(suite.ctx.Log(), message)
		require.NoError(suite.T(), err)
	}).Return(nil)

	randomPercentage = func() float64 { return 0.049999 }
	defer func() {
		randomPercentage = getRandomPercentage
	}()

	dynamicconfiguration.PercentageLimit = func(namespace string) float64 { return 0.05 }
	defer func() {
		dynamicconfiguration.PercentageLimit = dynamicconfiguration.GetPercentageLimit
	}()

	controlChannelIsTelemetryEnabled = func(log log.T, namespace string) bool {
		return true
	}
	controlChannelTooSoontoExportTelemetry = func(log log.T, namespace string) bool {
		return false
	}
	defer func() {
		controlChannelIsTelemetryEnabled = isTelemetryEnabled
	}()
	defer func() {
		controlChannelTooSoontoExportTelemetry = tooSoontoExportTelemetry
	}()

	// ACT
	err := suite.exporter.Export(namespace, sentMetrics, sentLogs)

	// ASSERT
	assert.NoError(suite.T(), err)

	suite.mockWsChannel.AssertNumberOfCalls(suite.T(), "SendMessage", 1)

	payload := receivedMessage.Payload

	agentTelemetryV2 := AgentTelemetryV2{}
	err = json.Unmarshal(payload, &agentTelemetryV2)
	assert.NoError(suite.T(), err)

	receivedInnerPayload := AgentTelemetryV2Payload{}
	err = json.Unmarshal([]byte(agentTelemetryV2.Payload), &receivedInnerPayload)
	assert.NoError(suite.T(), err)

	assert.Equal(suite.T(), uint32(1), agentTelemetryV2.SchemaVersion, "Schema version changed! Is this expected? If yes, only then update this test")

	assert.Equal(suite.T(), expectedMetrics, receivedInnerPayload.Metrics)
	assert.Equal(suite.T(), expectedLogs, receivedInnerPayload.Logs)
}

func (suite *controlChannelExporterTestSuite) TestExportTelemetryDoesNotExportWhenUnLucky() {
	// ARRANGE
	now := time.Now().UTC()

	// prepare test telemetry
	namespace := "testNamespace"

	sentMetrics := make([]metric.Metric[float64], 0)
	sentLogs := make([]telemetrylog.Entry, 0)

	expectedMetrics := make([]Metric, 0)
	expectedLogs := make([]LogEntry, 0)

	for j := range 10 {
		metricName := fmt.Sprintf("testMetric%v", j)
		sentMetrics = append(sentMetrics, metric.Metric[float64]{
			Name:       metricName,
			Unit:       "1",
			Kind:       metric.Sum,
			DataPoints: []metric.DataPoint[float64]{{StartTime: now, EndTime: now.Add(time.Second), Value: 100}},
		})
		expectedMetrics = append(expectedMetrics, Metric{
			Name:       metricName,
			Unit:       "1",
			DataPoints: []DataPoint{{Time: now, Value: 100}},
		})

		sentLogs = append(sentLogs, telemetrylog.Entry{
			Time:     now,
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message %v", j),
		})
		expectedLogs = append(expectedLogs, LogEntry{
			Time:     now,
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message %v", j),
		})
	}

	randomPercentage = getHigherThanConfiguredPercentageLimit
	defer func() {
		randomPercentage = getRandomPercentage
	}()

	dynamicconfiguration.PercentageLimit = func(namespace string) float64 { return 50 }
	defer func() {
		dynamicconfiguration.PercentageLimit = dynamicconfiguration.GetPercentageLimit
	}()

	controlChannelIsTelemetryEnabled = func(log log.T, namespace string) bool {
		return true
	}
	controlChannelTooSoontoExportTelemetry = func(log log.T, namespace string) bool {
		return false
	}
	defer func() {
		controlChannelIsTelemetryEnabled = isTelemetryEnabled
	}()
	defer func() {
		controlChannelTooSoontoExportTelemetry = tooSoontoExportTelemetry
	}()

	// ACT
	err := suite.exporter.Export(namespace, sentMetrics, sentLogs)

	// ASSERT
	assert.NoError(suite.T(), err)

	suite.mockWsChannel.AssertNotCalled(suite.T(), "SendMessage")
}

func (suite *controlChannelExporterTestSuite) TestExportTelemetryDoesNotExportWhenUnLuckyWithFloatingPercentage() {
	// ARRANGE
	now := time.Now().UTC()

	// prepare test telemetry
	namespace := "testNamespace"

	sentMetrics := make([]metric.Metric[float64], 0)
	sentLogs := make([]telemetrylog.Entry, 0)

	expectedMetrics := make([]Metric, 0)
	expectedLogs := make([]LogEntry, 0)

	for j := range 10 {
		metricName := fmt.Sprintf("testMetric%v", j)
		sentMetrics = append(sentMetrics, metric.Metric[float64]{
			Name:       metricName,
			Unit:       "1",
			Kind:       metric.Sum,
			DataPoints: []metric.DataPoint[float64]{{StartTime: now, EndTime: now.Add(time.Second), Value: 100}},
		})
		expectedMetrics = append(expectedMetrics, Metric{
			Name:       metricName,
			Unit:       "1",
			DataPoints: []DataPoint{{Time: now, Value: 100}},
		})

		sentLogs = append(sentLogs, telemetrylog.Entry{
			Time:     now,
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message %v", j),
		})
		expectedLogs = append(expectedLogs, LogEntry{
			Time:     now,
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message %v", j),
		})
	}

	randomPercentage = func() float64 { return 0.05000001 }

	defer func() {
		randomPercentage = getRandomPercentage
	}()

	dynamicconfiguration.PercentageLimit = func(namespace string) float64 { return 0.05 }
	defer func() {
		dynamicconfiguration.PercentageLimit = dynamicconfiguration.GetPercentageLimit
	}()

	controlChannelIsTelemetryEnabled = func(log log.T, namespace string) bool {
		return true
	}
	controlChannelTooSoontoExportTelemetry = func(log log.T, namespace string) bool {
		return false
	}
	defer func() {
		controlChannelIsTelemetryEnabled = isTelemetryEnabled
	}()
	defer func() {
		controlChannelTooSoontoExportTelemetry = tooSoontoExportTelemetry
	}()

	// ACT
	err := suite.exporter.Export(namespace, sentMetrics, sentLogs)

	// ASSERT
	assert.NoError(suite.T(), err)

	suite.mockWsChannel.AssertNotCalled(suite.T(), "SendMessage")
}

func (suite *controlChannelExporterTestSuite) TestExportTelemetryExportsWhenTelemetryEnabledForNamespace() {
	// ARRANGE
	now := time.Now().UTC()

	// prepare test telemetry
	namespace := "testNamespace"

	sentMetrics := make([]metric.Metric[float64], 0)
	sentLogs := make([]telemetrylog.Entry, 0)

	expectedMetrics := make([]Metric, 0)
	expectedLogs := make([]LogEntry, 0)

	for j := range 10 {
		metricName := fmt.Sprintf("testMetric%v", j)
		sentMetrics = append(sentMetrics, metric.Metric[float64]{
			Name:       metricName,
			Unit:       "1",
			Kind:       metric.Sum,
			DataPoints: []metric.DataPoint[float64]{{StartTime: now, EndTime: now.Add(time.Second), Value: 100}},
		})
		expectedMetrics = append(expectedMetrics, Metric{
			Name:       metricName,
			Unit:       "1",
			DataPoints: []DataPoint{{Time: now, Value: 100}},
		})

		sentLogs = append(sentLogs, telemetrylog.Entry{
			Time:     now,
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message %v", j),
		})
		expectedLogs = append(expectedLogs, LogEntry{
			Time:     now,
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message %v", j),
		})
	}

	// set expectations
	receivedMessage := &mgsContracts.AgentMessage{}

	suite.mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		// verify telemetry is sent
		message := args.Get(1).([]byte)
		assert.NotEmpty(suite.T(), message)

		err := receivedMessage.Deserialize(suite.ctx.Log(), message)
		require.NoError(suite.T(), err)
	}).Return(nil)

	controlChannelCheckTelemetryExportLuck = func(log log.T, namespace string) bool {
		return true
	}
	controlChannelIsTelemetryEnabled = func(log log.T, namespace string) bool {
		return true
	}
	controlChannelTooSoontoExportTelemetry = func(log log.T, namespace string) bool {
		return false
	}
	defer func() {
		controlChannelCheckTelemetryExportLuck = checkTelemetryExportLuck
	}()
	defer func() {
		controlChannelIsTelemetryEnabled = isTelemetryEnabled
	}()
	defer func() {
		controlChannelTooSoontoExportTelemetry = tooSoontoExportTelemetry
	}()

	// ACT
	err := suite.exporter.Export(namespace, sentMetrics, sentLogs)

	// ASSERT
	assert.NoError(suite.T(), err)

	suite.mockWsChannel.AssertNumberOfCalls(suite.T(), "SendMessage", 1)

	payload := receivedMessage.Payload

	agentTelemetryV2 := AgentTelemetryV2{}
	err = json.Unmarshal(payload, &agentTelemetryV2)
	assert.NoError(suite.T(), err)

	receivedInnerPayload := AgentTelemetryV2Payload{}
	err = json.Unmarshal([]byte(agentTelemetryV2.Payload), &receivedInnerPayload)
	assert.NoError(suite.T(), err)

	assert.Equal(suite.T(), uint32(1), agentTelemetryV2.SchemaVersion, "Schema version changed! Is this expected? If yes, only then update this test")

	assert.Equal(suite.T(), expectedMetrics, receivedInnerPayload.Metrics)
	assert.Equal(suite.T(), expectedLogs, receivedInnerPayload.Logs)
}

func (suite *controlChannelExporterTestSuite) TestExportTelemetryDoesNotExportWhenTelemetryDisabledForNamespace() {
	// ARRANGE
	now := time.Now().UTC()

	// prepare test telemetry
	namespace := "testNamespace"

	sentMetrics := make([]metric.Metric[float64], 0)
	sentLogs := make([]telemetrylog.Entry, 0)

	expectedMetrics := make([]Metric, 0)
	expectedLogs := make([]LogEntry, 0)

	for j := range 10 {
		metricName := fmt.Sprintf("testMetric%v", j)
		sentMetrics = append(sentMetrics, metric.Metric[float64]{
			Name:       metricName,
			Unit:       "1",
			Kind:       metric.Sum,
			DataPoints: []metric.DataPoint[float64]{{StartTime: now, EndTime: now.Add(time.Second), Value: 100}},
		})
		expectedMetrics = append(expectedMetrics, Metric{
			Name:       metricName,
			Unit:       "1",
			DataPoints: []DataPoint{{Time: now, Value: 100}},
		})

		sentLogs = append(sentLogs, telemetrylog.Entry{
			Time:     now,
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message %v", j),
		})
		expectedLogs = append(expectedLogs, LogEntry{
			Time:     now,
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message %v", j),
		})
	}

	controlChannelCheckTelemetryExportLuck = func(log log.T, namespace string) bool {
		return true
	}
	controlChannelIsTelemetryEnabled = func(log log.T, namespace string) bool {
		return false
	}
	controlChannelTooSoontoExportTelemetry = func(log log.T, namespace string) bool {
		return false
	}
	defer func() {
		controlChannelCheckTelemetryExportLuck = checkTelemetryExportLuck
	}()
	defer func() {
		controlChannelIsTelemetryEnabled = isTelemetryEnabled
	}()
	defer func() {
		controlChannelTooSoontoExportTelemetry = tooSoontoExportTelemetry
	}()

	// ACT
	err := suite.exporter.Export(namespace, sentMetrics, sentLogs)

	// ASSERT
	assert.NoError(suite.T(), err)

	suite.mockWsChannel.AssertNotCalled(suite.T(), "SendMessage")
}

func (suite *controlChannelExporterTestSuite) TestExportTelemetryExportsWhenNotTooSoonSinceLastEmission() {
	// ARRANGE
	now := time.Now().UTC()

	// prepare test telemetry
	namespace := "testNamespace"

	sentMetrics := make([]metric.Metric[float64], 0)
	sentLogs := make([]telemetrylog.Entry, 0)

	expectedMetrics := make([]Metric, 0)
	expectedLogs := make([]LogEntry, 0)

	for j := range 10 {
		metricName := fmt.Sprintf("testMetric%v", j)
		sentMetrics = append(sentMetrics, metric.Metric[float64]{
			Name:       metricName,
			Unit:       "1",
			Kind:       metric.Sum,
			DataPoints: []metric.DataPoint[float64]{{StartTime: now, EndTime: now.Add(time.Second), Value: 100}},
		})
		expectedMetrics = append(expectedMetrics, Metric{
			Name:       metricName,
			Unit:       "1",
			DataPoints: []DataPoint{{Time: now, Value: 100}},
		})

		sentLogs = append(sentLogs, telemetrylog.Entry{
			Time:     now,
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message %v", j),
		})
		expectedLogs = append(expectedLogs, LogEntry{
			Time:     now,
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message %v", j),
		})
	}

	// set expectations
	receivedMessage := &mgsContracts.AgentMessage{}

	suite.mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		// verify telemetry is sent
		message := args.Get(1).([]byte)
		assert.NotEmpty(suite.T(), message)

		err := receivedMessage.Deserialize(suite.ctx.Log(), message)
		require.NoError(suite.T(), err)
	}).Return(nil)

	controlChannelCheckTelemetryExportLuck = func(log log.T, namespace string) bool {
		return true
	}
	controlChannelIsTelemetryEnabled = func(log log.T, namespace string) bool {
		return true
	}
	controlChannelTooSoontoExportTelemetry = func(log log.T, namespace string) bool {
		return false
	}
	defer func() {
		controlChannelCheckTelemetryExportLuck = checkTelemetryExportLuck
	}()
	defer func() {
		controlChannelIsTelemetryEnabled = isTelemetryEnabled
	}()
	defer func() {
		controlChannelTooSoontoExportTelemetry = tooSoontoExportTelemetry
	}()
	// ACT
	err := suite.exporter.Export(namespace, sentMetrics, sentLogs)

	// ASSERT
	assert.NoError(suite.T(), err)

	suite.mockWsChannel.AssertNumberOfCalls(suite.T(), "SendMessage", 1)

	payload := receivedMessage.Payload

	agentTelemetryV2 := AgentTelemetryV2{}
	err = json.Unmarshal(payload, &agentTelemetryV2)
	assert.NoError(suite.T(), err)

	receivedInnerPayload := AgentTelemetryV2Payload{}
	err = json.Unmarshal([]byte(agentTelemetryV2.Payload), &receivedInnerPayload)
	assert.NoError(suite.T(), err)

	assert.Equal(suite.T(), uint32(1), agentTelemetryV2.SchemaVersion, "Schema version changed! Is this expected? If yes, only then update this test")

	assert.Equal(suite.T(), expectedMetrics, receivedInnerPayload.Metrics)
	assert.Equal(suite.T(), expectedLogs, receivedInnerPayload.Logs)
}

func (suite *controlChannelExporterTestSuite) TestExportTelemetryDoesNotExportWhenTooSoonSinceLastEmission() {
	// ARRANGE
	now := time.Now().UTC()

	// prepare test telemetry
	namespace := "testNamespace"

	sentMetrics := make([]metric.Metric[float64], 0)
	sentLogs := make([]telemetrylog.Entry, 0)

	expectedMetrics := make([]Metric, 0)
	expectedLogs := make([]LogEntry, 0)

	for j := range 10 {
		metricName := fmt.Sprintf("testMetric%v", j)
		sentMetrics = append(sentMetrics, metric.Metric[float64]{
			Name:       metricName,
			Unit:       "1",
			Kind:       metric.Sum,
			DataPoints: []metric.DataPoint[float64]{{StartTime: now, EndTime: now.Add(time.Second), Value: 100}},
		})
		expectedMetrics = append(expectedMetrics, Metric{
			Name:       metricName,
			Unit:       "1",
			DataPoints: []DataPoint{{Time: now, Value: 100}},
		})

		sentLogs = append(sentLogs, telemetrylog.Entry{
			Time:     now,
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message %v", j),
		})
		expectedLogs = append(expectedLogs, LogEntry{
			Time:     now,
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message %v", j),
		})
	}

	controlChannelCheckTelemetryExportLuck = func(log log.T, namespace string) bool {
		return true
	}
	controlChannelIsTelemetryEnabled = func(log log.T, namespace string) bool {
		return true
	}
	controlChannelTooSoontoExportTelemetry = func(log log.T, namespace string) bool {
		return true
	}
	defer func() {
		controlChannelCheckTelemetryExportLuck = checkTelemetryExportLuck
	}()
	defer func() {
		controlChannelIsTelemetryEnabled = isTelemetryEnabled
	}()
	defer func() {
		controlChannelTooSoontoExportTelemetry = tooSoontoExportTelemetry
	}()

	// ACT
	err := suite.exporter.Export(namespace, sentMetrics, sentLogs)

	// ASSERT
	assert.NoError(suite.T(), err)

	suite.mockWsChannel.AssertNotCalled(suite.T(), "SendMessage")
}

func (suite *controlChannelExporterTestSuite) TestExportTelemetryUpdatesLastEmittedTimestampAfterExporting() {
	// ARRANGE
	now := time.Now().UTC()

	// prepare test telemetry
	namespace := "testNamespace"

	sentMetrics := make([]metric.Metric[float64], 0)
	sentLogs := make([]telemetrylog.Entry, 0)

	expectedMetrics := make([]Metric, 0)
	expectedLogs := make([]LogEntry, 0)

	for j := range 10 {
		metricName := fmt.Sprintf("testMetric%v", j)
		sentMetrics = append(sentMetrics, metric.Metric[float64]{
			Name:       metricName,
			Unit:       "1",
			Kind:       metric.Sum,
			DataPoints: []metric.DataPoint[float64]{{StartTime: now, EndTime: now.Add(time.Second), Value: 100}},
		})
		expectedMetrics = append(expectedMetrics, Metric{
			Name:       metricName,
			Unit:       "1",
			DataPoints: []DataPoint{{Time: now, Value: 100}},
		})

		sentLogs = append(sentLogs, telemetrylog.Entry{
			Time:     now,
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message %v", j),
		})
		expectedLogs = append(expectedLogs, LogEntry{
			Time:     now,
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message %v", j),
		})
	}

	// set expectations
	receivedMessage := &mgsContracts.AgentMessage{}

	suite.mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		// verify telemetry is sent
		message := args.Get(1).([]byte)
		assert.NotEmpty(suite.T(), message)

		err := receivedMessage.Deserialize(suite.ctx.Log(), message)
		require.NoError(suite.T(), err)
	}).Return(nil)

	lsd := make(datastores.LastEmittedDataStore)
	datastores.TelemetryLastEmittedDataStore = func() *datastores.LastEmittedDataStore {
		return &lsd
	}
	defer func() {
		datastores.TelemetryLastEmittedDataStore = datastores.GetLastEmittedDataStore
	}()

	controlChannelCheckTelemetryExportLuck = func(log log.T, namespace string) bool {
		return true
	}
	controlChannelIsTelemetryEnabled = func(log log.T, namespace string) bool {
		return true
	}
	controlChannelTooSoontoExportTelemetry = func(log log.T, namespace string) bool {
		return false
	}
	defer func() {
		controlChannelCheckTelemetryExportLuck = checkTelemetryExportLuck
	}()
	defer func() {
		controlChannelIsTelemetryEnabled = isTelemetryEnabled
	}()
	defer func() {
		controlChannelTooSoontoExportTelemetry = tooSoontoExportTelemetry
	}()

	// ACT
	err := suite.exporter.Export(namespace, sentMetrics, sentLogs)

	// ASSERT
	assert.NoError(suite.T(), err)

	suite.mockWsChannel.AssertNumberOfCalls(suite.T(), "SendMessage", 1)

	assert.Equal(suite.T(), time.Now().Unix(), lsd[namespace])

	payload := receivedMessage.Payload

	agentTelemetryV2 := AgentTelemetryV2{}
	err = json.Unmarshal(payload, &agentTelemetryV2)
	assert.NoError(suite.T(), err)

	receivedInnerPayload := AgentTelemetryV2Payload{}
	err = json.Unmarshal([]byte(agentTelemetryV2.Payload), &receivedInnerPayload)
	assert.NoError(suite.T(), err)

	assert.Equal(suite.T(), uint32(1), agentTelemetryV2.SchemaVersion, "Schema version changed! Is this expected? If yes, only then update this test")

	assert.Equal(suite.T(), expectedMetrics, receivedInnerPayload.Metrics)
	assert.Equal(suite.T(), expectedLogs, receivedInnerPayload.Logs)
}

func (suite *controlChannelExporterTestSuite) TestExportTelemetryDoesNotUpdateLastEmittedTimestampForNoExport() {
	// ARRANGE
	now := time.Now().UTC()

	// prepare test telemetry
	namespace := "testNamespace"

	sentMetrics := make([]metric.Metric[float64], 0)
	sentLogs := make([]telemetrylog.Entry, 0)

	expectedMetrics := make([]Metric, 0)
	expectedLogs := make([]LogEntry, 0)

	for j := range 10 {
		metricName := fmt.Sprintf("testMetric%v", j)
		sentMetrics = append(sentMetrics, metric.Metric[float64]{
			Name:       metricName,
			Unit:       "1",
			Kind:       metric.Sum,
			DataPoints: []metric.DataPoint[float64]{{StartTime: now, EndTime: now.Add(time.Second), Value: 100}},
		})
		expectedMetrics = append(expectedMetrics, Metric{
			Name:       metricName,
			Unit:       "1",
			DataPoints: []DataPoint{{Time: now, Value: 100}},
		})

		sentLogs = append(sentLogs, telemetrylog.Entry{
			Time:     now,
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message %v", j),
		})
		expectedLogs = append(expectedLogs, LogEntry{
			Time:     now,
			Severity: telemetrylog.ERROR,
			Body:     fmt.Sprintf("This is a test message %v", j),
		})
	}

	lsd := make(datastores.LastEmittedDataStore)
	datastores.TelemetryLastEmittedDataStore = func() *datastores.LastEmittedDataStore {
		return &lsd
	}
	defer func() {
		datastores.TelemetryLastEmittedDataStore = datastores.GetLastEmittedDataStore
	}()

	controlChannelCheckTelemetryExportLuck = func(log log.T, namespace string) bool {
		return true
	}
	controlChannelIsTelemetryEnabled = func(log log.T, namespace string) bool {
		return true
	}
	controlChannelTooSoontoExportTelemetry = func(log log.T, namespace string) bool {
		return true
	}

	defer func() {
		controlChannelCheckTelemetryExportLuck = checkTelemetryExportLuck
	}()
	defer func() {
		controlChannelIsTelemetryEnabled = isTelemetryEnabled
	}()
	defer func() {
		controlChannelTooSoontoExportTelemetry = tooSoontoExportTelemetry
	}()

	// ACT
	err := suite.exporter.Export(namespace, sentMetrics, sentLogs)

	// ASSERT
	assert.NoError(suite.T(), err)

	suite.mockWsChannel.AssertNotCalled(suite.T(), "SendMessage")
	assert.Equal(suite.T(), int64(0), lsd[namespace])
}

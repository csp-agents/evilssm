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
package internal

import (
	"io"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/aws/amazon-ssm-agent/agent/telemetry/collector/internal/mocks"
	exporterMocks "github.com/aws/amazon-ssm-agent/agent/telemetry/exporter/mocks"
	"github.com/aws/amazon-ssm-agent/common/telemetry/metric"
	"github.com/aws/amazon-ssm-agent/common/telemetry/telemetrylog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type collectorTestSuite struct {
	suite.Suite
	ctx *context.Mock
}

func TestCollectorTestSuite(t *testing.T) {
	suite.Run(t, new(collectorTestSuite))
}

// SetupTest makes sure that all the components referenced in the test case are initialized
// before each test
func (suite *collectorTestSuite) SetupTest() {
	suite.ctx = context.NewMockDefault()
}

func (suite *collectorTestSuite) TestExportSchedule() {
	metricCollectorMock := mocks.NewSlowMetricsCollectorMock()
	logCollectorMock := mocks.NewLogCollectorMock()

	c, err := NewCollector(suite.ctx, time.Minute, 5*time.Minute)
	assert.NoError(suite.T(), err)

	collector := c.(*collectorT)
	collector.metricCollector = metricCollectorMock
	collector.logCollector = logCollectorMock
	const exportSchedulerInitialJitterInSeconds = 5
	collector.exportSchedulerInitialJitter = time.Duration(exportSchedulerInitialJitterInSeconds) * time.Second
	waitTime := time.Duration(2*exportSchedulerInitialJitterInSeconds) * time.Second
	exporterMock := exporterMocks.NewExporterMock()

	collector.AddExporter(exporterMock)

	metrics := metric.NamespaceMetrics[float64]{
		"namespace1": []metric.Metric[float64]{{
			Name: "metric1",
			Unit: "1",
			Kind: metric.Sum,
			DataPoints: []metric.DataPoint[float64]{
				{
					StartTime: time.Now(),
					EndTime:   time.Now(),
					Value:     1.0,
				},
			},
		},
			{
				Name: "metric2",
				Unit: "1",
				Kind: metric.Sum,
				DataPoints: []metric.DataPoint[float64]{
					{
						StartTime: time.Now(),
						EndTime:   time.Now(),
						Value:     1.0,
					},
				},
			},
		},
		"namespace2": []metric.Metric[float64]{{
			Name: "metric2",
			Unit: "1",
			Kind: metric.Sum,
			DataPoints: []metric.DataPoint[float64]{
				{
					StartTime: time.Now(),
					EndTime:   time.Now(),
					Value:     1.0,
				},
				{
					StartTime: time.Now(),
					EndTime:   time.Now(),
					Value:     3.0,
				},
			},
		},
			{
				Name: "metric1",
				Unit: "1",
				Kind: metric.Sum,
				DataPoints: []metric.DataPoint[float64]{
					{
						StartTime: time.Now(),
						EndTime:   time.Now(),
						Value:     1.0,
					},
				},
			}},
	}

	logs := telemetrylog.NamespaceLogs{
		"namespace1": []telemetrylog.Entry{
			{

				Time:     time.Now(),
				Severity: telemetrylog.ERROR,
				Body:     "message1",
			},
			{

				Time:     time.Now(),
				Severity: telemetrylog.ERROR,
				Body:     "message2",
			},
		},
		"namespace2": []telemetrylog.Entry{
			{

				Time:     time.Now(),
				Severity: telemetrylog.ERROR,
				Body:     "message1",
			},
			{

				Time:     time.Now(),
				Severity: telemetrylog.ERROR,
				Body:     "message2",
			},
		},
	}

	metricCollectorMock.On("FetchAndDrop", mock.Anything).Return(metrics, io.EOF).Once()
	logCollectorMock.On("FetchAndDrop", mock.Anything).Return(logs, io.EOF).Once()

	exporterMock.On("Export", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Sleep till exportSchedulerJob becomes available
	time.Sleep(waitTime)
	// skip the scheduler wait
	collector.exportSchedulerJob.SkipWait <- true

	assert.EventuallyWithT(suite.T(), func(c *assert.CollectT) {
		ct := NewCommonT(c)

		metricCollectorMock.AssertNumberOfCalls(ct, "FetchAndDrop", 1)
		logCollectorMock.AssertNumberOfCalls(ct, "FetchAndDrop", 1)

		exporterMock.AssertNumberOfCalls(ct, "Export", 2)
		exporterMock.AssertCalled(ct, "Export", "namespace1", metrics["namespace1"], logs["namespace1"])
		exporterMock.AssertCalled(ct, "Export", "namespace2", metrics["namespace2"], logs["namespace2"])
	}, 5*time.Second, 50*time.Millisecond)
}

func (suite *collectorTestSuite) TestCollectLog() {
	metricCollectorMock := mocks.NewSlowMetricsCollectorMock()
	logCollectorMock := mocks.NewLogCollectorMock()

	c, err := NewCollector(suite.ctx, time.Minute, 5*time.Minute)
	assert.NoError(suite.T(), err)

	collector := c.(*collectorT)
	collector.metricCollector = metricCollectorMock
	collector.logCollector = logCollectorMock

	logCollectorMock.On("CollectLog", mock.Anything, mock.Anything).Return(nil)

	log1 := telemetrylog.Entry{
		Time:     time.Now(),
		Severity: telemetrylog.ERROR,
		Body:     "message1",
	}
	collector.CollectLog("namespace1", log1)

	log2 := telemetrylog.Entry{
		Time:     time.Now(),
		Severity: telemetrylog.CRITICAL,
		Body:     "message2",
	}
	collector.CollectLog("namespace2", log2)

	logCollectorMock.AssertNumberOfCalls(suite.T(), "CollectLog", 2)
	logCollectorMock.AssertCalled(suite.T(), "CollectLog", "namespace1", log1)
	logCollectorMock.AssertCalled(suite.T(), "CollectLog", "namespace2", log2)
}

func (suite *collectorTestSuite) TestCollectMetric() {
	metricCollectorMock := mocks.NewSlowMetricsCollectorMock()
	logCollectorMock := mocks.NewLogCollectorMock()

	c, err := NewCollector(suite.ctx, time.Minute, 5*time.Minute)
	assert.NoError(suite.T(), err)

	collector := c.(*collectorT)
	collector.metricCollector = metricCollectorMock
	collector.logCollector = logCollectorMock

	metricCollectorMock.On("CollectMetric", mock.Anything, mock.Anything).Return(nil)

	metric1 := metric.Metric[float64]{
		Name: "metric1",
		Unit: metric.UnitCount,
		Kind: metric.Sum,
		DataPoints: []metric.DataPoint[float64]{
			{
				StartTime: time.Now(),
				EndTime:   time.Now(),
				Value:     1.0,
			},
		},
	}
	collector.CollectMetric("namespace1", metric1)

	metric2 := metric.Metric[float64]{
		Name: "metric2",
		Unit: metric.UnitCount,
		Kind: metric.Sum,
		DataPoints: []metric.DataPoint[float64]{
			{
				StartTime: time.Now(),
				EndTime:   time.Now(),
				Value:     1.0,
			},
		},
	}
	collector.CollectMetric("namespace2", metric2)

	metricCollectorMock.AssertNumberOfCalls(suite.T(), "CollectMetric", 2)
	metricCollectorMock.AssertCalled(suite.T(), "CollectMetric", "namespace1", metric1)
	metricCollectorMock.AssertCalled(suite.T(), "CollectMetric", "namespace2", metric2)
}

func (suite *collectorTestSuite) TestAddRemoveExporter() {
	metricCollectorMock := mocks.NewSlowMetricsCollectorMock()
	logCollectorMock := mocks.NewLogCollectorMock()

	c, err := NewCollector(suite.ctx, time.Minute, 5*time.Minute)
	assert.NoError(suite.T(), err)

	collector := c.(*collectorT)
	collector.metricCollector = metricCollectorMock
	collector.logCollector = logCollectorMock

	exporterMock := exporterMocks.NewExporterMock()

	collector.AddExporter(exporterMock)

	assert.Len(suite.T(), collector.exporters, 1)
	assert.Equal(suite.T(), exporterMock, collector.exporters[0])

	collector.RemoveExporter(exporterMock)

	assert.Len(suite.T(), collector.exporters, 0)
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

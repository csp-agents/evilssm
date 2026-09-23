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
package hybrid

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/telemetry/collector/internal/mocks"
	dynamicconfiguration "github.com/aws/amazon-ssm-agent/agent/telemetry/dynamic_configuration"
	"github.com/aws/amazon-ssm-agent/common/telemetry/metric"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type HydringMetricsCollectorTestSuite struct {
	suite.Suite
	ctx       *context.Mock
	collector *hybridMetricCollector
}

func TestHydringMetricsCollectorTestSuite(t *testing.T) {
	suite.Run(t, new(HydringMetricsCollectorTestSuite))
}

// SetupTest makes sure that all the components referenced in the test case are initialized
// before each test
func (suite *HydringMetricsCollectorTestSuite) SetupTest() {
	suite.ctx = context.NewMockDefault()

	collector, err := NewHybridMetricCollector(suite.ctx, "metrics", 10)
	assert.NoError(suite.T(), err)

	suite.collector = collector
}

func (suite *HydringMetricsCollectorTestSuite) TestHybridMetricCollectorWriteToDisk() {
	suite.collector.diskWriteSchedulerJob.Quit <- true

	inMemoryCollectorMock := mocks.NewFastMetricsCollectorMock()
	diskCollectorMock := mocks.NewSlowMetricsCollectorMock()

	suite.collector.inMemoryCollector = inMemoryCollectorMock
	suite.collector.onDiskCollector = diskCollectorMock

	resultList := metric.NamespaceMetrics[float64]{
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

	inMemoryCollectorMock.On("FetchAllAndDrop").Return(func() (metric.NamespaceMetrics[float64], error) {
		return resultList, nil
	})

	diskCollectorMock.On("CollectMetric", mock.Anything, mock.Anything).Return(nil)

	inMemoryCollectorMock.On("Clean").Return(nil)

	suite.collector.writeToDisk()

	inMemoryCollectorMock.AssertNumberOfCalls(suite.T(), "FetchAllAndDrop", 1)

	diskCollectorMock.AssertNumberOfCalls(suite.T(), "CollectMetric", 4)
}

func (suite *HydringMetricsCollectorTestSuite) TestHybridMetricCollectorWritesPeriodically() {
	inMemoryCollectorMock := mocks.NewFastMetricsCollectorMock()
	diskCollectorMock := mocks.NewSlowMetricsCollectorMock()

	suite.collector.inMemoryCollector = inMemoryCollectorMock
	suite.collector.onDiskCollector = diskCollectorMock

	resultList := metric.NamespaceMetrics[float64]{
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
				{
					StartTime: time.Now(),
					EndTime:   time.Now(),
					Value:     3.0,
				},
			},
		}},
	}

	inMemoryCollectorMock.On("FetchAllAndDrop").Return(func() (metric.NamespaceMetrics[float64], error) {
		return resultList, nil
	})

	diskCollectorMock.On("CollectMetric", mock.Anything, mock.Anything).Return(nil)

	// skip the scheduler wait
	suite.collector.diskWriteSchedulerJob.SkipWait <- true

	assert.EventuallyWithT(suite.T(), func(c *assert.CollectT) {
		ct := NewCommonT(c)

		inMemoryCollectorMock.AssertNumberOfCalls(ct, "FetchAllAndDrop", 1)
		diskCollectorMock.AssertNumberOfCalls(ct, "CollectMetric", 1)
	}, 20*time.Second, 50*time.Millisecond)
}

func (suite *HydringMetricsCollectorTestSuite) TestHybridMetricCollectorPanicRecovered() {
	inMemoryCollectorMock := mocks.NewFastMetricsCollectorMock()
	diskCollectorMock := mocks.NewSlowMetricsCollectorMock()

	suite.collector.inMemoryCollector = inMemoryCollectorMock
	suite.collector.onDiskCollector = diskCollectorMock

	inMemoryCollectorMock.On("FetchAllAndDrop").Panic("error fetching metrics")

	// skip the scheduler wait
	suite.collector.diskWriteSchedulerJob.SkipWait <- true

	logMock := suite.ctx.Log().(*log.Mock)

	assert.EventuallyWithT(suite.T(), func(c *assert.CollectT) {
		ct := NewCommonT(c)

		logMock.AssertCalled(ct, "Errorf", mock.MatchedBy(func(msg string) bool {
			return strings.Contains(msg, "Metric disk write panic")
		}), mock.Anything)

		inMemoryCollectorMock.AssertNumberOfCalls(ct, "FetchAllAndDrop", 1)
		inMemoryCollectorMock.AssertNumberOfCalls(ct, "Clean", 0)

		diskCollectorMock.AssertNumberOfCalls(ct, "CollectMetric", 0)
	}, 20*time.Second, 50*time.Millisecond)
}

func (suite *HydringMetricsCollectorTestSuite) TestHybridMetricCollectorFlush() {
	// create a hybrid metric collector
	dynamicconfiguration.MaxRolls = func(string) int { return 10 }
	dynamicconfiguration.MaxRollSize = func(string) int64 { return 1024 * 1024 }

	defer func() {
		dynamicconfiguration.MaxRolls = dynamicconfiguration.GetMaxRolls
		dynamicconfiguration.MaxRollSize = dynamicconfiguration.GetMaxRollSize
	}()
	collector, err := NewHybridMetricCollector(context.NewMockDefault(), "metrics", 10)
	assert.NoError(suite.T(), err)

	// mock the in memory collector
	inMemoryCollectorMock := mocks.NewFastMetricsCollectorMock()
	collector.inMemoryCollector = inMemoryCollectorMock

	// mock the on disk collector
	diskCollectorMock := mocks.NewSlowMetricsCollectorMock()
	collector.onDiskCollector = diskCollectorMock

	resultErr := errors.New("test error")
	diskCollectorMock.On("Flush").Return(resultErr)

	inMemoryCollectorMock.On("FetchAllAndDrop").Return(func() (metric.NamespaceMetrics[float64], error) {
		return metric.NamespaceMetrics[float64]{}, nil
	}).Once()

	// call the flush method
	err = collector.Flush()
	assert.Equal(suite.T(), resultErr, err)

	// Assert writeToDisk was called
	inMemoryCollectorMock.AssertNumberOfCalls(suite.T(), "FetchAllAndDrop", 1)
	diskCollectorMock.AssertNumberOfCalls(suite.T(), "CollectMetric", 0)

	// assert the flush method was called
	diskCollectorMock.AssertCalled(suite.T(), "Flush")
}

func (suite *HydringMetricsCollectorTestSuite) TestHybridMetricCollectorClose() {
	// mock the in memory collector
	inMemoryCollectorMock := mocks.NewFastMetricsCollectorMock()
	suite.collector.inMemoryCollector = inMemoryCollectorMock

	// mock the on disk collector
	diskCollectorMock := mocks.NewSlowMetricsCollectorMock()
	suite.collector.onDiskCollector = diskCollectorMock

	resultErr1 := errors.New("test error 1")
	inMemoryCollectorMock.On("Close").Return(resultErr1)

	resultErr2 := errors.New("test error 2")
	diskCollectorMock.On("Close").Return(resultErr2)

	// call the Close method
	err := suite.collector.Close()
	assert.Equal(suite.T(), errors.Join(resultErr1, resultErr2), err)
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

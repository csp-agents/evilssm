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

package mocks

import (
	"github.com/aws/amazon-ssm-agent/common/telemetry/metric"

	"github.com/stretchr/testify/mock"
)

type SlowMetricsCollectorMock struct {
	mock.Mock
}

func NewSlowMetricsCollectorMock() *SlowMetricsCollectorMock {
	c := new(SlowMetricsCollectorMock)
	return c
}

func (m *SlowMetricsCollectorMock) Flush() error {
	ret := m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (m *SlowMetricsCollectorMock) CollectMetric(namespace string, metric metric.Metric[float64]) error {
	ret := m.Called(namespace, metric)

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (m *SlowMetricsCollectorMock) FetchAndDrop(limit int) (metric.NamespaceMetrics[float64], error) {
	ret := m.Called(limit)

	var r0 metric.NamespaceMetrics[float64]
	var r1 error
	if rf, ok := ret.Get(0).(func() (metric.NamespaceMetrics[float64], error)); ok {
		r0, r1 = rf()
	} else {
		r0 = ret.Get(0).(metric.NamespaceMetrics[float64])
		r1 = ret.Get(1).(error)
	}
	return r0, r1
}

func (m *SlowMetricsCollectorMock) Clean() error {
	ret := m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (m *SlowMetricsCollectorMock) Close() error {
	ret := m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type FastMetricsCollectorMock struct {
	mock.Mock
}

func NewFastMetricsCollectorMock() *FastMetricsCollectorMock {
	c := new(FastMetricsCollectorMock)
	return c
}

func (m *FastMetricsCollectorMock) Flush() error {
	ret := m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (m *FastMetricsCollectorMock) CollectMetric(namespace string, metric metric.Metric[float64]) error {
	ret := m.Called(namespace, metric)

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (m *FastMetricsCollectorMock) FetchAndDrop(limit int) (metric.NamespaceMetrics[float64], error) {
	ret := m.Called(limit)

	var r0 metric.NamespaceMetrics[float64]
	var r1 error
	if rf, ok := ret.Get(0).(func() (metric.NamespaceMetrics[float64], error)); ok {
		r0, r1 = rf()
	} else {
		r0 = ret.Get(0).(metric.NamespaceMetrics[float64])
		r1 = ret.Get(1).(error)
	}
	return r0, r1
}

func (m *FastMetricsCollectorMock) FetchAllAndDrop() (metric.NamespaceMetrics[float64], error) {
	ret := m.Called()

	var r0 metric.NamespaceMetrics[float64]
	var r1 error
	rf := ret.Get(0).(func() (metric.NamespaceMetrics[float64], error))

	r0, r1 = rf()
	return r0, r1
}

func (m *FastMetricsCollectorMock) Clean() error {
	ret := m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (m *FastMetricsCollectorMock) Close() error {
	ret := m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

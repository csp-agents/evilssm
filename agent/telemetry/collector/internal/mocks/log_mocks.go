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
	"github.com/aws/amazon-ssm-agent/common/telemetry/telemetrylog"

	"github.com/stretchr/testify/mock"
)

type LogCollectorMock struct {
	mock.Mock
}

func NewLogCollectorMock() *LogCollectorMock {
	c := new(LogCollectorMock)
	return c
}

func (m *LogCollectorMock) Flush() error {
	ret := m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (m *LogCollectorMock) CollectLog(namespace string, log telemetrylog.Entry) error {
	ret := m.Called(namespace, log)

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (m *LogCollectorMock) FetchAndDrop(limit int) (telemetrylog.NamespaceLogs, error) {
	ret := m.Called(limit)

	var r0 telemetrylog.NamespaceLogs
	var r1 error
	if rf, ok := ret.Get(0).(func() (telemetrylog.NamespaceLogs, error)); ok {
		r0, r1 = rf()
	} else {
		r0 = ret.Get(0).(telemetrylog.NamespaceLogs)
		r1 = ret.Get(1).(error)
	}

	return r0, r1
}

func (m *LogCollectorMock) Close() error {
	ret := m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

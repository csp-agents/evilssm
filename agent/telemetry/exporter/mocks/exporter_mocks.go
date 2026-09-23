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
	"github.com/aws/amazon-ssm-agent/common/telemetry/telemetrylog"

	"github.com/stretchr/testify/mock"
)

type ExporterMock struct {
	mock.Mock
}

func NewExporterMock() *ExporterMock {
	c := new(ExporterMock)
	return c
}

func (m *ExporterMock) Export(namespace string, metrics []metric.Metric[float64], logs []telemetrylog.Entry) error {
	ret := m.Called(namespace, metrics, logs)

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

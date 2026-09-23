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
package telemetrylog

import (
	"github.com/aws/amazon-ssm-agent/common/telemetry/telemetrylog"

	"github.com/stretchr/testify/mock"
)

type Mock struct {
	mock.Mock
}

// NewMockDefault returns an instance of Mock with default expectations set.
func NewMockDefault() *Mock {
	log := new(Mock)
	return log
}

func (m *Mock) EmitLog(s telemetrylog.Severity, v ...interface{}) error {
	args := m.Called(s, v)
	return args.Error(0)
}

func (m *Mock) EmitLogf(s telemetrylog.Severity, format string, params ...interface{}) error {
	args := m.Called(s, format, params)
	return args.Error(0)
}

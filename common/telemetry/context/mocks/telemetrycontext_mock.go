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
package context

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"

	"github.com/stretchr/testify/mock"
)

type Mock struct {
	mock.Mock
}

// NewMockDefault returns an instance of Mock with default expectations set.
func NewMockDefault() *Mock {
	ctx := new(Mock)
	log := logmocks.NewMockLog()
	agentIdentity := identityMocks.NewDefaultMockAgentIdentity()
	ctx.On("ChannelName").Return("testTelemetry")
	ctx.On("Log").Return(log)
	ctx.On("Identity").Return(agentIdentity)
	return ctx
}

// ChannelName mocks the ChannelName function.
func (m *Mock) ChannelName() string {
	args := m.Called()
	return args.Get(0).(string)
}

// Identity mocks the Identity function.
func (m *Mock) Identity() identity.IAgentIdentity {
	args := m.Called()
	return args.Get(0).(identity.IAgentIdentity)
}

// Log mocks the Log function.
func (m *Mock) Log() log.T {
	args := m.Called()
	return args.Get(0).(log.T)
}

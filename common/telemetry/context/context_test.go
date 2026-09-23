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
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	logMock "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	identityMock "github.com/aws/amazon-ssm-agent/common/identity/mocks"

	"github.com/stretchr/testify/assert"
)

func TestNewTelemetryContext(t *testing.T) {
	// Setup
	mockLog := logMock.NewEmptyLogMock()
	mockIdentity := identityMock.NewDefaultMockAgentIdentity()
	channelName := "test-channel"

	tests := []struct {
		name        string
		channelName string
		log         log.T
		identity    identity.IAgentIdentity
	}{
		{
			name:        "Valid creation with all parameters",
			channelName: channelName,
			log:         mockLog,
			identity:    mockIdentity,
		},
		{
			name:        "Empty channel name",
			channelName: "",
			log:         mockLog,
			identity:    mockIdentity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute
			context := NewTelemetryContext(tt.channelName, tt.log, tt.identity)

			// Verify
			assert.NotNil(t, context)
			assert.Equal(t, tt.channelName+"_telemetry", context.ChannelName())
			assert.Equal(t, tt.log, context.Log())
			assert.Equal(t, tt.identity, context.Identity())
		})
	}
}

func TestTelemetryContext_ChannelName(t *testing.T) {
	// Setup
	expectedChannelName := "test-channel"
	context := &telemetryContext{
		channelName: expectedChannelName + "_telemetry",
	}

	// Execute
	result := context.ChannelName()

	// Verify
	assert.Equal(t, expectedChannelName+"_telemetry", result)
}

func TestTelemetryContext_Log(t *testing.T) {
	// Setup
	mockLog := logMock.NewEmptyLogMock()
	context := &telemetryContext{
		log: mockLog,
	}

	// Execute
	result := context.Log()

	// Verify
	assert.Equal(t, mockLog, result)
}

func TestTelemetryContext_Identity(t *testing.T) {
	// Setup
	mockIdentity := identityMock.NewDefaultMockAgentIdentity()
	context := &telemetryContext{
		identity: mockIdentity,
	}

	// Execute
	result := context.Identity()

	// Verify
	assert.Equal(t, mockIdentity, result)
}

func TestTelemetryContext_NilValues(t *testing.T) {
	// Setup & Execute
	context := NewTelemetryContext("", nil, nil)

	// Verify
	assert.NotNil(t, context)
	assert.Equal(t, "_telemetry", context.ChannelName())
	assert.Nil(t, context.Log())
	assert.Nil(t, context.Identity())
}

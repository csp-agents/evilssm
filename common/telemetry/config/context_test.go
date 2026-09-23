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
package config

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	identitymocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/stretchr/testify/assert"
)

func TestIsTelemetryEnabledIsoRegion(t *testing.T) {
	tests := []struct {
		region   string
		expected bool
	}{
		{"us-east-1", true},
		{"us-east-2", true},
		{"us-west-1", true},
		{"us-west-2", true},
		{"ap-southeast-1", true},
		{"ap-northeast-1", true},
		{"sa-east-1", true},
		{"us-iso-east-1", false},
		{"us-iso-west-1", false},
		{"us-isob-east-1", false},
		{"cn-north-1", false},
		{"cn-northwest-1", false},
		{"us-iso-east-1", false},
		{"us-iso-west-1", false},
		{"us-isob-east-1", false},
	}
	ctx := context.NewMockDefaultWithConfig(appconfig.SsmagentConfig{
		Agent: appconfig.AgentInfo{
			GlobalEnhancedTelemetryEnabled: true,
		},
	})
	agentIdentity := ctx.Identity().(*identitymocks.IAgentIdentity)

	for _, tt := range tests {
		t.Run(tt.region, func(t *testing.T) {

			agentIdentity.ExpectedCalls = nil
			agentIdentity.On("Region").Return(tt.region, nil)

			result := IsTelemetryEnabled(ctx.Log(), agentIdentity, ctx.AppConfig())

			assert.Equal(t, tt.expected, result, "Region: %s should have telemetry enabled=%v", tt.region, tt.expected)
		})
	}
}

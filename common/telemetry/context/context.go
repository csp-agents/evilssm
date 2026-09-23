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
	"github.com/aws/amazon-ssm-agent/common/identity"
)

type TelemetryContext interface {
	ChannelName() string
	Log() log.T
	Identity() identity.IAgentIdentity
}

type telemetryContext struct {
	channelName string
	log         log.T
	identity    identity.IAgentIdentity
}

func NewTelemetryContext(channelName string, log log.T, identity identity.IAgentIdentity) TelemetryContext {
	return &telemetryContext{
		channelName: channelName + "_telemetry",
		log:         log,
		identity:    identity,
	}
}

// Identity implements TelemetryContext.
func (t *telemetryContext) Identity() identity.IAgentIdentity {
	return t.identity
}

// Log implements TelemetryContext.
func (t *telemetryContext) Log() log.T {
	return t.log
}

// MainChannelName implements TelemetryContext.
func (t *telemetryContext) ChannelName() string {
	return t.channelName
}

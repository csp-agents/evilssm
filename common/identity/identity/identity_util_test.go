// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package identity

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/customidentity"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/onprem"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/stretchr/testify/assert"
)

func TestIsOnPremInstance(t *testing.T) {
	logger := log.NewMockLog()
	appConfig := &appconfig.SsmagentConfig{}
	agentIdentity := &agentIdentityCacher{
		client: newECSIdentity(logger, appConfig)[0],
	}
	assert.False(t, IsOnPremInstance(agentIdentity))

	agentIdentity = &agentIdentityCacher{
		client: newOnPremIdentity(logger, appConfig)[0],
	}
	assert.True(t, IsOnPremInstance(agentIdentity))
}

func TestIsEC2Instance(t *testing.T) {

	logger := log.NewMockLog()
	appConfig := &appconfig.SsmagentConfig{}

	assert.False(t, IsEC2Instance(nil))

	agentIdentity := &agentIdentityCacher{
		client: newOnPremIdentity(logger, appConfig)[0],
	}
	assert.False(t, IsEC2Instance(agentIdentity))

	agentIdentity = &agentIdentityCacher{
		client: newECSIdentity(logger, appConfig)[0],
	}
	assert.False(t, IsEC2Instance(agentIdentity))
}

func TestGetCredentialsRefresherIdentity(t *testing.T) {
	cacher := &agentIdentityCacher{
		client: &ec2.Identity{},
	}

	// Ec2 identity implements credentials refresher identity
	_, isCredsRefresher := GetRemoteProvider(cacher)
	assert.True(t, isCredsRefresher)

	// Verify onprem identity implements credentials refresher identity
	cacher.client = onprem.NewOnPremIdentity(log.NewMockLog(), &appconfig.SsmagentConfig{})
	_, isCredsRefresher = GetRemoteProvider(cacher)
	assert.True(t, isCredsRefresher)
}

func TestGetMetadataIdentity_SuccessfulRetrieval(t *testing.T) {
	// Create a mock metadata identity

	type mockIdentity struct {
		identityMocks.IAgentIdentityInner
		identityMocks.IMetadataIdentity
	}
	mockMetadataIdentity := &mockIdentity{}

	// Test successful retrieval of metadata identity
	result, ok := GetMetadataIdentity(mockMetadataIdentity)

	assert.True(t, ok, "Expected metadata identity to be successfully retrieved")
	assert.NotNil(t, result, "Retrieved metadata identity should not be nil")
}

func TestGetMetadataIdentity_NilInput(t *testing.T) {
	// Test with nil input
	result, ok := GetMetadataIdentity(nil)

	assert.False(t, ok, "Expected metadata identity retrieval to fail with nil input")
	assert.Nil(t, result, "Retrieved metadata identity should be nil")
}

func TestGetRemoteProviderNilIdentity(t *testing.T) {
	provider, ok := GetRemoteProvider(nil)
	assert.Nil(t, provider)
	assert.False(t, ok)
}

func TestNewCustomIdentity_MultipleValidIdentities(t *testing.T) {
	// Test creating multiple valid custom identities
	logger := log.NewMockLog()
	config := &appconfig.SsmagentConfig{
		Identity: appconfig.IdentityCfg{
			CustomIdentities: []*appconfig.CustomIdentity{
				{
					InstanceID: "instance1",
					Region:     "us-east-1",
				},
				{
					InstanceID: "instance2",
					Region:     "us-west-2",
				},
			},
		},
	}

	// Call the function
	identities := newCustomIdentity(logger, config)

	// Verify expectations
	assert.Len(t, identities, 2, "Should create two custom identities")

	// Verify individual identity details
	for _, identity := range identities {
		customIdentity, ok := identity.(*customidentity.Identity)
		assert.True(t, ok, "Should be a custom identity")
		assert.NotEmpty(t, customIdentity.CustomIdentity.InstanceID)
		assert.NotEmpty(t, customIdentity.CustomIdentity.Region)
	}
}

func TestNewCustomIdentity_EmptyRegion(t *testing.T) {
	// Test creating multiple valid custom identities
	logger := log.NewMockLog()
	config := &appconfig.SsmagentConfig{
		Identity: appconfig.IdentityCfg{
			CustomIdentities: []*appconfig.CustomIdentity{
				{
					InstanceID: "instance1",
					Region:     "",
				},
			},
		},
	}

	// Call the function
	logger.On("Warnf",
		"The Region provided as part of CustomIdentity cannot be empty. Skipping custom identity #%d",
		0).Once()

	// Call the function
	identities := newCustomIdentity(logger, config)

	// Verify expectations
	assert.Len(t, identities, 0)
}

func TestNewCustomIdentity_EmptyInstance(t *testing.T) {
	// Test creating multiple valid custom identities
	logger := log.NewMockLog()
	config := &appconfig.SsmagentConfig{
		Identity: appconfig.IdentityCfg{
			CustomIdentities: []*appconfig.CustomIdentity{
				{
					InstanceID: "",
					Region:     "us-west-2",
				},
			},
		},
	}

	// Call the function
	logger.On("Warnf",
		"The InstanceID provided as part of CustomIdentity cannot be empty. Skipping custom identity #%d",
		0).Once()

	// Call the function
	identities := newCustomIdentity(logger, config)

	// Verify expectations
	assert.Len(t, identities, 0)
}

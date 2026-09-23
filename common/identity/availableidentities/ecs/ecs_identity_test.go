// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package ecs

import (
	"net/http"
	"testing"

	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/assert"
)

func TestFetchClusterNameAndTaskIdSuccessFoo(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	metadataResponseOnce = func(client *http.Client, endpoint string, respType string) ([]byte, error) {
		return []byte("{\"Cluster\": \"clusterName\", \"DockerId\": \"1234567890\"," +
			"\"TaskARN\": \"arn:aws:ecs:us-east-2:012345678910:task/9781c248-0edd-4cdb-9a93-f63cb662a5d3\", \"AvailabilityZone\": \"mckedZone\"}"), nil
	}
	getMetadataEndpoint = func() (string, error) {
		return fakeV3Endpoint, nil
	}

	identityLocal := NewECSIdentity(mockLog)
	instanceID, err := identityLocal.InstanceID()
	assert.Nil(t, err)
	assert.Equal(t, instanceID, "ecs:clusterName_9781c248-0edd-4cdb-9a93-f63cb662a5d3_1234567890")

	region, err := identityLocal.Region()
	assert.Nil(t, err)
	assert.Equal(t, region, "us-east-2")

	availabilityZone, err := identityLocal.AvailabilityZone()
	assert.Nil(t, err)
	assert.Equal(t, availabilityZone, "mckedZone")

	availabilityZoneId, err := identityLocal.AvailabilityZoneId()
	assert.Nil(t, err)
	assert.Equal(t, availabilityZoneId, "")

	instanceType, err := identityLocal.InstanceType()
	assert.Nil(t, err)
	assert.Equal(t, instanceType, ecsInstanceType)

	identityType := identityLocal.IdentityType()
	assert.Equal(t, identityType, IdentityType)

	isIdentityEnv := identityLocal.IsIdentityEnvironment()
	assert.NotNil(t, isIdentityEnv)
	assert.Equal(t, isIdentityEnv, true)
}

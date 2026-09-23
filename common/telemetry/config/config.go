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
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/aws-sdk-go/aws/endpoints"
)

func IsTelemetryEnabled(log log.T, identity identity.IAgentIdentity, appConfig appconfig.SsmagentConfig) bool {
	region, err := identity.Region()
	if err != nil {
		log.Warnf("Could not determine region")
		return false
	}
	partition, ok := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), region)
	if !ok {
		log.Warnf("Could not determine partition for region %s", region)
		return false
	}
	log.Debugf("Determined partition for region %s: %s", region, partition.ID())

	if !appConfig.Agent.GlobalEnhancedTelemetryEnabled {
		log.Info("Agent GlobalEnhancedTelemetry is disabled")
		return false
	}

	return partition.ID() == endpoints.AwsPartitionID
}

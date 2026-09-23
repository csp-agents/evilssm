// Copyright 2022 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package ec2detector implements the detection of EC2 using specific sub-detectors
package ec2detector

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
)

type Detector interface {
	// IsEc2 returns true if detector detects attributes indicating it is an ec2 instance
	// and any errors it encountered while checking the status
	IsEc2(log log.T) (status bool, errCodes []string)
	// GetName returns the name of the detector
	GetName() string
}

type Ec2Detector interface {
	// IsEC2Instance returns true if any sub detector detects it is running on EC2
	// and any errors it encountered while checking the status
	IsEC2Instance() (status bool, errCodes []string)
}

type ec2Detector struct {
	log log.T
}

var detectors []Detector

func (e *ec2Detector) IsEC2Instance() (bool, []string) {
	var errCodes []string
	for _, detector := range detectors {
		if status, errs := detector.IsEc2(e.log); status {
			return true, nil
		} else if len(errs) > 0 {
			errCodes = append(errCodes, errs...)
		}
	}

	return false, errCodes
}

func RegisterDetector(detector Detector) {
	detectors = append(detectors, detector)
}

// New returns a struct implementing the EC2Detector interface to detect if we are running on ec2
func New(log log.T) *ec2Detector {
	return &ec2Detector{log: log}
}

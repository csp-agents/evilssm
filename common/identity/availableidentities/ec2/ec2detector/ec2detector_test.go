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

package ec2detector

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/ec2detector/mocks"
	"github.com/stretchr/testify/assert"
)

func TestIsEC2Instance(t *testing.T) {
	logMock := log.NewMockLog()
	detector := ec2Detector{log: logMock}
	trueSubDetector := &mocks.Detector{}
	falseSubDetector := &mocks.Detector{}
	temp := detectors
	defer func() { detectors = temp }()

	trueSubDetector.On("IsEc2", logMock).Return(true, nil).Once()
	detectors = []Detector{trueSubDetector}
	status, errCodes := detector.IsEC2Instance()
	assert.True(t, status)
	assert.Nil(t, errCodes)

	trueSubDetector.On("IsEc2", logMock).Return(true, nil).Once()
	detectors = []Detector{trueSubDetector, falseSubDetector}
	status, errCodes = detector.IsEC2Instance()
	assert.True(t, status)
	assert.Nil(t, errCodes)

	falseSubDetector.On("IsEc2", logMock).Return(false, []string{"NVE", "NUE"}).Once()
	trueSubDetector.On("IsEc2", logMock).Return(true, nil).Once()
	detectors = []Detector{falseSubDetector, trueSubDetector}
	status, errCodes = detector.IsEC2Instance()
	assert.True(t, status)
	assert.Nil(t, errCodes)

	falseSubDetector.On("IsEc2", logMock).Return(false, []string{"NVE", "NUE"}).Once()
	detectors = []Detector{falseSubDetector}
	status, errCodes = detector.IsEC2Instance()
	assert.False(t, status)
	assert.Equal(t, len(errCodes), 2)

	falseSubDetector.On("IsEc2", logMock).Return(false, nil).Once()
	detectors = []Detector{falseSubDetector}
	status, errCodes = detector.IsEC2Instance()
	assert.False(t, status)
	assert.Nil(t, errCodes)

	trueSubDetector.AssertExpectations(t)
	falseSubDetector.AssertExpectations(t)
}

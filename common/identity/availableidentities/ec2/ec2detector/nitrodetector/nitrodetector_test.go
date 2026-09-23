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

//go:build !darwin
// +build !darwin

package nitrodetector

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	logger "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/ec2detector/helper"
	"github.com/stretchr/testify/assert"
)

func TestIsEc2(t *testing.T) {
	detector := New("", "")
	logMock := logger.NewMockLog()
	tempMatchUuid := helper.MatchUuid
	tempGetVendor := getVendor
	tempGetUuid := getUuid
	resetFunc := func() {
		helper.MatchUuid = tempMatchUuid
		getVendor = tempGetVendor
		getUuid = tempGetUuid
	}

	getVendor = func(log.T, string, string) (string, string) { return "", "NVE" }
	getUuid = func(log.T, string, string) (string, string) { return "", "NUE" }
	status, errCodes := detector.IsEc2(logMock)
	resetFunc()
	assert.False(t, status)
	assert.Equal(t, len(errCodes), 2)

	getVendor = func(log.T, string, string) (string, string) { return expectedNitroVendor, "" }
	getUuid = func(log.T, string, string) (string, string) { return "", "NUFNF" }
	status, errCodes = detector.IsEc2(logMock)
	resetFunc()
	assert.False(t, status)
	assert.Equal(t, len(errCodes), 1)

	getVendor = func(log.T, string, string) (string, string) { return expectedNitroVendor, "" }
	getUuid = func(log.T, string, string) (string, string) { return "", "" }
	status, errCodes = detector.IsEc2(logMock)
	resetFunc()
	assert.False(t, status)
	assert.Equal(t, len(errCodes), 0)

	getVendor = func(log.T, string, string) (string, string) { return "someothervendor", "" }
	getUuid = func(log.T, string, string) (string, string) { return "123", "" }
	helper.MatchUuid = func(log.T, string) bool { return false }
	status, errCodes = detector.IsEc2(logMock)
	resetFunc()
	assert.False(t, status)
	assert.Equal(t, len(errCodes), 0)

	getVendor = func(log.T, string, string) (string, string) {
		return fmt.Sprintf("%s%s", expectedNitroVendor, "-somepostfix"), ""
	}
	getUuid = func(log.T, string, string) (string, string) { return "", "" }
	status, errCodes = detector.IsEc2(logMock)
	resetFunc()
	assert.False(t, status)
	assert.Equal(t, len(errCodes), 0)

	getVendor = func(log.T, string, string) (string, string) {
		return fmt.Sprintf("%s%s", "someprefix-", expectedNitroVendor), ""
	}
	getUuid = func(log.T, string, string) (string, string) { return "", "" }
	status, errCodes = detector.IsEc2(logMock)
	resetFunc()
	assert.False(t, status)
	assert.Equal(t, len(errCodes), 0)

	getVendor = func(log.T, string, string) (string, string) { return expectedNitroVendor, "" }
	getUuid = func(log.T, string, string) (string, string) { return "123", "" }
	helper.MatchUuid = func(log.T, string) bool { return true }
	status, errCodes = detector.IsEc2(logMock)
	resetFunc()
	assert.True(t, status)
	assert.Equal(t, len(errCodes), 0)

	getVendor = func(log.T, string, string) (string, string) { return strings.ToUpper(expectedNitroVendor), "" }
	getUuid = func(log.T, string, string) (string, string) { return "123", "" }
	helper.MatchUuid = func(log.T, string) bool { return true }
	status, errCodes = detector.IsEc2(logMock)
	resetFunc()
	assert.True(t, status)
	assert.Equal(t, len(errCodes), 0)
}

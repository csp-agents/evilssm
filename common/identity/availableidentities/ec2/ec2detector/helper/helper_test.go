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

package helper

import (
	"strings"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	logger "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/assert"
)

func TestUuidMatcher(t *testing.T) {
	logMock := logger.NewMockLog()
	temp := GetHostInfo
	defer func() { GetHostInfo = temp }()
	// big endian formats
	testUpperLower(t, logMock, true, "EC2FFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")

	testUpperLower(t, logMock, false, "2ECFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, logMock, false, "E2CFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, logMock, false, "C2EFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, logMock, false, "CE2FFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, logMock, false, "2CEFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")

	testUpperLower(t, logMock, false, "FC2FFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, logMock, false, "EF2FFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, logMock, false, "ECFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")

	// little endian formats
	testUpperLower(t, logMock, true, "FFFF2FEC-FFFF-FFFF-FFFF-FFFFFFFFFFFF")

	testUpperLower(t, logMock, false, "FFFF2FCE-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, logMock, false, "FFFFCF2E-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, logMock, false, "FFFFCFE2-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, logMock, false, "FFFFEFC2-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, logMock, false, "FFFFEF2C-FFFF-FFFF-FFFF-FFFFFFFFFFFF")

	testUpperLower(t, logMock, false, "FFFFFFEC-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, logMock, false, "FFFF2FEF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, logMock, false, "FFFF2FFC-FFFF-FFFF-FFFF-FFFFFFFFFFFF")

	// both
	testUpperLower(t, logMock, true, "EC2F2FEC-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
}

func testUpperLower(t *testing.T, log log.T, expected bool, testCase string) {
	assert.Equal(t, expected, MatchUuid(log, testCase))
	assert.Equal(t, expected, MatchUuid(log, strings.ToLower(testCase)))
	assert.Equal(t, expected, MatchUuid(log, strings.ToUpper(testCase)))
}

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

//go:build freebsd || linux || netbsd || openbsd || windows
// +build freebsd linux netbsd openbsd windows

package testcases

import (
	"errors"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	testCommon "github.com/aws/amazon-ssm-agent/agent/update/tester/common"
	"github.com/aws/amazon-ssm-agent/common/identity/identity"
)

const (
	failedToGetVendorAndVersion = "failed to get vendor and version"
	failedToGetUuid             = "failed to get uuid"
	failedQuerySystemHostInfo   = "failed to query system host info"
)

// ShouldRunTest determines if test should run
func (l *Ec2DetectorTestCase) ShouldRunTest() bool {
	return identity.IsEC2Instance(l.context.Identity())
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// cleanBiosString casts string to lower case and trims spaces from string
func cleanBiosString(val string) string {
	return strings.TrimSpace(strings.ToLower(val))
}

func matchUuid(uuid string) bool {
	return bigEndianEc2UuidRegex.MatchString(uuid) || littleEndianEc2UuidRegex.MatchString(uuid)
}

func matchNitroEc2(info HostInfo) bool {
	return info.Vendor == nitroVendorValue
}

func matchXenEc2(info HostInfo) bool {
	return strings.HasSuffix(info.Version, xenVersionSuffix)
}

func isEc2Instance(info HostInfo) bool {
	return matchUuid(info.Uuid) && (matchXenEc2(info) || matchNitroEc2(info))
}

func getSmbiosHostInfo(log log.T) (hostInfo HostInfo, err error) {
	hostInfo = HostInfo{}
	if hostInfo.Vendor, err = platform.GetSmbiosInfo(log, platform.SmbiosVendorParamKey); err != nil {
		return
	}
	if hostInfo.Version, err = platform.GetSmbiosInfo(log, platform.SmbiosVersionParamKey); err != nil {
		return
	}
	if hostInfo.Uuid, err = platform.GetSmbiosInfo(log, platform.SmbiosUuidParamKey); err != nil {
		return
	}

	if hostInfo.Version == "" && hostInfo.Vendor == "" {
		return hostInfo, errors.New(failedToGetVendorAndVersion)
	}
	if hostInfo.Uuid == "" {
		return hostInfo, errors.New(failedToGetUuid)
	}

	return
}

func (l *Ec2DetectorTestCase) generateHostInfoResult(info HostInfo, queryErr error, approach approachType) (bool, string) {
	testPass := false
	detectedHypervisor := unknown
	errDP := errNotSet

	if queryErr == nil {
		testPass = isEc2Instance(info)
		if testPass {
			if matchNitroEc2(info) {
				detectedHypervisor = nitro
			} else {
				detectedHypervisor = amazonXen
			}
		}
	} else {
		if errors.Is(queryErr, platform.ErrOpenSmbiosStream) {
			errDP = errFailedOpenStream
		} else if errors.Is(queryErr, platform.ErrDecodeSmbiosStream) {
			errDP = errFailedDecodeStream
		} else if strings.Contains(queryErr.Error(), failedQuerySystemHostInfo) {
			errDP = errFailedQuerySystemHostInfo
		} else if strings.Contains(queryErr.Error(), failedToGetVendorAndVersion) {
			errDP = errFailedGetVendorAndVersion
		} else if strings.Contains(queryErr.Error(), failedToGetUuid) {
			errDP = errFailedGetUuid
		} else {
			errDP = errUnknown
		}
	}

	return testPass, fmt.Sprintf("%sh%s_%se%d", approach, detectedHypervisor, approach, errDP)
}

func (l *Ec2DetectorTestCase) generateTestOutput() testCommon.TestOutput {
	isPrimarySuccess, primaryAdditionalInfo := l.generateHostInfoResult(l.primaryInfo, l.primaryErr, primary)
	isSecondarySuccess, secondaryAdditionalInfo := l.generateHostInfoResult(l.secondaryInfo, l.secondaryErr, secondary)

	result := testCommon.TestCaseFail
	if isPrimarySuccess || isSecondarySuccess {
		result = testCommon.TestCasePass
	}

	return testCommon.TestOutput{
		Result:         result,
		AdditionalInfo: fmt.Sprintf("_p%d_s%d_%s_%s", btoi(isPrimarySuccess), btoi(isSecondarySuccess), primaryAdditionalInfo, secondaryAdditionalInfo),
	}
}

// ExecuteTestCase executes the ec2 detector test case, test only runs when instance id starts with i-
func (l *Ec2DetectorTestCase) ExecuteTestCase() (output testCommon.TestOutput) {
	defer func() {
		if err := recover(); err != nil {
			l.context.Log().Warnf("test panic: %v", err)
			l.context.Log().Warnf("Stacktrace:\n%s", debug.Stack())

			output = l.generateTestOutput()
			output.Result = testCommon.TestCaseFail
			output.AdditionalInfo += "_panic"
		}
	}()

	l.queryHostInfo()
	return l.generateTestOutput()
}

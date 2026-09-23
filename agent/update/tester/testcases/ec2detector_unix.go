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
// permissions and limitations under the License

//go:build freebsd || linux || netbsd || openbsd
// +build freebsd linux netbsd openbsd

package testcases

import (
	"errors"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
)

const ec2DetectorTestCaseName = "UnixEc2Detector"

func getSystemHostInfo(log log.T) (HostInfo, error) {
	var hostInfo HostInfo

	vendor, _ := platform.GetSystemInfo(log, platform.NitroVendorSystemInfoParamKey)
	hostInfo.Vendor = cleanBiosString(vendor)
	version, _ := platform.GetSystemInfo(log, platform.XenVersionSystemInfoParamKey)
	hostInfo.Version = cleanBiosString(version)

	if hostInfo.Version == "" && hostInfo.Vendor == "" {
		return hostInfo, errors.New(failedToGetVendorAndVersion)
	}

	uuid, _ := platform.GetSystemInfo(log, platform.XenUuidSystemInfoParamKey)
	hostInfo.Uuid = cleanBiosString(uuid)
	if hostInfo.Uuid == "" {
		uuid, _ = platform.GetSystemInfo(log, platform.NitroUuidSystemInfoParamKey)
		hostInfo.Uuid = cleanBiosString(uuid)
	}

	if hostInfo.Uuid == "" {
		return hostInfo, errors.New(failedToGetUuid)
	}

	return hostInfo, nil
}

func (l *Ec2DetectorTestCase) queryHostInfo() {
	l.primaryInfo, l.primaryErr = getSystemHostInfo(l.context.Log())
	l.secondaryInfo, l.secondaryErr = getSmbiosHostInfo(l.context.Log())
}

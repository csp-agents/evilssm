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

//go:build windows
// +build windows

package testcases

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
)

const ec2DetectorTestCaseName = "WinEc2Detector"

func getSystemHostInfo(log log.T) (HostInfo, error) {
	info := HostInfo{}
	if version, err := platform.GetSystemInfo(log, platform.BiosVersionParamKey); err == nil {
		info.Version = cleanBiosString(version)
	} else {
		return info, fmt.Errorf("%s: %v", failedQuerySystemHostInfo, err)
	}
	if uuid, err := platform.GetSystemInfo(log, platform.BiosSerialNumberParamKey); err == nil {
		info.Uuid = cleanBiosString(uuid)
	} else {
		return info, fmt.Errorf("%s: %v", failedQuerySystemHostInfo, err)
	}
	if vendor, err := platform.GetSystemInfo(log, platform.BiosManufacturerParamKey); err == nil {
		info.Vendor = cleanBiosString(vendor)
	} else {
		return info, fmt.Errorf("%s: %v", failedQuerySystemHostInfo, err)
	}

	if info.Version == "" && info.Vendor == "" {
		return info, fmt.Errorf("%s", failedToGetVendorAndVersion)
	}

	if info.Uuid == "" {
		return info, fmt.Errorf("%s", failedToGetUuid)
	}

	return info, nil
}

func (l *Ec2DetectorTestCase) queryHostInfo() {
	l.primaryInfo, l.primaryErr = getSmbiosHostInfo(l.context.Log())
	l.secondaryInfo, l.secondaryErr = getSystemHostInfo(l.context.Log())
}

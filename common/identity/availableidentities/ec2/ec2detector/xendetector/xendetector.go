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

// Package xendetector implements logic to determine if we are running on an amazon Xen hypervisor
package xendetector

import (
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/ec2detector/helper"
)

const (
	expectedVersionSuffix = ".amazon"
	Name                  = "Xen"
)

var getVendor, getUuid = helper.GetHostInfo, helper.GetHostInfo

type xenDetector struct {
	uuidParamKey    string
	versionParamKey string
}

func (d *xenDetector) IsEc2(log log.T) (bool, []string) {
	var errCodes []string
	versionInfo, versionErrCode := getVendor(log, d.versionParamKey, "XV")
	if versionErrCode != "" {
		errCodes = append(errCodes, versionErrCode)
	}
	uuidInfo, uuidErrCode := getUuid(log, d.uuidParamKey, "XU")
	if uuidErrCode != "" {
		errCodes = append(errCodes, uuidErrCode)
	}

	if len(errCodes) > 0 {
		return false, errCodes
	} else {
		return strings.HasSuffix(strings.ToLower(versionInfo), expectedVersionSuffix) && helper.MatchUuid(log, uuidInfo), nil
	}
}

func (d *xenDetector) GetName() string {
	return Name
}

func New(uuidParamKey, versionParamKey string) *xenDetector {
	return &xenDetector{
		uuidParamKey:    uuidParamKey,
		versionParamKey: versionParamKey,
	}
}

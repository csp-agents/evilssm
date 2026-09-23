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

// Package nitrodetector implements logic to determine if we are running on an nitro hypervisor
package nitrodetector

import (
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/ec2detector/helper"
)

const (
	expectedNitroVendor = "amazon ec2"
	Name                = "Nitro"
)

type nitroDetector struct {
	uuidParamKey   string
	vendorParamKey string
}

var getVendor, getUuid = helper.GetHostInfo, helper.GetHostInfo

func (d *nitroDetector) IsEc2(log log.T) (bool, []string) {
	var errCodes []string
	vendorInfo, vendorErrCode := getVendor(log, d.vendorParamKey, "NV")
	if vendorErrCode != "" {
		errCodes = append(errCodes, vendorErrCode)
	}
	uuidInfo, uuidErrCode := getUuid(log, d.uuidParamKey, "NU")
	if uuidErrCode != "" {
		errCodes = append(errCodes, uuidErrCode)
	}

	if len(errCodes) > 0 {
		return false, errCodes
	} else {
		return strings.ToLower(vendorInfo) == expectedNitroVendor && helper.MatchUuid(log, uuidInfo), nil
	}
}

func (d *nitroDetector) GetName() string {
	return Name
}

func New(uuidParamKey, vendorParamKey string) *nitroDetector {
	return &nitroDetector{
		uuidParamKey:   uuidParamKey,
		vendorParamKey: vendorParamKey,
	}
}

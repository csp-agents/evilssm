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
	"regexp"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	bigEndianEc2UuidRegex    = "^ec2[0-9a-f]{5}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$"
	littleEndianEc2UuidRegex = "^[0-9a-f]{4}2[0-9a-f]ec(-[0-9a-f]{4}){3}-[0-9a-f]{12}$"
)

var MatchUuid = func(log log.T, uuidInfo string) bool {
	uuid := strings.ToLower(uuidInfo)
	isBigEndianEc2Uuid := regexp.MustCompile(bigEndianEc2UuidRegex).MatchString(uuid)
	isLittleEndianEc2Uuid := regexp.MustCompile(littleEndianEc2UuidRegex).MatchString(uuid)

	return isBigEndianEc2Uuid || isLittleEndianEc2Uuid
}

// Copyright 2025 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build windows
// +build windows

package helper

import (
	"errors"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
)

var GetHostInfo = func(log log.T, paramKey, errCodePrefix string) (string, string) {
	errCodePrefix = errCodePrefix + "-"
	paramValue, err := platform.GetSmbiosInfo(log, paramKey)
	if errCode := checkError(err); errCode != "" {
		return "", errCodePrefix + errCode
	} else if strings.TrimSpace(paramValue) == "" {
		return "", errCodePrefix + "E"
	} else {
		return paramValue, ""
	}
}

func checkError(err error) string {
	if err != nil {
		if errors.Is(err, platform.ErrOpenSmbiosStream) {
			return "OSS"
		} else if errors.Is(err, platform.ErrDecodeSmbiosStream) {
			return "DSS"
		} else {
			return "SBG"
		}
	} else {
		return ""
	}
}

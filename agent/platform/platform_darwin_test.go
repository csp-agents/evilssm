// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build darwin
// +build darwin

// Package platform contains platform specific utilities.
package platform

import (
	"fmt"
	"testing"

	logger "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/assert"
)

func TestPlatformVersionEmptyString(t *testing.T) {
	ClearCache()
	tmpFunc := execWithTimeout
	execWithTimeout = func(string, ...string) ([]byte, error) {
		return []byte(" \t"), nil
	}
	defer func() { execWithTimeout = tmpFunc }()

	platformVersion, err := PlatformVersion(logger.NewMockLog())
	assert.Equal(t, notAvailableMessage, platformVersion)
	assert.NotNil(t, err)
}

func TestPlatformVersion(t *testing.T) {
	ClearCache()
	tmpFunc := execWithTimeout
	execWithTimeout = func(string, ...string) ([]byte, error) {
		return []byte("ProductVersion:\t15.1\ntestingsomething\n"), nil
	}
	defer func() { execWithTimeout = tmpFunc }()

	logObj := logger.NewMockLog()
	platformVersion, err := PlatformVersion(logObj)
	assert.Equal(t, "15.1", platformVersion)
	assert.Nil(t, err)
}

func TestPlatformVersionWithError(t *testing.T) {
	ClearCache()
	tmpFunc := execWithTimeout
	execWithTimeout = func(string, ...string) ([]byte, error) {
		return []byte(""), fmt.Errorf("platform version error")
	}
	defer func() { execWithTimeout = tmpFunc }()

	logObj := logger.NewMockLog()
	platformVersion, err := PlatformVersion(logObj)
	assert.Equal(t, notAvailableMessage, platformVersion)
	assert.NotNil(t, err)
}

func TestPlatformName(t *testing.T) {
	ClearCache()
	tmpFunc := execWithTimeout
	execWithTimeout = func(string, ...string) ([]byte, error) {
		return []byte("ProductName:\tmacOS\nProductVersion:\t10.15.7\nBuildVersion:\t19H524\n"), nil
	}
	defer func() { execWithTimeout = tmpFunc }()

	logObj := logger.NewMockLog()
	platformName, err := PlatformName(logObj)
	assert.Equal(t, "macOS", platformName)
	assert.Nil(t, err)
}

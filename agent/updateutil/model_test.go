// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package updateutil contains updater specific utilities.
package updateutil

import (
	"io/fs"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/assert"
)

func TestLoadUpdatePluginResult(t *testing.T) {
	str1 := `{"StandOut": "John", "StartDateTime": "2025-04-12T10:00:00Z"}`
	logger = log.NewMockLog()
	ioUtilRead = func(filePath string) ([]byte, error) {
		return []byte(str1), nil
	}
	res, err := LoadUpdatePluginResult(logger, "foo")
	assert.Nil(t, err)
	assert.Equal(t, "John", res.StandOut)
}

func TestSaveUpdatePluginResult(t *testing.T) {
	util := Utility{}
	jsonTest := UpdatePluginResult{
		StandOut:      "John",
		StartDateTime: time.Now(),
	}
	logger = log.NewMockLog()
	ioUtilWrite = func(filename string, data []byte, perm fs.FileMode) error {
		return nil
	}
	err := util.SaveUpdatePluginResult(logger, "foo", &jsonTest)
	assert.Nil(t, err)
}

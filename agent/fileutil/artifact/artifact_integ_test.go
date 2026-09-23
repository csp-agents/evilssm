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

//go:build e2e
// +build e2e

package artifact

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type DownloadTest struct {
	input          DownloadInput
	expectedOutput DownloadOutput
}

var (
	localPathExist, _    = filepath.Abs(filepath.Join(".", "testdata", "CheckMyHash.txt"))
	localPathNotExist, _ = filepath.Abs(filepath.Join(".", "testdata", "IDontExist.txt"))
	downloadFolder, _    = filepath.Abs(filepath.Join(".", "testdata"))
	mockContext          = context.NewMockDefault()
	mockLog              = mockContext.Log()

	downloadTests = []DownloadTest{
		// {DownloadInput{SourceUrl, DestinationDirectory, SourceChecksums, ExpectedBucketOwner},
		// DownloadOutput{LocalFilePath, IsUpdated, IsHashMatched}},
		{
			// validate sha256
			DownloadInput{
				localPathExist,
				downloadFolder,
				map[string]string{
					"sha256": "090c1965e46155b2b23ba9093ed7c67243957a397e3ad5531a693d57958a760a",
				},
				""},
			DownloadOutput{
				localPathExist,
				false,
				true},
		},
		{
			// validate incorrect sha256 fails
			DownloadInput{
				localPathExist,
				downloadFolder,
				map[string]string{
					"sha256": "111111111",
				},
				""},
			DownloadOutput{
				localPathExist,
				false,
				false},
		},
		{
			// validate md5
			DownloadInput{
				localPathExist,
				downloadFolder,
				map[string]string{
					"md5": "e84913ff3a8eef39238b32170e657ba8",
				},
				""},
			DownloadOutput{
				localPathExist,
				false,
				true},
		},

		{
			// validate incorrect md5 fails
			DownloadInput{
				localPathExist,
				downloadFolder,
				map[string]string{
					"md5": "222222222",
				},
				""},
			DownloadOutput{
				localPathExist,
				false,
				false},
		},
		{
			// ensure default is sha256
			DownloadInput{
				localPathExist,
				downloadFolder,
				map[string]string{
					"": "090c1965e46155b2b23ba9093ed7c67243957a397e3ad5531a693d57958a760a",
				},
				""},
			DownloadOutput{
				localPathExist,
				false,
				true},
		},
		{
			// relative url is not supported
			DownloadInput{
				"IamRelativeFilePath",
				downloadFolder,
				map[string]string{
					"": "090c1965e46155b2b23ba9093ed7c67243957a397e3ad5531a693d57958a760a",
				},
				""},
			DownloadOutput{
				"",
				false,
				false},
		},
		{
			// relative url is not supported
			DownloadInput{
				"IamRelativeFilePath/IdontExist",
				downloadFolder,
				map[string]string{
					"": "090c1965e46155b2b23ba9093ed7c67243957a397e3ad5531a693d57958a760a",
				},
				""},
			DownloadOutput{
				"",
				false,
				false},
		},
		{
			// s3 download error
			DownloadInput{
				"https://s3.amazonaws.com/ssmnotsuchbucket/ssmnosuchfile.txt",
				downloadFolder,
				map[string]string{
					"": "",
				},
				""},
			DownloadOutput{
				"",
				false,
				false},
		},
		{
			// ensure empty map is valid
			DownloadInput{
				localPathExist,
				downloadFolder,
				map[string]string{},
				"",
			},
			DownloadOutput{
				localPathExist,
				false,
				true},
		},
		{
			// ensure empty value is valid; this is important for the agent updater itself
			DownloadInput{
				localPathExist,
				downloadFolder,
				map[string]string{"sha256": ""},
				"",
			},
			DownloadOutput{
				localPathExist,
				false,
				true},
		},
		{
			// first checksum fails, the second one succeeds
			DownloadInput{
				localPathExist,
				downloadFolder,
				map[string]string{
					"md5":    "111111111",
					"sha256": "090c1965e46155b2b23ba9093ed7c67243957a397e3ad5531a693d57958a760a",
				},
				"",
			},
			DownloadOutput{
				localPathExist,
				false,
				false},
		},
		{
			// none of the provided algorithms are supported
			DownloadInput{
				localPathExist,
				downloadFolder,
				map[string]string{
					"sha512": "111111111",
					"sha1":   "090c1965e46155b2b23ba9093ed7c67243957a397e3ad5531a693d57958a760a",
				},
				"",
			},
			DownloadOutput{
				localPathExist,
				false,
				false},
		},
		{
			// one supported algorithm and one not supported
			DownloadInput{
				localPathExist,
				downloadFolder,
				map[string]string{
					"foo":    "123456789",
					"sha256": "090c1965e46155b2b23ba9093ed7c67243957a397e3ad5531a693d57958a760a",
				},
				"",
			},
			DownloadOutput{
				localPathExist,
				false,
				true},
		},
	}
)

func runDownloadTests(t *testing.T, tests []DownloadTest) {
	for _, test := range tests {
		output, err := Download(mockContext, test.input)
		t.Log(err)
		assert.Equal(t, test.expectedOutput, output)
	}
}

func TestDownloads(t *testing.T) {
	x, y := os.Getwd()
	mockLog.Infof("Working Directory is %v, %v", x, y)
	runDownloadTests(t, downloadTests)
}

func TestHttpHttpsFallbackDownloadArtifact(t *testing.T) {

	// Configure fake s3 endpoint to block s3 download
	testConfig := appconfig.DefaultConfig()
	testConfig.S3.Endpoint = "fakes3.amazonaws.com"
	testContext := context.NewMockDefaultWithConfig(testConfig)

	// Capture log.Info() messages
	var logMessages []string
	testLogger := log.NewMockLog()
	testLogger.On("Info", mock.Anything).Unset()
	testLogger.On("Info", mock.AnythingOfType("[]interface {}")).Run(func(args mock.Arguments) {
		logMessages = append(logMessages, fmt.Sprint(args.Get(0).([]interface{})...))
	}).Return()

	testContext.On("Log").Unset()
	testContext.On("Log").Return(testLogger)

	testFilePath := "https://amazon-ssm-us-east-1.s3.amazonaws.com/3.3.40.0/VERSION"
	downloadInput := DownloadInput{
		DestinationDirectory: ".",
		SourceURL:            testFilePath,
		SourceChecksums: map[string]string{
			"sha256": "0c0f36c238e6c4c00f39d94dc6381930df2851db0ea2e2543d931474ddce1f8f",
		},
	}
	var expectedLocalPath = "b9f961391ec1ae061db3afcbed5571b2463139c8"
	os.Remove(expectedLocalPath)
	os.Remove(expectedLocalPath + ".etag")
	expectedOutput := DownloadOutput{
		expectedLocalPath,
		true,
		true}

	defer func() {
		os.Remove(expectedLocalPath)
		os.Remove(expectedLocalPath + ".etag")
	}()

	output, err := Download(testContext, downloadInput)

	assert.NoError(t, err, "Failed to download %v", downloadInput)
	testContext.Log().Infof("Download Result is %v and err:%v", output, err)
	assert.Equal(t, expectedOutput, output)
	assert.Contains(t, logMessages,
		"An error occurred when attempting s3 download. Attempting http/https download as fallback.",
		"http/https fallback is not triggered")

	// now since we have downloaded the file, try to download again should result in cache hit!
	expectedOutput = DownloadOutput{
		expectedLocalPath,
		false,
		true}
	output, err = Download(testContext, downloadInput)
	assert.NoError(t, err, "Failed to download %v", downloadInput)
	testContext.Log().Infof("Download Result is %v and err:%v", output, err)
	assert.Equal(t, expectedOutput, output)
}

func TestDualStackHttpHttpsFallbackDownloadArtifact(t *testing.T) {

	// Configure fake s3 endpoint to block s3 download
	testConfig := appconfig.DefaultConfig()
	testConfig.S3.Endpoint = "fakes3.us-east-1.amazonaws.com"
	testConfig.Agent.UseDualStackEndpoint = true
	testContext := context.NewMockDefaultWithConfig(testConfig)

	// Capture log.Debugf() messages
	var logMessages []string
	testLogger := log.NewMockLog()
	testLogger.On("Debugf", mock.Anything, mock.Anything).Unset()
	testLogger.On("Debugf", mock.AnythingOfType("string"), mock.AnythingOfType("[]interface {}")).Run(func(args mock.Arguments) {
		logMessages = append(logMessages, fmt.Sprintf(args.String(0), args.Get(1).([]interface{})...))
	}).Return()

	testContext.On("Log").Unset()
	testContext.On("Log").Return(testLogger)

	testFilePath := "https://amazon-ssm-us-east-1.s3.us-east-1.amazonaws.com/3.3.40.0/VERSION"
	downloadInput := DownloadInput{
		DestinationDirectory: ".",
		SourceURL:            testFilePath,
		SourceChecksums: map[string]string{
			"sha256": "0c0f36c238e6c4c00f39d94dc6381930df2851db0ea2e2543d931474ddce1f8f",
		},
	}
	var expectedLocalPath = "aa31c8581b6f7aef5122f819ef479a5939ace0fa"
	os.Remove(expectedLocalPath)
	os.Remove(expectedLocalPath + ".etag")
	expectedOutput := DownloadOutput{
		expectedLocalPath,
		true,
		true}

	defer func() {
		os.Remove(expectedLocalPath)
		os.Remove(expectedLocalPath + ".etag")
	}()

	output, err := Download(testContext, downloadInput)

	assert.NoError(t, err, "Failed to download %v", downloadInput)
	testContext.Log().Infof("Download Result is %v and err:%v", output, err)
	assert.Equal(t, expectedOutput, output)
	assert.Contains(t, logMessages,
		"attempting to download as http/https download from https://amazon-ssm-us-east-1.s3.dualstack.us-east-1.amazonaws.com/3.3.40.0/VERSION to aa31c8581b6f7aef5122f819ef479a5939ace0fa",
		"Download url should be converted to dual-stack")

	// now since we have downloaded the file, try to download again should result in cache hit!
	expectedOutput = DownloadOutput{
		expectedLocalPath,
		false,
		true}
	output, err = Download(testContext, downloadInput)
	assert.NoError(t, err, "Failed to download %v", downloadInput)
	testContext.Log().Infof("Download Result is %v and err:%v", output, err)
	assert.Equal(t, expectedOutput, output)
}

func TestDownloadUsingHttpInvalidUrl(t *testing.T) {
	downloadInput := DownloadInput{
		DestinationDirectory: ".",
		SourceURL:            "xyz@amazon.com",
		SourceChecksums: map[string]string{
			"sha256": "0c0f36c238e6c4c00f39d94dc6381930df2851db0ea2e2543d931474ddce1f8f",
		},
	}
	output, err := DownloadUsingHttp(mockContext, downloadInput)
	assert.ErrorContains(t, err, "unsupported protocol scheme")
	assert.Nil(t, output)
}

func TestDownloadUsingHttp(t *testing.T) {
	testFilePath := "https://amazon-ssm-us-east-1.s3.amazonaws.com/3.3.40.0/VERSION"
	downloadInput := DownloadInput{
		DestinationDirectory: ".",
		SourceURL:            testFilePath,
		SourceChecksums: map[string]string{
			"sha256": "0c0f36c238e6c4c00f39d94dc6381930df2851db0ea2e2543d931474ddce1f8f",
		},
	}
	var expectedLocalPath = "b9f961391ec1ae061db3afcbed5571b2463139c8"
	os.Remove(expectedLocalPath)
	os.Remove(expectedLocalPath + ".etag")
	expectedOutput := DownloadOutput{
		expectedLocalPath,
		true,
		true}

	output, err := DownloadUsingHttp(mockContext, downloadInput)
	assert.NoError(t, err, "Failed to download %v", downloadInput)
	mockLog.Infof("Download Result is %v and err:%v", output, err)

	defer func() {
		os.Remove(expectedLocalPath)
		os.Remove(expectedLocalPath + ".etag")
	}()
	assert.Equal(t, expectedOutput, *output)

	// now since we have downloaded the file, try to download again should result in cache hit!
	expectedOutput = DownloadOutput{
		expectedLocalPath,
		false,
		true}
	output, err = DownloadUsingHttp(mockContext, downloadInput)
	assert.NoError(t, err, "Failed to download %v", downloadInput)
	mockLog.Infof("Download Result is %v and err:%v", output, err)
	assert.Equal(t, expectedOutput, *output)
}

func TestDownloadUsingHttpMismatchingHash(t *testing.T) {
	testFilePath := "https://amazon-ssm-us-east-1.s3.amazonaws.com/3.3.40.0/VERSION"
	downloadInput := DownloadInput{
		DestinationDirectory: ".",
		SourceURL:            testFilePath,
		SourceChecksums: map[string]string{
			"sha256": "invalidhash",
		},
	}
	var expectedLocalPath = "b9f961391ec1ae061db3afcbed5571b2463139c8"
	os.Remove(expectedLocalPath)
	os.Remove(expectedLocalPath + ".etag")

	output, err := DownloadUsingHttp(mockContext, downloadInput)
	assert.ErrorContains(t, err, "failed to verify hash of downloadinput")
	mockLog.Infof("Download Result is %v and err:%v", output, err)

	defer func() {
		os.Remove(expectedLocalPath)
		os.Remove(expectedLocalPath + ".etag")
	}()
	assert.Nil(t, output)
}

func ExampleMd5HashValue() {
	path := filepath.Join("testdata", "CheckMyHash.txt")
	mockLog := log.NewMockLog()
	content, _ := Md5HashValue(mockLog, path)
	fmt.Println(content)
}

func ExampleSha256HashValue() {
	path := filepath.Join("testdata", "CheckMyHash.txt")
	mockLog := log.NewMockLog()
	content, _ := Sha256HashValue(mockLog, path)
	fmt.Println(content)
}

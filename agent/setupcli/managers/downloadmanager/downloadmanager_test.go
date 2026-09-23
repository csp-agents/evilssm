// Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package downloadmanager helps us with file download related functions in ssm-setup-cli
package downloadmanager

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updatemanifest"
	updatemanifestmocks "github.com/aws/amazon-ssm-agent/agent/updateutil/updatemanifest/mocks"

	"github.com/aws/amazon-ssm-agent/agent/log"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	updateinfomocks "github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// Define ConfigManager TestSuite struct
type DownloadManagerTestSuite struct {
	suite.Suite
	logMock *logmocks.Mock
}

// Initialize the ConfigManagerTestSuite test suite struct
func (suite *DownloadManagerTestSuite) SetupTest() {
	hasLowerKernelVersionFunc = func() bool {
		return false
	}
	logMock := logmocks.NewMockLog()
	suite.logMock = logMock
}

func (suite *DownloadManagerTestSuite) TestDownloadManager_GetStableVersion_Success() {
	path := "path1"
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		return destinationPath, nil
	}
	updateManifestNew = func(context context.T, info updateinfo.T, region string) updatemanifest.T {
		updateManifestMock := &updatemanifestmocks.T{}
		updateManifestMock.On("LoadManifest", path).Return(nil).Once()
		return updateManifestMock
	}
	downloadMgr := New(suite.logMock, "us-east-1", "https://s3.amazonaws.com/"+updateconstants.ManifestFile, nil, "path1", true, false)
	versionUrl := ""
	expectedVersionNumber := "3.2.1377.0"
	expectedStableVersionURL := "https://s3.amazonaws.com/stable/VERSION"
	fileUtilityReadContent = func(stableVersionUrl string, client *http.Client) ([]byte, error) {
		versionUrl = stableVersionUrl
		return []byte(expectedVersionNumber), nil
	}
	versionNum, err := downloadMgr.GetStableVersion()
	assert.Equal(suite.T(), expectedVersionNumber, versionNum, "mismatched version number")
	assert.Nil(suite.T(), err, "unexpected error")
	assert.Equal(suite.T(), expectedStableVersionURL, versionUrl, "mismatched version URL")
	downloadMgr = New(suite.logMock, "us-east-1", "https://s3.amazonaws.com/"+updateconstants.ManifestFile+" ", nil, path, true, false)

	versionUrl = ""
	expectedVersionNumber = "3.2.1377.0"
	expectedStableVersionURL = "https://s3.amazonaws.com/stable/VERSION"
	fileUtilityReadContent = func(stableVersionUrl string, client *http.Client) ([]byte, error) {
		versionUrl = stableVersionUrl
		return []byte(expectedVersionNumber), nil
	}
	versionNum, err = downloadMgr.GetStableVersion()
	assert.Equal(suite.T(), expectedVersionNumber, versionNum, "mismatched version number")
	assert.Nil(suite.T(), err, "unexpected error")
	assert.Equal(suite.T(), expectedStableVersionURL, versionUrl, "mismatched version URL")

	downloadMgr = New(suite.logMock, "us-east-1", "", nil, "path1", true, false)
	versionUrl = ""
	expectedVersionNumber = "3.2.1377.0"
	expectedStableVersionURL = "https://s3.us-east-1.amazonaws.com/amazon-ssm-us-east-1/stable/VERSION"
	fileUtilityReadContent = func(stableVersionUrl string, client *http.Client) ([]byte, error) {
		versionUrl = stableVersionUrl
		return []byte(expectedVersionNumber), nil
	}
	versionNum, err = downloadMgr.GetStableVersion()
	assert.Equal(suite.T(), expectedVersionNumber, versionNum, "mismatched version number")
	assert.Nil(suite.T(), err, "unexpected error")
	assert.Equal(suite.T(), expectedStableVersionURL, versionUrl, "mismatched version URL")
}

func (suite *DownloadManagerTestSuite) TestDownloadManager_GetStableVersion_Failure() {
	path := "path1"
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		return destinationPath, nil
	}
	updateManifestNew = func(context context.T, info updateinfo.T, region string) updatemanifest.T {
		updateManifestMock := &updatemanifestmocks.T{}
		updateManifestMock.On("LoadManifest", path).Return(nil).Once()
		return updateManifestMock
	}
	downloadMgr := New(suite.logMock, "us-east-1", "https://s3.amazonaws.com/"+updateconstants.ManifestFile, nil, "path1", true, false)
	versionUrl := ""
	expectedStableVersionURL := "https://s3.amazonaws.com/stable/VERSION"
	fileUtilityReadContent = func(stableVersionUrl string, client *http.Client) ([]byte, error) {
		versionUrl = stableVersionUrl
		return []byte("3.d.32.2" + " "), nil
	}
	versionNum, err := downloadMgr.GetStableVersion()
	assert.Equal(suite.T(), "", versionNum, "mismatched version number")
	assert.NotNil(suite.T(), err, "should throw error")
	assert.Equal(suite.T(), expectedStableVersionURL, versionUrl, "mismatched version URL")

	fileUtilityReadContent = func(stableVersionUrl string, client *http.Client) ([]byte, error) {
		return nil, nil
	}
	_, err = downloadMgr.GetStableVersion()
	assert.NotNil(suite.T(), err, "should throw error")
}

func (suite *DownloadManagerTestSuite) TestDownloadManager_GetLatestVersion_Success() {
	path := "path1"
	expectedVersionNumber := "3.2.1377.0"
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		return destinationPath, nil
	}

	updateManifestNew = func(context context.T, info updateinfo.T, region string) updatemanifest.T {
		updateManifestMock := &updatemanifestmocks.T{}
		updateManifestMock.On("LoadManifest", path).Return(nil).Once()
		updateManifestMock.On("GetLatestActiveVersion", appconfig.DefaultAgentName).Return(expectedVersionNumber, nil).Once()
		return updateManifestMock
	}
	downloadMgr := New(suite.logMock, "us-east-1", "https://s3.amazonaws.com/"+updateconstants.ManifestFile, nil, path, true, false)
	versionNum, err := downloadMgr.GetLatestVersion()
	assert.Equal(suite.T(), expectedVersionNumber, versionNum, "mismatched version number")
	assert.Nil(suite.T(), err, "unexpected error")
}

func (suite *DownloadManagerTestSuite) TestDownloadManager_GetLatestVersion_Failure() {
	path := "path1"
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		return destinationPath, nil
	}
	updateManifestNew = func(context context.T, info updateinfo.T, region string) updatemanifest.T {
		updateManifestMock := &updatemanifestmocks.T{}
		updateManifestMock.On("LoadManifest", path).Return(nil).Once()
		updateManifestMock.On("GetLatestActiveVersion", appconfig.DefaultAgentName).Return("", fmt.Errorf("err1")).Once()
		return updateManifestMock
	}
	downloadMgr := New(suite.logMock, "us-east-1", "https://s3.amazonaws.com/"+updateconstants.ManifestFile, nil, "path1", true, false)
	versionNum, err := downloadMgr.GetLatestVersion()
	assert.Equal(suite.T(), "", versionNum, "mismatched version number")
	assert.NotNil(suite.T(), err, "should throw error")

	fileUtilityReadContent = func(stableVersionUrl string, client *http.Client) ([]byte, error) {
		return nil, nil
	}
	_, err = downloadMgr.GetStableVersion()
	assert.NotNil(suite.T(), err, "should throw error")
}

func (suite *DownloadManagerTestSuite) TestDownloadManager_DownloadLatestSSMSetupCLI_Success() {
	info := &updateinfomocks.T{}
	info.On("GenerateCompressedFileName", appconfig.DefaultAgentName).Return("linux_amd64").Once()
	info.On("GeneratePlatformBasedFolderName").Return("linux_amd64").Once()
	path := "path1"
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		return destinationPath, nil
	}
	updateManifestNew = func(context context.T, info updateinfo.T, region string) updatemanifest.T {
		updateManifestMock := &updatemanifestmocks.T{}
		updateManifestMock.On("LoadManifest", path).Return(nil).Once()
		return updateManifestMock
	}
	downloadMgr := New(suite.logMock, "us-east-1", "", info, "path1", true, false)
	actualSSMSetupCLIURL := ""
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		actualSSMSetupCLIURL = fileURL
		return "temp2", nil
	}
	checkSum := "23232"
	computeAgentChecksumFunc = func(agentFilePath string) (hash string, err error) {
		return checkSum, nil
	}
	expectedLatestSSMSetupCLIURL := "https://s3.us-east-1.amazonaws.com/amazon-ssm-us-east-1/latest/linux_amd64/ssm-setup-cli"
	err := downloadMgr.DownloadLatestSSMSetupCLI("temp1", checkSum)

	assert.Nil(suite.T(), err, "should not throw error")
	assert.Contains(suite.T(), actualSSMSetupCLIURL, expectedLatestSSMSetupCLIURL, "mismatched version URL")
}

func (suite *DownloadManagerTestSuite) TestDownloadManager_DownloadLatestSSMSetupCLI_HttpDownloadFailure() {
	info := &updateinfomocks.T{}
	info.On("GenerateCompressedFileName", appconfig.DefaultAgentName).Return("linux_amd64").Once()
	info.On("GeneratePlatformBasedFolderName").Return("linux_amd64").Once()
	path := "path1"
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		return destinationPath, nil
	}
	updateManifestNew = func(context context.T, info updateinfo.T, region string) updatemanifest.T {
		updateManifestMock := &updatemanifestmocks.T{}
		updateManifestMock.On("LoadManifest", path).Return(nil).Once()
		return updateManifestMock
	}
	downloadMgr := New(suite.logMock, "us-east-1", "", info, "path1", true, false)
	actualSSMSetupCLIURL := ""
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		actualSSMSetupCLIURL = fileURL
		return "temp2", fmt.Errorf("test")
	}
	checkSum := "23232"
	notVisited := true
	computeAgentChecksumFunc = func(agentFilePath string) (hash string, err error) {
		notVisited = false
		return checkSum, nil
	}
	expectedLatestSSMSetupCLIURL := "https://s3.us-east-1.amazonaws.com/amazon-ssm-us-east-1/latest/linux_amd64/ssm-setup-cli"
	err := downloadMgr.DownloadLatestSSMSetupCLI("temp1", checkSum)

	assert.Contains(suite.T(), err.Error(), "error while downloading SSM Setup CLI", "should throw error")
	assert.Contains(suite.T(), actualSSMSetupCLIURL, expectedLatestSSMSetupCLIURL, "mismatched version URL")
	assert.True(suite.T(), notVisited)
}

func (suite *DownloadManagerTestSuite) TestDownloadManager_DownloadLatestSSMSetupCLI_CheckSumFailure() {
	info := &updateinfomocks.T{}
	info.On("GenerateCompressedFileName", appconfig.DefaultAgentName).Return("linux_amd64").Once()
	info.On("GeneratePlatformBasedFolderName").Return("linux_amd64").Once()
	path := "path1"
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		return destinationPath, nil
	}
	updateManifestNew = func(context context.T, info updateinfo.T, region string) updatemanifest.T {
		updateManifestMock := &updatemanifestmocks.T{}
		updateManifestMock.On("LoadManifest", path).Return(nil).Once()
		return updateManifestMock
	}
	downloadMgr := New(suite.logMock, "us-east-1", "", info, "path1", true, false)
	actualSSMSetupCLIURL := ""
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		actualSSMSetupCLIURL = fileURL
		return "temp2", nil
	}
	checkSum := "23232"
	computeAgentChecksumFunc = func(agentFilePath string) (hash string, err error) {
		return "sdsds", nil
	}
	expectedLatestSSMSetupCLIURL := "https://s3.us-east-1.amazonaws.com/amazon-ssm-us-east-1/latest/linux_amd64/ssm-setup-cli"
	err := downloadMgr.DownloadLatestSSMSetupCLI("temp1", checkSum)

	assert.Contains(suite.T(), err.Error(), "checksum mismatch with latest ssm-setup-cli", "should throw error")
	assert.Contains(suite.T(), actualSSMSetupCLIURL, expectedLatestSSMSetupCLIURL, "mismatched version URL")
}

func (suite *DownloadManagerTestSuite) TestDownloadManager_DownloadArtifacts_Success() {
	info := &updateinfomocks.T{}
	info.On("GenerateCompressedFileName", appconfig.DefaultAgentName).Return("linux_amd64").Once()
	info.On("GeneratePlatformBasedFolderName").Return("linux_amd64").Once()
	path := "path1"
	tempPath := "temp2"
	version := "3.2.3.5"
	checkSum := "1234"
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		return destinationPath, nil
	}
	expectedLatestSSMSetupCLIURL := "https://s3.us-east-1.amazonaws.com/amazon-ssm-us-east-1/amazon-ssm-agent/3.2.3.5/linux_amd64"

	updateManifestNew = func(context context.T, info updateinfo.T, region string) updatemanifest.T {
		updateManifestMock := &updatemanifestmocks.T{}
		updateManifestMock.On("LoadManifest", path).Return(nil).Once()
		updateManifestMock.On("GetDownloadURLAndHash", appconfig.DefaultAgentName, version).Return(expectedLatestSSMSetupCLIURL, checkSum, nil).Once()
		return updateManifestMock
	}
	downloadMgr := New(suite.logMock, "us-east-1", "", info, path, true, false)
	actualSSMSetupCLIURL := ""

	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		if actualSSMSetupCLIURL == "" {
			actualSSMSetupCLIURL = fileURL
		}
		return tempPath, nil
	}

	computeAgentChecksumFunc = func(agentFilePath string) (hash string, err error) {
		return checkSum, nil
	}
	fileUtilUnCompress = func(log log.T, src, dest string) error {
		return nil
	}
	err := downloadMgr.DownloadArtifacts(version, "manifestURL1", "temp1")
	assert.Nil(suite.T(), err, "should not throw error")
	assert.Equal(suite.T(), expectedLatestSSMSetupCLIURL, actualSSMSetupCLIURL, "mismatched version URL")
}

func (suite *DownloadManagerTestSuite) TestDownloadManager_DownloadArtifacts_DualStack_Success() {
	info := &updateinfomocks.T{}
	info.On("GenerateCompressedFileName", appconfig.DefaultAgentName).Return("linux_amd64").Once()
	info.On("GeneratePlatformBasedFolderName").Return("linux_amd64").Once()
	path := "path1"
	tempPath := "temp2"
	version := "3.2.3.5"
	checkSum := "1234"

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		return destinationPath, nil
	}
	expectedLatestSSMSetupCLIURL := "https://s3.dualstack.us-east-1.amazonaws.com/amazon-ssm-us-east-1/amazon-ssm-agent/3.2.3.5/linux_amd64"
	manifestLatestSSMSetupCLIURL := "https://s3.us-east-1.amazonaws.com/amazon-ssm-us-east-1/amazon-ssm-agent/3.2.3.5/linux_amd64"

	updateManifestNew = func(context context.T, info updateinfo.T, region string) updatemanifest.T {
		updateManifestMock := &updatemanifestmocks.T{}
		updateManifestMock.On("LoadManifest", path).Return(nil).Once()
		updateManifestMock.On("GetDownloadURLAndHash", appconfig.DefaultAgentName, version).Return(manifestLatestSSMSetupCLIURL, checkSum, nil).Once()
		return updateManifestMock
	}

	downloadMgr := New(suite.logMock, "us-east-1", "", info, path, true, true)
	actualSSMSetupCLIURL := ""

	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		if actualSSMSetupCLIURL == "" {
			actualSSMSetupCLIURL = fileURL
		}
		return tempPath, nil
	}

	computeAgentChecksumFunc = func(agentFilePath string) (hash string, err error) {
		return checkSum, nil
	}
	fileUtilUnCompress = func(log log.T, src, dest string) error {
		return nil
	}
	err := downloadMgr.DownloadArtifacts(version, "manifestURL1", "temp1")
	assert.Nil(suite.T(), err, "should not throw error")
	assert.Equal(suite.T(), expectedLatestSSMSetupCLIURL, actualSSMSetupCLIURL, "mismatched version URL")

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)

	assert.NotContains(suite.T(), buf.String(), "Warnf: URL does not match")
}

func (suite *DownloadManagerTestSuite) TestDownloadManager_GetS3BucketUrl_StandardEndpoint() {
	// Test standard endpoint generation
	region := "us-west-2"
	path := "tmp"

	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		return destinationPath, nil
	}

	updateManifestNew = func(context context.T, info updateinfo.T, region string) updatemanifest.T {
		updateManifestMock := &updatemanifestmocks.T{}
		updateManifestMock.On("LoadManifest", path).Return(nil).Once()
		return updateManifestMock
	}

	// Create download manager with dual-stack disabled
	downloadMgr := New(suite.logMock, region, "", nil, path, false, false)
	assert.NotNil(suite.T(), downloadMgr)

	// Cast to access internal method
	dm := downloadMgr.(*downloadManager)

	// Get the S3 bucket URL
	bucketUrl := dm.getS3BucketUrl()

	// Verify it contains standard endpoint
	assert.Equal(suite.T(), bucketUrl, "https://s3.us-west-2.amazonaws.com/amazon-ssm-us-west-2")
	assert.NotContains(suite.T(), bucketUrl, "dualstack")
}

func (suite *DownloadManagerTestSuite) TestDownloadManager_GetS3BucketUrl_DualStackEndpoint() {
	// Test dual-stack endpoint generation
	region := "us-west-2"
	path := "path1"

	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		return destinationPath, nil
	}

	updateManifestNew = func(context context.T, info updateinfo.T, region string) updatemanifest.T {
		updateManifestMock := &updatemanifestmocks.T{}
		updateManifestMock.On("LoadManifest", path).Return(nil).Once()
		return updateManifestMock
	}

	// Create download manager with dual-stack enabled
	downloadMgr := New(suite.logMock, region, "", nil, path, false, true)
	assert.NotNil(suite.T(), downloadMgr)

	// Cast to access internal method
	dm := downloadMgr.(*downloadManager)

	// Get the S3 bucket URL
	bucketUrl := dm.getS3BucketUrl()

	// Verify it contains dual-stack endpoint
	assert.Contains(suite.T(), bucketUrl, "s3.dualstack.us-west-2.amazonaws.com")
	assert.Contains(suite.T(), bucketUrl, "https://")
}

func TestDownloadManagerTestSuite(t *testing.T) {
	suite.Run(t, new(DownloadManagerTestSuite))
}

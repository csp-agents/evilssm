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

// Package processor contains the methods for update ssm agent.
// It also provides methods for sendReply and updateInstanceInfo
package processor

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	contextmocks "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo"
	updateinfomocks "github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo/mocks"
	updatemanifestmocks "github.com/aws/amazon-ssm-agent/agent/updateutil/updatemanifest/mocks"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateprecondition"
	updatepreconditionmocks "github.com/aws/amazon-ssm-agent/agent/updateutil/updateprecondition/mocks"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updates3util"
	updates3utilmocks "github.com/aws/amazon-ssm-agent/agent/updateutil/updates3util/mocks"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type serviceStub struct {
	Service
}

func (s *serviceStub) SendReply(log log.T, update *UpdateDetail) error {
	return nil
}

func (s *serviceStub) DeleteMessage(log log.T, update *UpdateDetail) error {
	return nil
}

func (s *serviceStub) UpdateHealthCheck(log log.T, update *UpdateDetail, errorCode string) error {
	return nil
}

type contextMgrStub struct {
	tempStdOut string
}

func (c *contextMgrStub) uploadOutput(log log.T, updateDetail *UpdateDetail, orchestrationDir string) error {
	return nil
}

func TestStartOrResumeUpdateFromInstalledState(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	isMethodExecuted := false
	updateDetail := createUpdateDetail(Installed)
	// mock the verify method
	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) error {
		isMethodExecuted = true
		return nil
	}
	// action
	updater.StartOrResumeUpdate(logger, updateDetail)
	// assert
	assert.True(t, isMethodExecuted)
}

func TestStartOrResumeUpdateFromInitializedState(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	isMethodExecuted := false
	updateDetail := createUpdateDetail(Initialized)
	// mock the verify method
	updater.mgr.initManifest = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) error {
		isMethodExecuted = true
		return nil
	}
	// action
	updater.StartOrResumeUpdate(logger, updateDetail)
	// assert
	assert.True(t, isMethodExecuted)
}

func TestStartOrResumeUpdateFromStagedState(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	isMethodExecuted := false
	updateDetail := createUpdateDetail(Staged)
	// mock the verify method
	updater.mgr.update = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) error {
		isMethodExecuted = true
		return nil
	}
	// action
	updater.StartOrResumeUpdate(logger, updateDetail)
	// assert
	assert.True(t, isMethodExecuted)
}

func TestInitializeUpdate(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail("")

	// action
	err := updater.InitializeUpdate(logger, updateDetail)

	// assert
	assert.NotEmpty(t, updateDetail.StandardOut)
	assert.NotEmpty(t, updateDetail.StartDateTime)
	assert.NoError(t, err)
}

func TestInitManifest_NoManifestURLNoSource(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Initialized)
	updateDetail.SourceLocation = ""
	updateDetail.ManifestURL = ""

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	// action
	err := initManifest(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, finalizeCalled)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)

	assert.Equal(t, "", updateDetail.SourceLocation)
	assert.Equal(t, "", updateDetail.ManifestURL)
	assert.Contains(t, updateDetail.StandardOut, "Failed to resolve manifest url:")
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestInitManifest_NoManifestURLButSuccess(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	s3Util := &updates3utilmocks.T{}
	s3Util.On("DownloadManifest", mock.Anything, mock.Anything).Return(nil)
	updater.mgr.S3util = s3Util

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.SourceLocation = "https://bucket.s3.region.amazonaws.com/amazon-ssm-agent/version/amazon-ssm-agent.tar.gz"
	updateDetail.ManifestURL = ""

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.initSelfUpdate = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := initManifest(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, called)
	assert.False(t, finalizeCalled)

	assert.Equal(t, Initialized, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusInProgress, updateDetail.Result)

	assert.Equal(t, "https://bucket.s3.region.amazonaws.com/ssm-agent-manifest.json", updateDetail.ManifestURL)
	assert.Equal(t, "", updateDetail.StandardOut)
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestInitManifest_ErrorDownloadManifest(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	s3Util := &updates3utilmocks.T{}
	s3Util.On("DownloadManifest", mock.Anything, mock.Anything).Return(
		&updates3util.UpdateErrorStruct{Error: fmt.Errorf("SomeDownloadError")})
	updater.mgr.S3util = s3Util

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.ManifestURL = "https://bucket.s3.region.amazonaws.com/ssm-agent-manifest.json"

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.initSelfUpdate = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := initManifest(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.False(t, called)
	assert.True(t, finalizeCalled)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)

	assert.Contains(t, updateDetail.StandardOut, "Failed to download manifest: SomeDownloadError")
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestInitSelfUpdate_NoSelfUpdate(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.SelfUpdate = false

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.determineTarget = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := initSelfUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, called)
	assert.False(t, finalizeCalled)
	assert.Equal(t, Initialized, updateDetail.State)

	assert.Equal(t, "", updateDetail.StandardOut)
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestInitSelfUpdate_FailedCheckDeprecated(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.SelfUpdate = true

	manifest := &updatemanifestmocks.T{}
	manifest.On("IsVersionDeprecated", mock.Anything, mock.Anything).Return(false, fmt.Errorf("SomeDeprecationError"))
	updateDetail.Manifest = manifest

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.determineTarget = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := initSelfUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.False(t, called)
	assert.True(t, finalizeCalled)
	assert.Equal(t, Completed, updateDetail.State)

	assert.Contains(t, updateDetail.StandardOut, "Failed to check if version is deprecated: SomeDeprecationError")
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestInitSelfUpdate_NotDeprecated(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.SelfUpdate = true

	manifest := &updatemanifestmocks.T{}
	manifest.On("IsVersionDeprecated", mock.Anything, mock.Anything).Return(false, nil)
	updateDetail.Manifest = manifest

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	// action
	err := initSelfUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, finalizeCalled)

	assert.Equal(t, Initialized, updateDetail.State)

	assert.Equal(t, "", updateDetail.StandardOut)
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestInitSelfUpdate_IsDeprecated_FailedGetLastestActive(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.SelfUpdate = true

	manifest := &updatemanifestmocks.T{}
	manifest.On("IsVersionDeprecated", mock.Anything, mock.Anything).Return(true, nil)
	manifest.On("GetLatestActiveVersion", mock.Anything).Return("", fmt.Errorf("SomeGetLatestError"))
	updateDetail.Manifest = manifest

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	// action
	err := initSelfUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, finalizeCalled)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)

	assert.Contains(t, updateDetail.StandardOut, "Failed to get latest active version from manifest: SomeGetLatestError")
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestInitSelfUpdate_IsDeprecated_Success(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.SelfUpdate = true

	manifest := &updatemanifestmocks.T{}
	manifest.On("IsVersionDeprecated", mock.Anything, mock.Anything).Return(true, nil)
	manifest.On("GetLatestActiveVersion", mock.Anything).Return("5.5.0.0", nil)
	updateDetail.Manifest = manifest

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.determineTarget = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := initSelfUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.False(t, finalizeCalled)
	assert.True(t, called)
	assert.Equal(t, Initialized, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusInProgress, updateDetail.Result)

	assert.Equal(t, "", updateDetail.StandardOut)
	assert.Equal(t, "", updateDetail.StandardError)
	assert.Equal(t, "5.5.0.0", updateDetail.TargetVersion)
	assert.Equal(t, "5.0.0.0", updateDetail.SourceVersion)
	assert.True(t, updateconstants.TargetVersionSelfUpdate == updateDetail.TargetResolver)
}

func TestDetermineTarget_TargetVersionNone_FailedGetLatest(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Initialized)
	updateDetail.TargetVersion = "None"

	manifest := &updatemanifestmocks.T{}
	manifest.On("GetLatestActiveVersion", mock.Anything).Return("", fmt.Errorf("SomeGetLatestError"))
	updateDetail.Manifest = manifest

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.validateUpdateParam = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := determineTarget(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.False(t, called)
	assert.True(t, finalizeCalled)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.True(t, updateconstants.TargetVersionLatest == updateDetail.TargetResolver)

	assert.Contains(t, updateDetail.StandardOut, "Failed to get latest active version from manifest: SomeGetLatestError")
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestDetermineTarget_TargetVersionLatest_FailedGetLatest(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.TargetVersion = "latest"

	manifest := &updatemanifestmocks.T{}
	manifest.On("GetLatestActiveVersion", mock.Anything).Return("", fmt.Errorf("SomeGetLatestError"))
	updateDetail.Manifest = manifest

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.validateUpdateParam = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := determineTarget(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.False(t, called)
	assert.True(t, finalizeCalled)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)

	assert.Contains(t, updateDetail.StandardOut, "Failed to get latest active version from manifest: SomeGetLatestError")
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestDetermineTarget_TargetVersionLatest_Success(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.TargetVersion = "latest"

	manifest := &updatemanifestmocks.T{}
	manifest.On("GetLatestActiveVersion", mock.Anything).Return("5.6.5.0", nil)
	updateDetail.Manifest = manifest

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.validateUpdateParam = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := determineTarget(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, called)
	assert.False(t, finalizeCalled)
	assert.Equal(t, Initialized, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusInProgress, updateDetail.Result)
	assert.True(t, updateconstants.TargetVersionLatest == updateDetail.TargetResolver)
	assert.Equal(t, "5.6.5.0", updateDetail.TargetVersion)

	assert.Equal(t, "", updateDetail.StandardOut)
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestDetermineTarget_TargetVersionStable_FailedGetStableURL(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.TargetVersion = "stable"
	updateDetail.ManifestURL = "someInvalidManifestUrl"

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.validateUpdateParam = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := determineTarget(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.False(t, called)
	assert.True(t, finalizeCalled)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)

	assert.Contains(t, updateDetail.StandardOut, "Failed to generate stable version from manifest url: ")
	assert.Equal(t, "", updateDetail.StandardError)

	finalizeCalled = false
	tempCode := ""
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		tempCode = code
		finalizeCalled = true
		return nil
	}
	getStableManifestURL = func(manifestURL string, identity identity.IAgentIdentity) (string, error) {
		return "", fmt.Errorf("err1")
	}
	finalizeCalled = false
	// action
	err = determineTarget(updater.mgr, logger, updateDetail)
	// assert
	assert.NoError(t, err)
	assert.True(t, finalizeCalled)
	assert.Contains(t, tempCode, string(updateconstants.ErrorGetStableVersionS3))
	assert.True(t, updateconstants.TargetVersionStable == updateDetail.TargetResolver)
}

func TestDetermineTarget_TargetVersionStable_FailedGetStableVersion(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	getStableManifestURL = updateutil.GetStableURLFromManifestURL

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.TargetVersion = "stable"
	updateDetail.ManifestURL = "https://some.bucket.url/ssm-agent-manifest.json"

	s3Util := &updates3utilmocks.T{}
	s3Util.On("GetStableVersion", "https://some.bucket.url/stable/VERSION", mock.Anything).Return("", fmt.Errorf("SomeGetStableVersionError"))
	updater.mgr.S3util = s3Util

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.validateUpdateParam = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := determineTarget(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.False(t, called)
	assert.True(t, finalizeCalled)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)

	assert.Contains(t, updateDetail.StandardOut, "Failed to get stable version form s3: SomeGetStableVersionError")
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestDetermineTarget_TargetVersionStable_Success(t *testing.T) {
	// setup

	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.TargetVersion = "stable"
	updateDetail.ManifestURL = "https://some.bucket.url/ssm-agent-manifest.json"

	s3Util := &updates3utilmocks.T{}
	s3Util.On("GetStableVersion", "https://some.bucket.url/stable/VERSION", mock.Anything).Return("3.1.1188.0", nil)
	updater.mgr.S3util = s3Util

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.validateUpdateParam = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := determineTarget(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, called)
	assert.False(t, finalizeCalled)
	assert.Equal(t, Initialized, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusInProgress, updateDetail.Result)
	assert.True(t, updateconstants.TargetVersionStable == updateDetail.TargetResolver)
	assert.Equal(t, "3.1.1188.0", updateDetail.TargetVersion)

	assert.Equal(t, "", updateDetail.StandardOut)
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestCleanAgentArtifacts_UpdateSuccess(t *testing.T) {
	var logger = logmocks.NewMockLog()
	ssmAgentDownloadDir := filepath.Join(appconfig.UpdaterArtifactsRoot, updateconstants.UpdateAmazonSSMAgentDir)
	ssmAgentUpdaterDir := filepath.Join(appconfig.UpdaterArtifactsRoot, updateconstants.UpdateAmazonSSMAgentUpdaterDir)
	getDirectoryNames = func(srcPath string) (directories []string, err error) {
		if srcPath == ssmAgentDownloadDir {
			return []string{"2.3.0.1", version.Version}, nil
		}
		return []string{}, nil
	}
	deleteDirectory = func(dirName string) (err error) {
		return nil
	}
	updateDetail := createUpdateDetail(Initialized)
	updateDetail.TargetVersion = version.Version
	updateDetail.SourceVersion = "2.3.0.1"
	updateDetail.Result = contracts.ResultStatusSuccess
	removedVersions := cleanAgentUpdaterDir(logger, updateDetail)
	assert.Equal(t, len(removedVersions[ssmAgentDownloadDir]), 1)
	assert.Equal(t, len(removedVersions[ssmAgentUpdaterDir]), 0)
	assert.Equal(t, removedVersions[ssmAgentDownloadDir][0], "2.3.0.1")
}

func TestCleanAgentArtifacts_UpdateFailed(t *testing.T) {
	var logger = logmocks.NewMockLog()
	ssmAgentDownloadDir := filepath.Join(appconfig.UpdaterArtifactsRoot, updateconstants.UpdateAmazonSSMAgentDir)
	ssmAgentUpdaterDir := filepath.Join(appconfig.UpdaterArtifactsRoot, updateconstants.UpdateAmazonSSMAgentUpdaterDir)
	getDirectoryNames = func(srcPath string) (directories []string, err error) {
		if srcPath == ssmAgentDownloadDir {
			return []string{"2.3.0.1", version.Version}, nil
		}
		return []string{}, nil
	}
	deleteDirectory = func(dirName string) (err error) {
		return nil
	}
	updateDetail := createUpdateDetail(Initialized)
	updateDetail.TargetVersion = version.Version
	updateDetail.SourceVersion = "2.3.0.1"
	updateDetail.Result = contracts.ResultStatusFailed
	removedVersions := cleanAgentUpdaterDir(logger, updateDetail)
	assert.Equal(t, len(removedVersions[ssmAgentDownloadDir]), 1)
	assert.Equal(t, len(removedVersions[ssmAgentUpdaterDir]), 0)
	assert.Equal(t, removedVersions[ssmAgentDownloadDir][0], version.Version)
}

func TestCleanUpdaterArtifacts_UpdateSuccess(t *testing.T) {
	var logger = logmocks.NewMockLog()
	ssmAgentDownloadDir := filepath.Join(appconfig.UpdaterArtifactsRoot, updateconstants.UpdateAmazonSSMAgentDir)
	ssmAgentUpdaterDir := filepath.Join(appconfig.UpdaterArtifactsRoot, updateconstants.UpdateAmazonSSMAgentUpdaterDir)
	getDirectoryNames = func(srcPath string) (directories []string, err error) {
		if srcPath == ssmAgentUpdaterDir {
			return []string{"2.3.0.1", version.Version}, nil
		}
		return []string{}, nil
	}
	deleteDirectory = func(dirName string) (err error) {
		return nil
	}
	updateDetail := createUpdateDetail(Initialized)
	updateDetail.TargetVersion = version.Version
	updateDetail.SourceVersion = "2.3.0.1"
	updateDetail.Result = contracts.ResultStatusSuccess
	removedVersions := cleanAgentUpdaterDir(logger, updateDetail)
	assert.Equal(t, len(removedVersions[ssmAgentDownloadDir]), 0)
	assert.Equal(t, len(removedVersions[ssmAgentUpdaterDir]), 1)
	assert.Equal(t, removedVersions[ssmAgentUpdaterDir][0], "2.3.0.1")
}

func TestCleanUpdaterArtifacts_UpdateFail(t *testing.T) {
	var logger = logmocks.NewMockLog()
	ssmAgentDownloadDir := filepath.Join(appconfig.UpdaterArtifactsRoot, updateconstants.UpdateAmazonSSMAgentDir)
	ssmAgentUpdaterDir := filepath.Join(appconfig.UpdaterArtifactsRoot, updateconstants.UpdateAmazonSSMAgentUpdaterDir)
	getDirectoryNames = func(srcPath string) (directories []string, err error) {
		if srcPath == ssmAgentUpdaterDir {
			return []string{"2.3.0.1", version.Version}, nil
		}
		return []string{}, nil
	}
	deleteDirectory = func(dirName string) (err error) {
		return nil
	}
	updateDetail := createUpdateDetail(Initialized)
	updateDetail.TargetVersion = version.Version
	updateDetail.SourceVersion = "2.3.0.1"
	updateDetail.Result = contracts.ResultStatusFailed
	removedVersions := cleanAgentUpdaterDir(logger, updateDetail)
	assert.Equal(t, len(removedVersions[ssmAgentDownloadDir]), 0)
	assert.Equal(t, len(removedVersions[ssmAgentUpdaterDir]), 1)
	assert.Equal(t, removedVersions[ssmAgentUpdaterDir][0], "2.3.0.1")
}

func TestDetermineTarget_CustomerDefinedVersion_InvalidTarget(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.TargetVersion = "SomeRandomTargetVersion"

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.validateUpdateParam = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := determineTarget(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.False(t, called)
	assert.True(t, finalizeCalled)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.True(t, updateconstants.TargetVersionCustomerDefined == updateDetail.TargetResolver)

	assert.Contains(t, updateDetail.StandardOut, "Invalid target version: SomeRandomTargetVersion")
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestDetermineTarget_CustomerDefinedVersion_Success(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.TargetVersion = "5.6.9.9"

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.validateUpdateParam = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := determineTarget(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, called)
	assert.False(t, finalizeCalled)
	assert.Equal(t, Initialized, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusInProgress, updateDetail.Result)
	assert.True(t, updateconstants.TargetVersionCustomerDefined == updateDetail.TargetResolver)
	assert.Equal(t, "5.6.9.9", updateDetail.TargetVersion)

	assert.Equal(t, "", updateDetail.StandardOut)
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestValidateUpdateParam_FailedInvalidSourceVersion(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.SourceVersion = "SomeInvalidSource"

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.populateUrlHash = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := validateUpdateParam(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.False(t, called)
	assert.True(t, finalizeCalled)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)

	assert.Contains(t, updateDetail.StandardOut, "Failed to compare versions SomeInvalidSource")
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestValidateUpdateParam_VersionAlreadyInstalled(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.TargetVersion = updateDetail.SourceVersion

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.populateUrlHash = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := validateUpdateParam(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.False(t, called)
	assert.True(t, finalizeCalled)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusSuccess, updateDetail.Result)

	assert.Contains(t, updateDetail.StandardOut, "has already been installed")
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestValidateUpdateParam_FailedAttemptDowngrade_AllowDowngradeFalse(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.AllowDowngrade = false
	updateDetail.SourceVersion = "3.0.0.0"
	updateDetail.TargetVersion = "2.0.0.0"

	manifest := &updatemanifestmocks.T{}
	manifest.On("IsVersionActive", mock.Anything, mock.Anything).Return(true, nil)
	updateDetail.Manifest = manifest

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.populateUrlHash = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := validateUpdateParam(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.False(t, called)
	assert.True(t, finalizeCalled)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.True(t, updateDetail.RequiresUninstall)

	assert.Contains(t, updateDetail.StandardOut, "Updating 3.0.0.0 to an older version 2.0.0.0, please enable allow downgrade to proceed")
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestValidateUpdateParam_AllowDowngrade_SourceVersionNotExist(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.AllowDowngrade = true
	updateDetail.SourceVersion = "3.0.0.0"
	updateDetail.TargetVersion = "2.0.0.0"

	manifest := &updatemanifestmocks.T{}
	manifest.On("HasVersion", mock.Anything, updateDetail.SourceVersion).Return(false)
	updateDetail.Manifest = manifest

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.populateUrlHash = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := validateUpdateParam(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.False(t, called)
	assert.True(t, finalizeCalled)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.True(t, updateDetail.RequiresUninstall)

	assert.Contains(t, updateDetail.StandardOut, "source version 3.0.0.0 is unsupported on current platform")
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestValidateUpdateParam_TargetVersionNotExist(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.AllowDowngrade = false

	manifest := &updatemanifestmocks.T{}
	manifest.On("HasVersion", mock.Anything, updateDetail.SourceVersion).Return(true)
	manifest.On("HasVersion", mock.Anything, updateDetail.TargetVersion).Return(false)
	updateDetail.Manifest = manifest

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.populateUrlHash = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := validateUpdateParam(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.False(t, called)
	assert.True(t, finalizeCalled)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.False(t, updateDetail.RequiresUninstall)

	assert.Contains(t, updateDetail.StandardOut, "target version 6.0.0.0 is unsupported on current platform")
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestValidateUpdateParam_FailInvalidVersion(t *testing.T) {
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Initialized)

	manifest := &updatemanifestmocks.T{}
	manifest.On("HasVersion", mock.Anything, updateDetail.SourceVersion).Return(true)
	manifest.On("HasVersion", mock.Anything, updateDetail.TargetVersion).Return(false)
	manifest.On("IsVersionActive", mock.Anything, mock.Anything).Return(false, nil)
	updateDetail.Manifest = manifest

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.populateUrlHash = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := validateUpdateParam(updater.mgr, logger, updateDetail)

	// assert
	assert.Nil(t, err)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)

	assert.Contains(t, updateDetail.StandardOut, "Updating  from 5.0.0.0 to 6.0.0.0")
	assert.True(t, finalizeCalled)
	assert.False(t, called)
}

func TestValidateUpdateParam_FailedPrecondition(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.AllowDowngrade = false

	manifest := &updatemanifestmocks.T{}
	manifest.On("HasVersion", mock.Anything, updateDetail.SourceVersion).Return(true)
	manifest.On("HasVersion", mock.Anything, updateDetail.TargetVersion).Return(true)
	manifest.On("IsVersionActive", mock.Anything, mock.Anything).Return(true, nil)

	updateDetail.Manifest = manifest

	precondition1 := &updatepreconditionmocks.T{}
	precondition1.On("GetPreconditionName").Return("Precondition1")
	precondition1.On("CheckPrecondition", updateDetail.TargetVersion).Return(nil)

	precondition2 := &updatepreconditionmocks.T{}
	precondition2.On("GetPreconditionName").Return("Precondition2")
	precondition2.On("CheckPrecondition", updateDetail.TargetVersion).Return(fmt.Errorf("SomeFailedPrecondition"))
	updater.mgr.preconditions = []updateprecondition.T{precondition1, precondition2}

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.populateUrlHash = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := validateUpdateParam(updater.mgr, logger, updateDetail)

	assert.NoError(t, err)
	assert.False(t, called)
	assert.True(t, finalizeCalled)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.False(t, updateDetail.RequiresUninstall)

	assert.Contains(t, updateDetail.StandardOut, "Failed update precondition check: SomeFailedPrecondition")
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestValidateUpdateParam_FailedIncompatibleVersion(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.AllowDowngrade = true
	updateDetail.UpstreamServiceName = string(contracts.MessageGatewayService)
	updateDetail.TargetVersion = "2.0.0.0"

	manifest := &updatemanifestmocks.T{}
	manifest.On("HasVersion", mock.Anything, updateDetail.SourceVersion).Return(true)
	manifest.On("HasVersion", mock.Anything, updateDetail.TargetVersion).Return(true)
	manifest.On("IsVersionActive", mock.Anything, mock.Anything).Return(true, nil)
	updateDetail.Manifest = manifest

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	// action
	err := validateUpdateParam(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, finalizeCalled)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.True(t, updateDetail.RequiresUninstall)

	assert.Contains(t, updateDetail.StandardOut, "before downgrading to 2.0.0.0, first downgrade to any version from 3.1.821.0 to 3.2.923.0")
}

func TestValidateUpdateParam_SourceVersionV1UpdatePlugin(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.AllowDowngrade = false
	updateDetail.SourceVersion = "3.0.855.0"

	manifest := &updatemanifestmocks.T{}
	manifest.On("HasVersion", mock.Anything, updateDetail.SourceVersion).Return(true)
	manifest.On("HasVersion", mock.Anything, updateDetail.TargetVersion).Return(true)
	manifest.On("IsVersionActive", mock.Anything, mock.Anything).Return(true, nil)

	updateDetail.Manifest = manifest

	precondition1 := &updatepreconditionmocks.T{}
	precondition1.On("GetPreconditionName").Return("Precondition1")
	precondition1.On("CheckPrecondition", updateDetail.TargetVersion).Return(nil)

	precondition2 := &updatepreconditionmocks.T{}
	precondition2.On("GetPreconditionName").Return("Precondition2")
	precondition2.On("CheckPrecondition", updateDetail.TargetVersion).Return(nil)
	updater.mgr.preconditions = []updateprecondition.T{precondition1, precondition2}

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.populateUrlHash = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := validateUpdateParam(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, called)
	assert.False(t, finalizeCalled)
	assert.Equal(t, Initialized, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusInProgress, updateDetail.Result)
	assert.False(t, updateDetail.RequiresUninstall)

	assert.Empty(t, updateDetail.StandardOut)
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestValidateUpdateParam_Success(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.AllowDowngrade = false

	manifest := &updatemanifestmocks.T{}
	manifest.On("HasVersion", mock.Anything, updateDetail.SourceVersion).Return(true)
	manifest.On("HasVersion", mock.Anything, updateDetail.TargetVersion).Return(true)
	manifest.On("IsVersionActive", mock.Anything, mock.Anything).Return(true, nil)
	updateDetail.Manifest = manifest

	precondition1 := &updatepreconditionmocks.T{}
	precondition1.On("GetPreconditionName").Return("Precondition1")
	precondition1.On("CheckPrecondition", updateDetail.TargetVersion).Return(nil)

	precondition2 := &updatepreconditionmocks.T{}
	precondition2.On("GetPreconditionName").Return("Precondition2")
	precondition2.On("CheckPrecondition", updateDetail.TargetVersion).Return(nil)
	updater.mgr.preconditions = []updateprecondition.T{precondition1, precondition2}

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.populateUrlHash = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := validateUpdateParam(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, called)
	assert.False(t, finalizeCalled)
	assert.Equal(t, Initialized, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusInProgress, updateDetail.Result)
	assert.False(t, updateDetail.RequiresUninstall)

	assert.Contains(t, updateDetail.StandardOut, "Updating  from 5.0.0.0 to 6.0.0.0")
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestValidateUpdateParam_Success_DowngradeWhenCurrentVersionInactive(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.SourceVersion = "6.0.0.0"
	updateDetail.TargetVersion = "5.0.0.0"
	updateDetail.PackageName = "amazon-ssm-agent"
	updateDetail.TargetResolver = updateconstants.TargetVersionLatest
	updateDetail.AllowDowngrade = false

	manifest := &updatemanifestmocks.T{}
	manifest.On("HasVersion", mock.Anything, updateDetail.SourceVersion).Return(true)
	manifest.On("HasVersion", mock.Anything, updateDetail.TargetVersion).Return(true)
	manifest.On("IsVersionActive", mock.Anything, updateDetail.TargetVersion).Return(true, nil)
	manifest.On("IsVersionActive", mock.Anything, updateDetail.SourceVersion).Return(false, nil)
	updateDetail.Manifest = manifest

	precondition1 := &updatepreconditionmocks.T{}
	precondition1.On("GetPreconditionName").Return("Precondition1")
	precondition1.On("CheckPrecondition", updateDetail.TargetVersion).Return(nil)

	precondition2 := &updatepreconditionmocks.T{}
	precondition2.On("GetPreconditionName").Return("Precondition2")
	precondition2.On("CheckPrecondition", updateDetail.TargetVersion).Return(nil)
	updater.mgr.preconditions = []updateprecondition.T{precondition1, precondition2}

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	called := false
	updater.mgr.populateUrlHash = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		called = true
		return nil
	}

	// action
	err := validateUpdateParam(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, called)
	assert.False(t, finalizeCalled)
	assert.Equal(t, Initialized, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusInProgress, updateDetail.Result)
	assert.True(t, updateDetail.RequiresUninstall)

	assert.Contains(t, updateDetail.StandardOut, "Updating amazon-ssm-agent from 6.0.0.0 to 5.0.0.0")
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestValidateUpdateParam_Fail_DowngradeWhenCurrentVersionInactiveButManuallySpecified(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()

	updateDetail := createUpdateDetail(Initialized)
	updateDetail.SourceVersion = "6.0.0.0"
	updateDetail.TargetVersion = "5.0.0.1"
	updateDetail.PackageName = "amazon-ssm-agent"
	updateDetail.TargetResolver = updateconstants.TargetVersionCustomerDefined
	updateDetail.AllowDowngrade = false

	manifest := &updatemanifestmocks.T{}
	manifest.On("HasVersion", mock.Anything, updateDetail.SourceVersion).Return(true)
	manifest.On("HasVersion", mock.Anything, updateDetail.TargetVersion).Return(true)
	manifest.On("IsVersionActive", mock.Anything, updateDetail.TargetVersion).Return(true, nil)
	manifest.On("IsVersionActive", mock.Anything, updateDetail.SourceVersion).Return(false, nil)
	updateDetail.Manifest = manifest

	precondition1 := &updatepreconditionmocks.T{}
	precondition1.On("GetPreconditionName").Return("Precondition1")
	precondition1.On("CheckPrecondition", updateDetail.TargetVersion).Return(nil)

	precondition2 := &updatepreconditionmocks.T{}
	precondition2.On("GetPreconditionName").Return("Precondition2")
	precondition2.On("CheckPrecondition", updateDetail.TargetVersion).Return(nil)
	updater.mgr.preconditions = []updateprecondition.T{precondition1, precondition2}

	finalizeCalled := false
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, code string) (err error) {
		finalizeCalled = true
		return nil
	}

	// action
	err := validateUpdateParam(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, finalizeCalled)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.True(t, updateDetail.RequiresUninstall)

	assert.Contains(t, updateDetail.StandardOut, "please enable allow downgrade to proceed")
	assert.Equal(t, "", updateDetail.StandardError)
}

func TestPrepareInstallationPackages(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Initialized)

	manifest := &updatemanifestmocks.T{}
	manifest.On("IsVersionActive", mock.Anything, mock.Anything).Return(true, nil)
	updateDetail.Manifest = manifest

	isUpdateCalled := false

	// stub download for updater
	updater.mgr.download = func(mgr *updateManager, log log.T, downloadInput artifact.DownloadInput, updateDetail *UpdateDetail, version string) (err error) {
		return nil
	}
	// stop at the end of prepareInstallationPackages, do not perform update
	updater.mgr.update = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isUpdateCalled = true
		return nil
	}
	// action
	err := downloadPackages(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, updateDetail.State, Staged)
	assert.NotEmpty(t, updateDetail.StandardOut)
	assert.True(t, isUpdateCalled)
}

func TestDownloadPackagesFailCreateUpdateDownloadFolder(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Initialized)

	manifest := &updatemanifestmocks.T{}
	manifest.On("IsVersionActive", mock.Anything, mock.Anything).Return(true, nil)
	updateDetail.Manifest = manifest

	// stub download for updater
	updater.mgr.download = func(mgr *updateManager, log log.T, downloadInput artifact.DownloadInput, updateDetail *UpdateDetail, version string) (err error) {
		return fmt.Errorf("no access")
	}

	// action
	err := downloadPackages(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
}

func TestDownloadPackagesFailDownload(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{failCreateUpdateDownloadFolder: true}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Initialized)

	manifest := &updatemanifestmocks.T{}
	manifest.On("IsVersionActive", mock.Anything, mock.Anything).Return(true, nil)
	updateDetail.Manifest = manifest

	// action
	err := downloadPackages(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
}

func TestValidateUpdateVersion(t *testing.T) {
	var logger = logmocks.NewMockLog()
	updateDetail := createUpdateDetail(Initialized)

	info := &updateinfomocks.T{}
	info.On("GetPlatform").Return(updateconstants.PlatformRedHat)
	info.On("GetPlatformVersion").Return("10.0.22100")

	err := validateUpdateVersion(logger, updateDetail, info)

	assert.NoError(t, err)
}

func TestValidateUpdateVersionFailCentOs(t *testing.T) {
	var logger = logmocks.NewMockLog()
	updateDetail := createUpdateDetail(Initialized)
	updateDetail.TargetVersion = "1.0.0.0"
	info := &updateinfomocks.T{}
	info.On("GetPlatform").Return(updateconstants.PlatformCentOS)

	err := validateUpdateVersion(logger, updateDetail, info)

	assert.Error(t, err)
}

func TestValidateUpdateVersionFailWindows2025(t *testing.T) {
	updateDetail := createUpdateDetail(Initialized)
	updateDetail.TargetVersion = "1.0.0.0"
	info := &updateinfomocks.T{}
	info.On("GetPlatform").Return("")
	info.On("GetPlatformVersion").Return("10.0.26100")
	isWindowsServer2025OrLater = func(_ string, _ log.T) (bool, error) {
		return true, nil
	}

	err := validateUpdateVersion(logmocks.NewMockLog(), updateDetail, info)

	assert.Error(t, err)
	assert.ErrorContains(t, err, fmt.Sprintf("agent version %s is unsupported on current platform", updateDetail.TargetVersion))
}

func TestValidateUpdateVersionWindows2025(t *testing.T) {
	updateDetail := createUpdateDetail(Initialized)
	updateDetail.TargetVersion = "3.3.1234.0"
	info := &updateinfomocks.T{}
	info.On("GetPlatform").Return("")
	info.On("GetPlatformVersion").Return("10.0.26100")
	isWindowsServer2025OrLater = func(_ string, _ log.T) (bool, error) {
		return true, nil
	}

	err := validateUpdateVersion(logmocks.NewMockLog(), updateDetail, info)

	assert.NoError(t, err)
}

func TestProceedUpdate(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Staged)

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isRollbackCalled := false
	isVerifyCalled := false

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, nil
	}

	updater.mgr.rollback = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isRollbackCalled = true
		return nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, isWaitForCloudInitCalled)
	assert.False(t, isUninstallCalled)
	assert.True(t, isInstallCalled)
	assert.False(t, isRollbackCalled)
	assert.True(t, isVerifyCalled)
}

func TestProceedUpdateWithDowngrade(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Staged)
	updateDetail.RequiresUninstall = true

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isRollbackCalled := false
	isVerifyCalled := false

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, nil
	}

	updater.mgr.rollback = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isRollbackCalled = true
		return nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, isWaitForCloudInitCalled)
	assert.True(t, isUninstallCalled)
	assert.True(t, isInstallCalled)
	assert.False(t, isRollbackCalled)
	assert.True(t, isVerifyCalled)
}

func TestProceedUpdateWithDowngradeFailUninstall(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Staged)
	updateDetail.RequiresUninstall = true
	errMessage := "cannot uninstall"

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isRollbackCalled := false
	isVerifyCalled := false
	finalize_error_code := ""

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, fmt.Errorf("%s", errMessage)
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, nil
	}

	updater.mgr.rollback = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isRollbackCalled = true
		return nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		finalize_error_code = errorCode
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.Contains(t, finalize_error_code, updateconstants.ErrorUninstallFailed)
	assert.Contains(t, updateDetail.StandardOut, errMessage)
	assert.True(t, isWaitForCloudInitCalled)
	assert.True(t, isUninstallCalled)
	assert.False(t, isInstallCalled)
	assert.False(t, isRollbackCalled)
	assert.False(t, isVerifyCalled)
}

func TestProceedUpdateWithDowngradeUninstallReportMetricErrorUsingPkgMgr(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Staged)
	updateDetail.RequiresUninstall = true
	errMessage := "legacy package manager install error"

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isRollbackCalled := false
	isVerifyCalled := false
	var intermediateMetric updateconstants.ErrorCode
	finalize_error_code := ""

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return updateconstants.ExitCodeUpdateErrorUsingPkgMgrLegacy, fmt.Errorf("%s", errMessage)
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, nil
	}

	updater.mgr.rollback = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isRollbackCalled = true
		return nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	updater.mgr.reportMetric = func(mgr *updateManager, updateDetail *UpdateDetail, code updateconstants.ErrorCode) (err error) {
		intermediateMetric = code
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Contains(t, intermediateMetric, updateconstants.ErrorUninstallFailed)
	assert.Contains(t, intermediateMetric, updateconstants.ErrorUsingPkgMgrLegacySuffix)
	assert.Empty(t, finalize_error_code)
	assert.Contains(t, updateDetail.StandardOut, errMessage)
	assert.True(t, isWaitForCloudInitCalled)
	assert.True(t, isUninstallCalled)
	assert.True(t, isInstallCalled)
	assert.False(t, isRollbackCalled)
	assert.True(t, isVerifyCalled)
}

func TestProceedUpdateFailInstall(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Staged)
	errMessage := "install failed"

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isRollbackCalled := false
	isVerifyCalled := false

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, fmt.Errorf("%s", errMessage)
	}

	updater.mgr.rollback = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isRollbackCalled = true
		return nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Rollback, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusInProgress, updateDetail.Result)
	assert.True(t, isWaitForCloudInitCalled)
	assert.False(t, isUninstallCalled)
	assert.True(t, isInstallCalled)
	assert.True(t, isRollbackCalled)
	assert.False(t, isVerifyCalled)
}

func TestProceedUpdateWithDowngradeFailUninstallErrorPrepareUpdateCommand(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Staged)
	updateDetail.RequiresUninstall = true
	errMessage := "Error Prepare Update Command"

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isRollbackCalled := false
	isVerifyCalled := false
	finalize_error_code := ""

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return updateconstants.ExitCodeErrorPrepareUpdateCommand, fmt.Errorf("%s", errMessage)
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, nil
	}

	updater.mgr.rollback = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isRollbackCalled = true
		return nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		finalize_error_code = errorCode
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.Contains(t, finalize_error_code, updateconstants.ErrorUninstallFailed)
	assert.Contains(t, finalize_error_code, updateconstants.ErrorPrepareUpdateCommandSuffix)
	assert.Contains(t, updateDetail.StandardOut, errMessage)
	assert.True(t, isWaitForCloudInitCalled)
	assert.True(t, isUninstallCalled)
	assert.False(t, isInstallCalled)
	assert.False(t, isRollbackCalled)
	assert.False(t, isVerifyCalled)
}

func TestProceedUpdateFailInstallErrorPrepareUpdateCommand(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Staged)
	errMessage := "Error Prepare Update Command"

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isRollbackCalled := false
	isVerifyCalled := false
	finalize_error_code := ""

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return updateconstants.ExitCodeErrorPrepareUpdateCommand, fmt.Errorf("%s", errMessage)
	}

	updater.mgr.rollback = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isRollbackCalled = true
		return nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		finalize_error_code = errorCode
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.Contains(t, finalize_error_code, updateconstants.ErrorInstallFailed)
	assert.Contains(t, finalize_error_code, updateconstants.ErrorPrepareUpdateCommandSuffix)
	assert.Contains(t, updateDetail.StandardOut, errMessage)
	assert.True(t, isWaitForCloudInitCalled)
	assert.False(t, isUninstallCalled)
	assert.True(t, isInstallCalled)
	assert.False(t, isRollbackCalled)
	assert.False(t, isVerifyCalled)
}

func TestProceedUpdateWithDowngradeFailUninstallWithUnsupportedServiceMgr(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Staged)
	updateDetail.RequiresUninstall = true
	errMessage := "Invalid Platform"

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isRollbackCalled := false
	isVerifyCalled := false
	finalize_error_code := ""

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return updateconstants.ExitCodeUnsupportedPlatform, fmt.Errorf("%s", errMessage)
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, nil
	}

	updater.mgr.rollback = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isRollbackCalled = true
		return nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		finalize_error_code = errorCode
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.Contains(t, finalize_error_code, updateconstants.ErrorUnsupportedServiceManager)
	assert.Contains(t, updateDetail.StandardOut, errMessage)
	assert.True(t, isWaitForCloudInitCalled)
	assert.True(t, isUninstallCalled)
	assert.False(t, isInstallCalled)
	assert.False(t, isRollbackCalled)
	assert.False(t, isVerifyCalled)
}

func TestProceedUpdateFailInstallWithUnsupportedServiceMgr(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Staged)
	errMessage := "Invalid Platform"

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isRollbackCalled := false
	isVerifyCalled := false
	finalize_error_code := ""

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return updateconstants.ExitCodeUnsupportedPlatform, fmt.Errorf("%s", errMessage)
	}

	updater.mgr.rollback = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isRollbackCalled = true
		return nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		finalize_error_code = errorCode
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.Contains(t, finalize_error_code, updateconstants.ErrorUnsupportedServiceManager)
	assert.Contains(t, updateDetail.StandardOut, errMessage)
	assert.True(t, isWaitForCloudInitCalled)
	assert.False(t, isUninstallCalled)
	assert.True(t, isInstallCalled)
	assert.False(t, isRollbackCalled)
	assert.False(t, isVerifyCalled)
}

func TestProceedUpdateFailInstallWithFailureDueToSnapd(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Staged)
	errMessage := "child process still running"

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isRollbackCalled := false
	isVerifyCalled := false
	finalize_error_code := ""

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return updateconstants.ExitCodeUpdateFailedDueToSnapd, fmt.Errorf("%s", errMessage)
	}

	updater.mgr.rollback = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isRollbackCalled = true
		return nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		finalize_error_code = errorCode
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.Contains(t, finalize_error_code, updateconstants.ErrorInstallFailureDueToSnapd)
	assert.Contains(t, updateDetail.StandardOut, errMessage)
	assert.True(t, isWaitForCloudInitCalled)
	assert.False(t, isUninstallCalled)
	assert.True(t, isInstallCalled)
	assert.False(t, isRollbackCalled)
	assert.False(t, isVerifyCalled)
}

func TestProceedUpdateInstallReportMetricDueToSigningIssue(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Staged)
	errMessage := "does not verify: no digest"

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isRollbackCalled := false
	isVerifyCalled := false
	var intermediateMetric updateconstants.ErrorCode

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return updateconstants.ExitCodeInstallFailedDueToSigningIssue, fmt.Errorf("%s", errMessage)
	}

	updater.mgr.rollback = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isRollbackCalled = true
		return nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	updater.mgr.reportMetric = func(mgr *updateManager, updateDetail *UpdateDetail, code updateconstants.ErrorCode) (err error) {
		intermediateMetric = code
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Rollback, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusInProgress, updateDetail.Result)
	assert.Contains(t, intermediateMetric, updateconstants.ErrorInstallFailedDueToSigningIssue)
	assert.Contains(t, updateDetail.StandardOut, errMessage)
	assert.True(t, isWaitForCloudInitCalled)
	assert.False(t, isUninstallCalled)
	assert.True(t, isInstallCalled)
	assert.True(t, isRollbackCalled)
	assert.False(t, isVerifyCalled)
}

func TestProceedUpdateInstallReportMetricErrorUsingPkgMgr(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Staged)
	errMessage := "yum install error"

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isRollbackCalled := false
	isVerifyCalled := false
	var intermediateMetric updateconstants.ErrorCode

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return updateconstants.ExitCodeUpdateErrorUsingYumAndRpm, fmt.Errorf("%s", errMessage)
	}

	updater.mgr.rollback = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isRollbackCalled = true
		return nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	updater.mgr.reportMetric = func(mgr *updateManager, updateDetail *UpdateDetail, code updateconstants.ErrorCode) (err error) {
		intermediateMetric = code
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Rollback, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusInProgress, updateDetail.Result)
	assert.Contains(t, intermediateMetric, updateconstants.ErrorInstallFailed)
	assert.Contains(t, intermediateMetric, updateconstants.ErrorUsingYumAndRpmSuffix)
	assert.Contains(t, updateDetail.StandardOut, errMessage)
	assert.True(t, isWaitForCloudInitCalled)
	assert.False(t, isUninstallCalled)
	assert.True(t, isInstallCalled)
	assert.True(t, isRollbackCalled)
	assert.False(t, isVerifyCalled)
}

func TestVerifyInstallation(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{serviceIsRunning: true}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Installed)

	// action
	err := verifyInstallation(updater.mgr, logger, updateDetail, false)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
}

func TestVerifyInstallationFailedGetInstanceInfo(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{failCreateInstanceContext: true}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Installed)

	// action
	err := verifyInstallation(updater.mgr, logger, updateDetail, false)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
}

func TestVerifyInstallation_VersionCheck_Success(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{serviceIsRunning: true, versionErrorCode: ""}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Installed)
	isRollbackCalled := false

	updater.mgr.rollback = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isRollbackCalled = true
		return nil
	}

	finalize_error_code := ""
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		finalize_error_code = errorCode
		return nil
	}
	// action
	err := verifyInstallation(updater.mgr, logger, updateDetail, false)

	// assert
	assert.NoError(t, err)
	assert.False(t, isRollbackCalled)
	assert.Equal(t, updateDetail.State, Completed)
	assert.Equal(t, updateDetail.Result, contracts.ResultStatusSuccess)
	assert.Equal(t, string(""), finalize_error_code)
}

func TestVerifyInstallation_VersionCheck_Failed_Reg(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{serviceIsRunning: true, versionErrorCode: updateconstants.ErrorInstTargetVersionNotFoundViaReg}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Installed)
	isRollbackCalled := false

	updater.mgr.rollback = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isRollbackCalled = true
		return nil
	}
	finalize_error_code := ""
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		finalize_error_code = errorCode
		return nil
	}
	// action
	err := verifyInstallation(updater.mgr, logger, updateDetail, false)

	// assert
	assert.NoError(t, err)
	assert.False(t, isRollbackCalled)
	assert.Equal(t, updateDetail.State, Completed)
	assert.Equal(t, updateDetail.Result, contracts.ResultStatusFailed)
	assert.Equal(t, string(updateconstants.ErrorInstTargetVersionNotFoundViaReg), finalize_error_code)
}

func TestVerifyInstallationCannotStartAgent(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Installed)
	expectedVersion := updateDetail.TargetVersion
	isRollbackCalled := false

	updater.mgr.rollback = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isRollbackCalled = true
		return nil
	}

	// action
	err := verifyInstallation(updater.mgr, logger, updateDetail, false)

	// assert
	assert.NoError(t, err)
	assert.True(t, isRollbackCalled)
	assert.Equal(t, expectedVersion, control.getWaitForServiceVersion())
	assert.Equal(t, updateDetail.State, Rollback)
}

func TestVerifyRollback(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{serviceIsRunning: true}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(RolledBack)
	expectedVersion := updateDetail.SourceVersion

	// action
	err := verifyInstallation(updater.mgr, logger, updateDetail, true)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, expectedVersion, control.getWaitForServiceVersion())
}

func TestVerifyRollbackCannotStartAgent(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)

	// open network required
	updateDetail := createUpdateDetail(RolledBack)

	// action
	err := verifyInstallation(updater.mgr, logger, updateDetail, true)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
}

func TestRollbackInstallation(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Rollback)

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isVerifyCalled := false

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	// action
	err := rollbackInstallation(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, RolledBack, updateDetail.State)
	assert.False(t, isWaitForCloudInitCalled)
	assert.True(t, isUninstallCalled)
	assert.True(t, isInstallCalled)
	assert.True(t, isVerifyCalled)
}

func TestRollbackInstallationFailUninstall(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Rollback)
	errMessage := "cannot uninstall"

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isVerifyCalled := false
	finalize_error_code := ""

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, fmt.Errorf("%s", errMessage)
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		finalize_error_code = errorCode
		return nil
	}

	// action
	err := rollbackInstallation(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.Contains(t, finalize_error_code, updateconstants.ErrorUninstallFailed)
	assert.Contains(t, updateDetail.StandardOut, errMessage)
	assert.False(t, isWaitForCloudInitCalled)
	assert.True(t, isUninstallCalled)
	assert.False(t, isInstallCalled)
	assert.False(t, isVerifyCalled)
}

func TestRollbackInstallationUninstallReportMetricErrorUsingPkgMgr(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Rollback)
	errMessage := "dpkg install error"

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isVerifyCalled := false
	var intermediateMetric updateconstants.ErrorCode
	finalize_error_code := ""

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return updateconstants.ExitCodeUpdateErrorUsingDpkg, fmt.Errorf("%s", errMessage)
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	updater.mgr.reportMetric = func(mgr *updateManager, updateDetail *UpdateDetail, code updateconstants.ErrorCode) (err error) {
		intermediateMetric = code
		return nil
	}

	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		finalize_error_code = errorCode
		return nil
	}

	// action
	err := rollbackInstallation(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, RolledBack, updateDetail.State)
	assert.Contains(t, intermediateMetric, updateconstants.ErrorUninstallFailed)
	assert.Contains(t, intermediateMetric, updateconstants.ErrorUsingDpkgSuffix)
	assert.Empty(t, finalize_error_code)
	assert.Contains(t, updateDetail.StandardOut, errMessage)
	assert.False(t, isWaitForCloudInitCalled)
	assert.True(t, isUninstallCalled)
	assert.True(t, isInstallCalled)
	assert.True(t, isVerifyCalled)
}

func TestRollbackInstallationFailInstall(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Rollback)
	errMessage := "cannot install"

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isVerifyCalled := false
	finalize_error_code := ""

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, fmt.Errorf("%s", errMessage)
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		finalize_error_code = errorCode
		return nil
	}

	// action
	err := rollbackInstallation(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.Contains(t, finalize_error_code, updateconstants.ErrorInstallFailed)
	assert.Contains(t, updateDetail.StandardOut, errMessage)
	assert.False(t, isWaitForCloudInitCalled)
	assert.True(t, isUninstallCalled)
	assert.True(t, isInstallCalled)
	assert.False(t, isVerifyCalled)
}

func TestRollbackInstallationFailUninstallErrorPrepareUpdateCommand(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Rollback)
	errMessage := "Error Prepare Update Command"

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isVerifyCalled := false
	finalize_error_code := ""

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return updateconstants.ExitCodeErrorPrepareUpdateCommand, fmt.Errorf("%s", errMessage)
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		finalize_error_code = errorCode
		return nil
	}

	// action
	err := rollbackInstallation(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.Contains(t, finalize_error_code, updateconstants.ErrorUninstallFailed)
	assert.Contains(t, finalize_error_code, updateconstants.ErrorPrepareUpdateCommandSuffix)
	assert.Contains(t, updateDetail.StandardOut, errMessage)
	assert.False(t, isWaitForCloudInitCalled)
	assert.True(t, isUninstallCalled)
	assert.False(t, isInstallCalled)
	assert.False(t, isVerifyCalled)
}

func TestRollbackInstallationFailInstallErrorPrepareUpdateCommand(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Rollback)
	errMessage := "Error Prepare Update Command"

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isVerifyCalled := false
	finalize_error_code := ""

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return updateconstants.ExitCodeErrorPrepareUpdateCommand, fmt.Errorf("%s", errMessage)
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		finalize_error_code = errorCode
		return nil
	}

	// action
	err := rollbackInstallation(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.Contains(t, finalize_error_code, updateconstants.ErrorInstallFailed)
	assert.Contains(t, finalize_error_code, updateconstants.ErrorPrepareUpdateCommandSuffix)
	assert.Contains(t, updateDetail.StandardOut, errMessage)
	assert.False(t, isWaitForCloudInitCalled)
	assert.True(t, isUninstallCalled)
	assert.True(t, isInstallCalled)
	assert.False(t, isVerifyCalled)
}

func TestRollbackInstallationFailUninstallWithUnsupportedServiceMgr(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Rollback)
	errMessage := "Invalid Platform"

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isVerifyCalled := false
	finalize_error_code := ""

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return updateconstants.ExitCodeUnsupportedPlatform, fmt.Errorf("%s", errMessage)
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		finalize_error_code = errorCode
		return nil
	}

	// action
	err := rollbackInstallation(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.Contains(t, finalize_error_code, updateconstants.ErrorUnsupportedServiceManager)
	assert.Contains(t, updateDetail.StandardOut, errMessage)
	assert.False(t, isWaitForCloudInitCalled)
	assert.True(t, isUninstallCalled)
	assert.False(t, isInstallCalled)
	assert.False(t, isVerifyCalled)
}

func TestRollbackInstallationFailInstallWithUnsupportedServiceMgr(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Rollback)
	errMessage := "Invalid Platform"

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isVerifyCalled := false
	finalize_error_code := ""

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return updateconstants.ExitCodeUnsupportedPlatform, fmt.Errorf("%s", errMessage)
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		finalize_error_code = errorCode
		return nil
	}

	// action
	err := rollbackInstallation(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.Contains(t, finalize_error_code, updateconstants.ErrorUnsupportedServiceManager)
	assert.Contains(t, updateDetail.StandardOut, errMessage)
	assert.False(t, isWaitForCloudInitCalled)
	assert.True(t, isUninstallCalled)
	assert.True(t, isInstallCalled)
	assert.False(t, isVerifyCalled)
}

func TestRollbackInstallationFailInstallWithFailureDueToSnapd(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Rollback)
	errMessage := "child process still running"

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isVerifyCalled := false
	finalize_error_code := ""

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return updateconstants.ExitCodeUpdateFailedDueToSnapd, fmt.Errorf("%s", errMessage)
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		finalize_error_code = errorCode
		return nil
	}

	// action
	err := rollbackInstallation(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.NotContains(t, finalize_error_code, updateconstants.ErrorInstallFailureDueToSnapd)
	assert.Contains(t, updateDetail.StandardOut, errMessage)
	assert.False(t, isWaitForCloudInitCalled)
	assert.True(t, isUninstallCalled)
	assert.True(t, isInstallCalled)
	assert.False(t, isVerifyCalled)
}

func TestRollbackInstallationFailInstallDueToSigningIssue(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Rollback)
	errMessage := "does not verify: no digest"

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isVerifyCalled := false
	finalize_error_code := ""

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return updateconstants.ExitCodeInstallFailedDueToSigningIssue, fmt.Errorf("%s", errMessage)
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		finalize_error_code = errorCode
		return nil
	}

	// action
	err := rollbackInstallation(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.Contains(t, finalize_error_code, updateconstants.ErrorInstallFailedDueToSigningIssue)
	assert.Contains(t, updateDetail.StandardOut, errMessage)
	assert.False(t, isWaitForCloudInitCalled)
	assert.True(t, isUninstallCalled)
	assert.True(t, isInstallCalled)
	assert.False(t, isVerifyCalled)
}

func TestRollbackInstallationFailInstallErrorUsingPkgMgr(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Rollback)
	errMessage := "legacy package manager install error"

	isWaitForCloudInitCalled := false
	isUninstallCalled := false
	isInstallCalled := false
	isVerifyCalled := false
	finalize_error_code := ""

	// stub functions
	waitForCloudInit = func(log log.T, timeoutSeconds int) error {
		isWaitForCloudInitCalled = true
		return nil
	}

	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return updateconstants.ExitCodeUpdateErrorUsingPkgMgrLegacy, fmt.Errorf("%s", errMessage)
	}

	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		finalize_error_code = errorCode
		return nil
	}

	// action
	err := rollbackInstallation(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, contracts.ResultStatusFailed, updateDetail.Result)
	assert.Contains(t, finalize_error_code, updateconstants.ErrorInstallFailed)
	assert.Contains(t, finalize_error_code, updateconstants.ErrorUsingPkgMgrLegacySuffix)
	assert.Contains(t, updateDetail.StandardOut, errMessage)
	assert.False(t, isWaitForCloudInitCalled)
	assert.True(t, isUninstallCalled)
	assert.True(t, isInstallCalled)
	assert.False(t, isVerifyCalled)
}

func TestUninstallAgent(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{failExeCommand: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Initialized)

	// action
	exitCode, err := uninstallAgent(updater.mgr, logger, updateDetail.TargetVersion, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, -1, int(exitCode))
}

func TestUninstallAgentFailExeCommand(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{failExeCommand: true, failExeCommandExitCode: -1}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Initialized)

	// action
	exitCode, err := uninstallAgent(updater.mgr, logger, updateDetail.TargetVersion, updateDetail)

	// assert
	assert.Error(t, err)
	assert.Equal(t, -1, int(exitCode))
}

func TestInstallAgent(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{failExeCommand: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Initialized)
	// action
	exitCode, err := installAgent(updater.mgr, logger, updateDetail.TargetVersion, updateDetail)
	// assert

	assert.NoError(t, err)
	assert.Equal(t, -1, int(exitCode))
}

func TestInstallAgentFailExeCommand(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{failExeCommand: true, failExeCommandExitCode: -1}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Initialized)

	// action
	exitCode, err := installAgent(updater.mgr, logger, updateDetail.TargetVersion, updateDetail)

	// assert
	assert.Error(t, err)
	assert.Equal(t, -1, int(exitCode))
}

func TestInstallAgentFailedDueToSigningIssue(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{failExeCommand: true, failExeCommandExitCode: updateconstants.ExitCodeInstallFailedDueToSigningIssue}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Initialized)

	// action
	exitCode, err := installAgent(updater.mgr, logger, updateDetail.TargetVersion, updateDetail)

	// assert
	assert.Contains(t, "failed to run script", err.Error())
	assert.Equal(t, updateconstants.ExitCodeInstallFailedDueToSigningIssue, exitCode)
}

func createUpdaterWithStubsForSnap(control *stubControl) *Updater {
	context := contextmocks.NewMockDefault()
	info := &updateinfomocks.T{}
	info.On("GetPlatform").Return(updateconstants.PlatformUbuntu)
	info.On("GetUninstallScriptName").Return("snap_" + updateconstants.UninstallScript)
	info.On("GetInstallScriptName").Return("snap_" + updateconstants.InstallScript)

	updater := NewUpdater(context, info, &updateutil.Utility{})
	updater.mgr.svc = &serviceStub{}
	util := &utilityStub{controller: control}
	util.Context = context
	updater.mgr.util = util
	updater.mgr.ctxMgr = &contextMgrStub{}
	updater.mgr.Context = context

	return updater
}

func TestInstallAgentUsingSnap_Success(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{failExeCommand: false}
	updater := createUpdaterWithStubsForSnap(control)
	updateDetail := createUpdateDetail(Initialized)

	// action
	exitCode, err := installAgent(updater.mgr, logger, updateDetail.TargetVersion, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, -1, int(exitCode))
}

func TestInstallAgentUsingSnap_Failed(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{failExeCommand: true, failExeCommandExitCode: updateconstants.ExitCodeUpdateFailedDueToSnapd}
	updater := createUpdaterWithStubsForSnap(control)
	updateDetail := createUpdateDetail(Initialized)

	// action
	exitCode, err := installAgent(updater.mgr, logger, updateDetail.TargetVersion, updateDetail)

	// assert
	assert.Contains(t, "failed to run script", err.Error())
	assert.Equal(t, updateconstants.ExitCodeUpdateFailedDueToSnapd, exitCode)
}

func TestDownloadAndUnzipArtifact(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{failExeCommand: true}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Initialized)
	downloadOutput := artifact.DownloadOutput{
		IsHashMatched: true,
		LocalFilePath: "filepath",
	}

	downloadArtifact = func(context context.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
		return downloadOutput, nil
	}
	uncompress = func(log log.T, src, dest string) error {
		return nil
	}

	// action
	err := downloadAndUnzipArtifact(updater.mgr, logger, artifact.DownloadInput{}, updateDetail, updateDetail.TargetVersion)

	// assert
	assert.NoError(t, err)
}

func TestDownloadWithError(t *testing.T) {
	// setup
	var logger = logmocks.NewMockLog()
	control := &stubControl{failExeCommand: true}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Initialized)
	downloadOutput := artifact.DownloadOutput{
		IsHashMatched: false,
		LocalFilePath: "",
	}

	downloadArtifact = func(context context.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
		return downloadOutput, nil
	}

	// action
	err := downloadAndUnzipArtifact(updater.mgr, logger, artifact.DownloadInput{}, updateDetail, updateDetail.TargetVersion)

	// assert
	assert.Error(t, err)
}

func TestValidateTargetVersionCompatible_DoesNotBlockMDSUpdates(t *testing.T) {
	updateDetail := createUpdateDetail(Initialized)
	updateDetail.TargetVersion = "2.0.0.0"
	updateDetail.UpstreamServiceName = string(contracts.MessageDeliveryService)
	assert.Nil(t, validateTargetVersionCompatible(updateDetail))
}

func TestValidateTargetVersionCompatible_DoesNotBlockMGSUpdateToCompatibleVersion(t *testing.T) {
	updateDetail := createUpdateDetail(Initialized)
	updateDetail.TargetVersion = "3.1.821.0"
	updateDetail.UpstreamServiceName = string(contracts.MessageDeliveryService)
	assert.Nil(t, validateTargetVersionCompatible(updateDetail))
}

func TestValidateTargetVersionCompatible_BlocksMGSUpdatesToIncompatibleVersion(t *testing.T) {
	updateDetail := createUpdateDetail(Initialized)
	updateDetail.TargetVersion = "2.0.0.0"
	updateDetail.UpstreamServiceName = string(contracts.MessageGatewayService)
	assert.NotNil(t, validateTargetVersionCompatible(updateDetail))
}

// createUpdaterWithStubs creates stubs updater and it's manager, util and service
func createDefaultUpdaterStub() *Updater {
	return createUpdaterStubs(&stubControl{})
}

func createUpdaterStubs(control *stubControl) *Updater {
	context := contextmocks.NewMockDefault()
	info := &updateinfomocks.T{}
	info.On("GetPlatform").Return(updateconstants.PlatformRedHat)
	info.On("GetPlatformVersion").Return("10.0.22100")
	info.On("GetUninstallScriptName").Return(updateconstants.UninstallScript)
	info.On("GetInstallScriptName").Return(updateconstants.InstallScript)

	updater := NewUpdater(context, info, &updateutil.Utility{})
	updater.mgr.svc = &serviceStub{}
	util := &utilityStub{controller: control}
	util.Context = context
	updater.mgr.util = util
	updater.mgr.ctxMgr = &contextMgrStub{}
	updater.mgr.Context = context

	return updater
}

type stubControl struct {
	failCreateInstanceContext      bool
	failCreateUpdateDownloadFolder bool
	serviceIsRunning               bool
	failExeCommand                 bool
	failExeCommandExitCode         updateconstants.UpdateScriptExitCode
	waitForServiceVersion          string
	versionErrorCode               updateconstants.ErrorCode
}

func (s *stubControl) getWaitForServiceVersion() string {
	return s.waitForServiceVersion
}

type utilityStub struct {
	updateutil.Utility
	controller *stubControl
}

func (u *utilityStub) CreateUpdateDownloadFolder() (folder string, err error) {
	if u.controller.failCreateUpdateDownloadFolder {
		return "", fmt.Errorf("failed to create update download folder")
	}
	return "rootfolder", nil
}

func (u *utilityStub) ExeCommand(commandExecSettings *updateutil.CommandExecutionSettings) (pid int, exitCode updateconstants.UpdateScriptExitCode, err error) {
	if u.controller.failExeCommand {
		return -1, u.controller.failExeCommandExitCode, fmt.Errorf("failed run script")
	}
	return 1, -1, nil
}

func (u *utilityStub) ExecCommandWithOutput(commandExecSettings *updateutil.CommandExecutionSettings) (pid int, exitCode updateconstants.UpdateScriptExitCode, stdoutBytes *bytes.Buffer, errorBytes *bytes.Buffer, cmdErr error) {
	var outBytes bytes.Buffer
	var errBytes bytes.Buffer

	if u.controller.failExeCommand {
		if u.controller.failExeCommandExitCode == updateconstants.ExitCodeUpdateFailedDueToSnapd {
			errBytes.WriteString("snap \"amazon-ssm-agent\" has running apps")
			return -1, -1, nil, &errBytes, fmt.Errorf("failed to run script")
		}

		if u.controller.failExeCommandExitCode == updateconstants.ExitCodeInstallFailedDueToSigningIssue {
			outBytes.WriteString("does not verify: no digest")
			return -1, -1, &outBytes, &errBytes, fmt.Errorf("failed to run script")
		}
		return -1, u.controller.failExeCommandExitCode, &outBytes, &errBytes, fmt.Errorf("failed to run script")
	}
	return 1, -1, &outBytes, nil, nil
}

func (u *utilityStub) SaveUpdatePluginResult(log log.T, updaterRoot string, updateResult *updateutil.UpdatePluginResult) (err error) {
	return nil
}

func (u *utilityStub) IsServiceRunning(log log.T, i updateinfo.T) (result bool, err error) {
	if u.controller.serviceIsRunning {
		return true, nil
	}
	return false, nil
}

func (u *utilityStub) WaitForServiceToStart(log log.T, i updateinfo.T, targetVersion string) (result bool, err error) {
	u.controller.waitForServiceVersion = targetVersion
	if u.controller.serviceIsRunning {
		return true, nil
	}
	return false, nil
}

func (u *utilityStub) VerifyInstalledVersion(log log.T, targetVersion string) updateconstants.ErrorCode {
	return u.controller.versionErrorCode
}

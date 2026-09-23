// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package bootstrap

import (
	"errors"
	"testing"

	"runtime"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	identity2 "github.com/aws/amazon-ssm-agent/common/identity/identity"
	identityMock "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	contextPackage "github.com/aws/amazon-ssm-agent/core/app/context"
	fileSystemMock "github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/datastore/filesystem/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestBootstrap(t *testing.T) {
	logger := logmocks.NewMockLog()
	fileSystem := &fileSystemMock.FileSystem{}

	agentIdentity := &identityMock.IAgentIdentity{}
	agentIdentity.On("InstanceID").Return("SomeInstanceId", nil)

	newAgentIdentitySelector = func(log log.T) identity2.IAgentIdentitySelector {
		// Selector object does not matter since it is exclusively passed to newAgentIdentity function
		return nil
	}

	newAgentIdentity = func(log log.T, config *appconfig.SsmagentConfig, selector identity2.IAgentIdentitySelector) (identity.IAgentIdentity, error) {
		return agentIdentity, nil
	}

	fileSystem.On("Stat", mock.Anything).Return(nil, nil)
	fileSystem.On("IsNotExist", mock.Anything).Return(true)
	fileSystem.On("MkdirAll", mock.Anything, mock.Anything).Return(nil)

	bs := NewBootstrap(logger, fileSystem)

	context, err := bs.Init()
	assert.Nil(t, err)
	assert.NotNil(t, context)

	assert.Equal(t, context.Log(), logger)
	assert.Equal(t, context.Identity(), agentIdentity)

	bsTestDefer := NewBootstrap(logger, nil)
	bsTestDefer.Init()

	fileSystemError := &fileSystemMock.FileSystem{}
	fileSystemError.On("Stat", mock.Anything).Return(nil, nil)
	fileSystemError.On("IsNotExist", mock.Anything).Return(true)
	fileSystemError.On("MkdirAll", mock.Anything, mock.Anything).Return(errors.New("filesystem mocked error for testing"))
	bsTestCreateIpsFolder := NewBootstrap(logger, fileSystemError)
	_, createIpsFolderError := bsTestCreateIpsFolder.Init()
	if runtime.GOOS != "windows" {
		assert.NotNil(t, createIpsFolderError)
		assert.EqualError(t, createIpsFolderError, "Errorf: failed to create IPC folder, filesystem mocked error for testing")
	}

	agentIdentityMocked := &identityMock.IAgentIdentity{}
	agentIdentityMocked.On("InstanceID").Return("SomeInstanceId", errors.New("InstanceID mocked error for testing"))
	newAgentIdentity = func(log log.T, config *appconfig.SsmagentConfig, selector identity2.IAgentIdentitySelector) (identity.IAgentIdentity, error) {
		return agentIdentityMocked, nil
	}
	bsMockedAgentIdentitiy := NewBootstrap(logger, fileSystem)
	contextInstanceIdTest, errInstanceId := bsMockedAgentIdentitiy.Init()
	assert.Nil(t, contextInstanceIdTest)
	assert.NotNil(t, errInstanceId)
	assert.EqualError(t, errInstanceId, "Errorf: error fetching the instanceID, InstanceID mocked error for testing")

	newAgentIdentity = func(log log.T, config *appconfig.SsmagentConfig, selector identity2.IAgentIdentitySelector) (identity.IAgentIdentity, error) {
		return agentIdentity, nil
	}
	coreagentContext = func(logger log.T, ssmAppconfig *appconfig.SsmagentConfig, agentIdentity identity.IAgentIdentity) (contextPackage.ICoreAgentContext, error) {
		return nil, errors.New("context mocked error for testing")
	}
	bsMockedForContext := NewBootstrap(logger, fileSystem)
	_, errContext := bsMockedForContext.Init()
	assert.NotNil(t, errContext)
	assert.EqualError(t, errContext, "Errorf: context could not be loaded - <nil>")

	newAgentIdentity = func(log log.T, config *appconfig.SsmagentConfig, selector identity2.IAgentIdentitySelector) (identity.IAgentIdentity, error) {
		return agentIdentity, errors.New("identity mocked error for testing")
	}
	bsAgentIdentitiyError := NewBootstrap(logger, fileSystem)
	_, errIdentity := bsAgentIdentitiyError.Init()
	assert.NotNil(t, errIdentity)
	assert.EqualError(t, errIdentity, "Errorf: failed to get identity: identity mocked error for testing")

	appConfig = func(reload bool) (appconfig.SsmagentConfig, error) {
		return appconfig.DefaultConfig(), errors.New("appconfig mocked error for testing")
	}
	bsMockedAppConfig := NewBootstrap(logger, fileSystem)
	_, errAppConfig := bsMockedAppConfig.Init()
	assert.NotNil(t, errAppConfig)
	assert.EqualError(t, errAppConfig, "app config could not be loaded - appconfig mocked error for testing")
}

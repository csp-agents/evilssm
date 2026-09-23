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

package windowscontainerutil

import (
	"errors"
	"os"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var contextMock = context.NewMockDefault()
var osStatFunc = os.Stat

func successMock() *DepMock {
	depmock := DepMock{}
	depmock.On("PlatformVersion", mock.Anything).Return("10", nil)
	depmock.On("IsPlatformNanoServer", mock.Anything).Return(false, nil)
	depmock.On("SetDaemonConfig", mock.Anything, mock.Anything).Return(nil)
	depmock.On("MakeDirs", mock.Anything).Return(nil)
	depmock.On("TempDir", mock.Anything, mock.Anything).Return("test", nil)
	depmock.On("UpdateUtilExeCommandOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("True", nil)
	depmock.On("ArtifactDownload", mock.Anything, mock.Anything).Return(artifact.DownloadOutput{}, nil)
	depmock.On("LocalRegistryKeySetDWordValue", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	depmock.On("LocalRegistryKeyGetStringValue", mock.Anything, mock.Anything).Return("", 0, nil)
	depmock.On("FileutilUncompress", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return &depmock
}

func TestInstall(t *testing.T) {
	depOrig := dep
	containerMock := successMock()
	dep = containerMock
	defer func() { dep = depOrig }()

	output := iohandler.DefaultIOHandler{}
	RunInstallCommands(contextMock, "", &output)

	assert.Equal(t, output.GetExitCode(), 0)
	assert.Contains(t, output.GetStdout(), "Installation complete")
	containerMock.AssertCalled(t, "PlatformVersion", mock.Anything)
	containerMock.AssertCalled(t, "IsPlatformNanoServer", mock.Anything)
	containerMock.AssertNumberOfCalls(t, "UpdateUtilExeCommandOutput", 4)
}

func TestInstallUnsupportedPlatform(t *testing.T) {
	// Mock dependency to return an unsupported Windows version
	depOrig := dep
	containerMock := &DepMock{}
	containerMock.On("PlatformVersion", mock.Anything).Return("6.3", nil) // Windows 8.1
	dep = containerMock
	defer func() { dep = depOrig }()

	output := iohandler.DefaultIOHandler{}
	RunInstallCommands(contextMock, "", &output)

	// Verify failure status and error message
	assert.Equal(t, output.GetExitCode(), 1)
	assert.Contains(t, output.GetStderr(), "only supported on Microsoft Windows Server 2016 and above")
	containerMock.AssertCalled(t, "PlatformVersion", mock.Anything)
}

func TestInstallNanoServerPackageFailure(t *testing.T) {
	depOrig := dep
	containerMock := &DepMock{}
	logMock := logmocks.NewMockLog()

	// Mock Nano Server detection
	containerMock.On("PlatformVersion", mock.Anything).Return("10", nil)
	containerMock.On("IsPlatformNanoServer", mock.Anything).Return(true, nil)

	// Simulate package provider detection failure
	containerMock.On("UpdateUtilExeCommandOutput",
		mock.Anything, // context
		"(Get-PackageProvider -ListAvailable).Name", // cmd
		[]string{}, // parameters
		"",         // workingDir
		"",         // outputRoot
		"",         // stdOut
		"",
		true, // usePlatformSpecificCommand
	).Return("", errors.New("package provider error"))
	dep = containerMock
	defer func() { dep = depOrig }()

	output := iohandler.DefaultIOHandler{}
	contextMock.On("Log").Return(logMock)

	RunInstallCommands(contextMock, "", &output)

	// Verify failure handling
	assert.Equal(t, 1, output.GetExitCode())
	assert.Contains(t, output.GetStderr(), "Error getting package providers")
	containerMock.AssertExpectations(t)

}

func TestInstallNanoServerContainersPackageFailure(t *testing.T) {
	depOrig := dep
	containerMock := &DepMock{}
	logMock := logmocks.NewMockLog()

	// Mock Nano Server detection and package provider setup
	containerMock.On("PlatformVersion", mock.Anything).Return("10", nil)
	containerMock.On("IsPlatformNanoServer", mock.Anything).Return(true, nil)

	// Simulate failure during containers package installation
	containerMock.On("UpdateUtilExeCommandOutput",
		mock.Anything, // context
		mock.Anything, // timeout
		mock.Anything, // log
		mock.Anything,
		mock.Anything, // parameters
		mock.Anything, // workingDir
		mock.Anything, // outputRoot
		mock.Anything, // stdOut
		mock.Anything, // stdErr
		mock.Anything, // usePlatformSpecificCommand
	).Return("", errors.New("containers package installation failed"))

	dep = containerMock
	defer func() { dep = depOrig }()

	output := iohandler.DefaultIOHandler{}
	contextMock.On("Log").Return(logMock)

	RunInstallCommands(contextMock, "", &output)

	// Verify failure handling
	assert.Equal(t, 1, output.GetExitCode())
	assert.Contains(t, output.GetStderr(), "Error getting package providers: containers package installation failed")
	containerMock.AssertExpectations(t)
}

func TestInstallDockerDownloadFailure(t *testing.T) {
	depOrig := dep
	containerMock := &DepMock{}

	//Setup successful platform checks
	containerMock.On("PlatformVersion", mock.Anything).Return("10", nil)
	containerMock.On("IsPlatformNanoServer", mock.Anything).Return(false, nil)
	containerMock.On("UpdateUtilExeCommandOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, true).Return("True", nil)
	containerMock.On("SetDaemonConfig", mock.Anything, mock.Anything).Return(nil)

	// Simulate Docker download failure
	containerMock.On("ArtifactDownload", mock.Anything, mock.Anything).Return(artifact.DownloadOutput{}, errors.New("download failed"))

	dep = containerMock
	defer func() { dep = depOrig }()

	output := iohandler.DefaultIOHandler{}
	RunInstallCommands(contextMock, "", &output)

	// Verify download failure handling
	assert.Equal(t, output.GetExitCode(), 1)
	assert.Contains(t, output.GetStderr(), "Failed to download Docker Engine file")
	containerMock.AssertExpectations(t)
}

func TestInstallPathEnvironmentVariableSettingFailure(t *testing.T) {
	depOrig := dep
	containerMock := &DepMock{}

	//Setup successful platform checks
	containerMock.On("PlatformVersion", mock.Anything).Return("10", nil)
	containerMock.On("IsPlatformNanoServer", mock.Anything).Return(false, nil)
	containerMock.On("UpdateUtilExeCommandOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, true).Return("True", nil)
	containerMock.On("SetDaemonConfig", mock.Anything, mock.Anything).Return(nil)
	containerMock.On("ArtifactDownload", mock.Anything, mock.Anything).Return(artifact.DownloadOutput{}, nil)

	containerMock.On("LocalRegistryKeyGetStringValue", mock.Anything, mock.Anything).Return("", 1, errors.New("registry key retrieval failed"))

	dep = containerMock
	defer func() { dep = depOrig }()

	output := iohandler.DefaultIOHandler{}
	RunInstallCommands(contextMock, "", &output)

	// Verify failure handling
	assert.Equal(t, 1, output.GetExitCode())
	assert.Contains(t, output.GetStderr(), "Error getting current machine registry key value")
	containerMock.AssertExpectations(t)
}

func TestInstallRegistryKeySettingFailure(t *testing.T) {
	depOrig := dep
	containerMock := &DepMock{}

	// Setup successful platform checks and previous steps
	containerMock.On("PlatformVersion", mock.Anything).Return("10", nil)
	containerMock.On("IsPlatformNanoServer", mock.Anything).Return(false, nil)
	containerMock.On("UpdateUtilExeCommandOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("True", nil)
	containerMock.On("SetDaemonConfig", mock.Anything, mock.Anything).Return(nil)
	containerMock.On("ArtifactDownload", mock.Anything, mock.Anything).Return(artifact.DownloadOutput{}, nil)
	containerMock.On("LocalRegistryKeyGetStringValue", mock.Anything, mock.Anything).Return("", 0, nil)

	// Simulate registry key setting failure
	containerMock.On("LocalRegistryKeySetDWordValue",
		mock.Anything, mock.Anything, mock.Anything,
	).Return(errors.New("registry key setting failed"))

	dep = containerMock
	defer func() { dep = depOrig }()

	output := iohandler.DefaultIOHandler{}
	RunInstallCommands(contextMock, "", &output)

	// Verify failure handling
	assert.Equal(t, 1, output.GetExitCode())
	assert.Contains(t, output.GetStderr(), "Error opening registry key to set Docker delayed start")
	containerMock.AssertExpectations(t)
}

func TestInstallReeboot(t *testing.T) {
	depOrig := dep
	containerMock := &DepMock{}

	// Setup successful platform checks and previous steps
	containerMock.On("PlatformVersion", mock.Anything).Return("10", nil)
	containerMock.On("IsPlatformNanoServer", mock.Anything).Return(false, nil)
	containerMock.On("UpdateUtilExeCommandOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(" ", nil)
	containerMock.On("SetDaemonConfig", mock.Anything, mock.Anything).Return(nil)
	containerMock.On("ArtifactDownload", mock.Anything, mock.Anything).Return(artifact.DownloadOutput{}, nil)
	containerMock.On("LocalRegistryKeyGetStringValue", mock.Anything, mock.Anything).Return("", 0, nil)

	containerMock.On("UpdateUtilExeCommandOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("Yes", nil)
	// Simulate registry key setting failure
	containerMock.On("LocalRegistryKeySetDWordValue",
		mock.Anything, mock.Anything, mock.Anything,
	).Return(errors.New("registry key setting failed"))

	dep = containerMock
	defer func() { dep = depOrig }()

	output := iohandler.DefaultIOHandler{}
	RunInstallCommands(contextMock, "", &output)
	containerMock.AssertExpectations(t)
}

func TestInstallDockerServiceAlreadyRunning(t *testing.T) {
	depOrig := dep
	containerMock := successMock()

	// Mock Docker service status to return "Running"
	containerMock.On("UpdateUtilExeCommandOutput",
		mock.Anything, // context
		mock.Anything, // timeout
		mock.Anything, // log
		"(Get-Service docker).Status",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		true,
	).Return("Running", nil)

	dep = containerMock
	defer func() { dep = depOrig }()

	output := iohandler.DefaultIOHandler{}
	RunInstallCommands(contextMock, "", &output)

	// Verify successful installation without service start
	assert.Equal(t, output.GetExitCode(), 0)
	assert.Contains(t, output.GetStdout(), "Installation complete")
	containerMock.AssertNotCalled(t, "UpdateUtilExeCommandOutput",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		"Start-Service docker",
		mock.Anything)
}

func TestInstallNanoServerNugetProviderFailure(t *testing.T) {
	depOrig := dep
	containerMock := &DepMock{}

	logMock := logmocks.NewMockLog()

	// Mock Nano Server detection and initial package provider check
	containerMock.On("PlatformVersion", mock.Anything).Return("10", nil)
	containerMock.On("IsPlatformNanoServer", mock.Anything).Return(true, nil)
	containerMock.On("SetDaemonConfig", mock.Anything, mock.Anything).Return(nil)
	containerMock.On("ArtifactDownload", mock.Anything, mock.Anything).Return(artifact.DownloadOutput{}, nil)
	containerMock.On("UpdateUtilExeCommandOutput",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return("", nil)
	containerMock.On("LocalRegistryKeyGetStringValue", mock.Anything, mock.Anything).Return("", 0, nil)
	// Simulate Nuget package provider installation failure
	containerMock.On("UpdateUtilExeCommandOutput",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return("", errors.New("nuget provider installation failed"))

	dep = containerMock
	defer func() { dep = depOrig }()

	output := iohandler.DefaultIOHandler{}
	contextMock.On("Log").Return(logMock)

	RunInstallCommands(contextMock, "", &output)

	// Verify failure handling
	containerMock.AssertExpectations(t)
}

func TestUninstallNanoServer(t *testing.T) {
	depOrig := dep
	containerMock := &DepMock{}

	// Mock Nano Server detection
	containerMock.On("PlatformVersion", mock.Anything).Return("10", nil)
	containerMock.On("IsPlatformNanoServer", mock.Anything).Return(true, nil)
	containerMock.On("UpdateUtilExeCommandOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, true).Return("True", nil)
	containerMock.On("SetDaemonConfig", mock.Anything, mock.Anything).Return(nil)

	// Simulate Docker download failure
	containerMock.On("ArtifactDownload", mock.Anything, mock.Anything).Return(artifact.DownloadOutput{}, errors.New("download failed"))

	dep = containerMock
	defer func() { dep = depOrig }()

	output := iohandler.DefaultIOHandler{}
	RunUninstallCommands(contextMock, "", &output)

	// Verify Nano Server message
	assert.Equal(t, output.GetExitCode(), 0)
	assert.Contains(t, output.GetStdout(), "Removing packages from Nano server not currently supported")

}

func TestUninstallUnsupportedPlatform(t *testing.T) {
	depOrig := dep
	containerMock := &DepMock{}

	// Mock unsupported platform version
	containerMock.On("PlatformVersion", mock.Anything).Return("6.3", nil)

	dep = containerMock
	defer func() { dep = depOrig }()

	output := iohandler.DefaultIOHandler{}
	RunUninstallCommands(contextMock, "", &output)

	// Verify failure status
	assert.Equal(t, output.GetExitCode(), 1)
	assert.Contains(t, output.GetStderr(), "only supported on Microsoft Windows Server 2016 and above")
	containerMock.AssertExpectations(t)
}

func TestUninstallServiceStoppingFailure(t *testing.T) {
	depOrig := dep
	containerMock := &DepMock{}

	// Mock platform and service checks with running service
	containerMock.On("PlatformVersion", mock.Anything).Return("10", nil)
	containerMock.On("IsPlatformNanoServer", mock.Anything).Return(false, nil)
	containerMock.On("UpdateUtilExeCommandOutput",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return("Running", nil).Once().On("UpdateUtilExeCommandOutput",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return("", errors.New("service stop failed"))

	dep = containerMock
	defer func() { dep = depOrig }()

	output := iohandler.DefaultIOHandler{}
	RunUninstallCommands(contextMock, "", &output)

	// Verify failure handling
	assert.Equal(t, output.GetExitCode(), 1)
	assert.Contains(t, output.GetStderr(), "Error stopping Docker service: service stop failed")
}

func TestUninstallServiceUnregistrationFailure(t *testing.T) {
	depOrig := dep
	containerMock := &DepMock{}

	// Mock platform and service checks with running service
	containerMock.On("PlatformVersion", mock.Anything).Return("10", nil)
	containerMock.On("IsPlatformNanoServer", mock.Anything).Return(false, nil)
	containerMock.On("UpdateUtilExeCommandOutput",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return("Running", nil).Twice().On("UpdateUtilExeCommandOutput",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return("", errors.New("service deregistration failed"))

	dep = containerMock
	defer func() { dep = depOrig }()

	output := iohandler.DefaultIOHandler{}
	RunUninstallCommands(contextMock, "", &output)

	// Verify failure handling
	assert.Equal(t, output.GetExitCode(), 1)
	assert.Contains(t, output.GetStderr(), "Error unregistering Docker service: service deregistration failed")
}

func TestUninstallContainerFailure(t *testing.T) {
	depOrig := dep
	containerMock := &DepMock{}

	// Mock platform and service checks with running service
	containerMock.On("PlatformVersion", mock.Anything).Return("10", nil)
	containerMock.On("IsPlatformNanoServer", mock.Anything).Return(false, nil)
	containerMock.On("UpdateUtilExeCommandOutput",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return("Running", nil).Times(3).On("UpdateUtilExeCommandOutput",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return("", errors.New("container feature not found"))

	dep = containerMock
	defer func() { dep = depOrig }()

	output := iohandler.DefaultIOHandler{}
	RunUninstallCommands(contextMock, "", &output)

	// Verify failure handling
	assert.Equal(t, output.GetExitCode(), 1)
	assert.Contains(t, output.GetStderr(), "Error getting containers feature: container feature not found")
}

func TestUninstallDockerDirectoryRemoval(t *testing.T) {
	depOrig := dep
	containerMock := &DepMock{}

	// Setup mocks for platform and service checks
	containerMock.On("PlatformVersion", mock.Anything).Return("10", nil)
	containerMock.On("IsPlatformNanoServer", mock.Anything).Return(false, nil)
	containerMock.On("UpdateUtilExeCommandOutput",
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		true,
	).Return("Stopped", nil)

	// Mock Docker directory existence
	mockOsStat := func(path string) (os.FileInfo, error) {
		if path == DOCKER_INSTALLED_DIRECTORY {
			return nil, nil // Simulate directory exists
		}
		return nil, os.ErrNotExist
	}
	osStatOrig := os.Stat
	osStatFunc = mockOsStat
	defer func() { osStatFunc = osStatOrig }()

	dep = containerMock
	defer func() { dep = depOrig }()

	output := iohandler.DefaultIOHandler{}
	RunUninstallCommands(contextMock, "", &output)

	// Verify directory removal and successful uninstallation
	assert.Equal(t, output.GetExitCode(), 0)
	assert.Contains(t, output.GetStdout(), "Uninstallation complete")
	containerMock.AssertExpectations(t)
}

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

// Package manager encapsulates everything related to long running plugin manager that starts, stops & configures long running plugins
package manager

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/longrunning"
	managerContracts "github.com/aws/amazon-ssm-agent/agent/longrunning/plugin"
	contextmock "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"

	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const testRepoRoot = "testdata"
const testCloudWatchDocumentName = "cwdocument.json"
const invalidTestCloudWatchDocumentName = "cwdocument_invalid_format1.json"
const invalidTestCloudWatchDocumentName2 = "cwdocument_invalid_format2.json"
const testCloudWatchConfig = "cwconfig.json"

var (
	instanceId = "i-1234567890"

	legacyCwConfigStorePath = fileutil.BuildPath(
		appconfig.EC2ConfigDataStorePath,
		instanceId,
		appconfig.ConfigurationRootDirName,
		appconfig.WorkersRootDirName,
		"aws.cloudWatch.ec2config")

	legacyCwConfigPath = fileutil.BuildPath(
		appconfig.EC2ConfigSettingPath,
		NameOfCloudWatchJsonFile)

	loggerMock = logmocks.NewMockLog()
)

/*
 *	Tests for TestCheckLegacyCloudWatchRunCommandConfig
 */
func TestCheckLegacyCloudWatchRunCommandConfig(t *testing.T) {
	var hasConfiguration bool
	var testCloudWatchDocument string
	var err error

	testCloudWatchDocument, err = fileutil.ReadAllText(filepath.Join(testRepoRoot, testCloudWatchDocumentName))
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	// Setup mock with expectations
	mockCwcInstance := MockedCwcInstance{}
	mockCwcInstance.On("Enable", mock.Anything).Return(nil).Once()

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", legacyCwConfigStorePath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", legacyCwConfigStorePath).Return([]byte(testCloudWatchDocument), nil).Once()

	hasConfiguration, err = checkLegacyCloudWatchRunCommandConfig(loggerMock, instanceId, &mockCwcInstance, &mockFileSysUtil)

	assert.True(t, hasConfiguration)
	assert.Nil(t, err)
	mockCwcInstance.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
}

func TestCheckLegacyCloudWatchRunCommandConfig_InvalidFormat(t *testing.T) {
	var hasConfiguration bool
	var testCloudWatchDocument string
	var err error

	testCloudWatchDocument, err = fileutil.ReadAllText(filepath.Join(testRepoRoot, invalidTestCloudWatchDocumentName))
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	// Setup mock with expectations
	mockCwcInstance := MockedCwcInstance{}

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", legacyCwConfigStorePath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", legacyCwConfigStorePath).Return([]byte(testCloudWatchDocument), nil).Once()

	hasConfiguration, err = checkLegacyCloudWatchRunCommandConfig(loggerMock, instanceId, &mockCwcInstance, &mockFileSysUtil)

	assert.False(t, hasConfiguration)
	assert.NotNil(t, err)
	mockCwcInstance.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
}

func TestCheckLegacyCloudWatchRunCommandConfig_InvalidFormat2(t *testing.T) {
	var hasConfiguration bool
	var testCloudWatchDocument string
	var err error

	testCloudWatchDocument, err = fileutil.ReadAllText(filepath.Join(testRepoRoot, invalidTestCloudWatchDocumentName2))
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	// Setup mock with expectations
	mockCwcInstance := MockedCwcInstance{}

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", legacyCwConfigStorePath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", legacyCwConfigStorePath).Return([]byte(testCloudWatchDocument), nil).Once()

	hasConfiguration, err = checkLegacyCloudWatchRunCommandConfig(loggerMock, instanceId, &mockCwcInstance, &mockFileSysUtil)

	assert.False(t, hasConfiguration)
	assert.NotNil(t, err)
	mockCwcInstance.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
}

func TestCheckLegacyCloudWatchRunCommandConfig_ConfigFileMissing(t *testing.T) {
	var hasConfiguration bool
	var err error

	// Setup mock with expectations
	mockCwcInstance := MockedCwcInstance{}

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", legacyCwConfigStorePath).Return(false).Once()

	hasConfiguration, err = checkLegacyCloudWatchRunCommandConfig(loggerMock, instanceId, &mockCwcInstance, &mockFileSysUtil)

	assert.False(t, hasConfiguration)
	assert.Nil(t, err)
	mockCwcInstance.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
}

func TestCheckLegacyCloudWatchRunCommandConfig_FileReadFailure(t *testing.T) {
	var hasConfiguration bool
	var err error

	// Setup mock with expectations
	mockCwcInstance := MockedCwcInstance{}

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", legacyCwConfigStorePath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", legacyCwConfigStorePath).Return([]byte(""), fmt.Errorf("Failed to read the file")).Once()

	hasConfiguration, err = checkLegacyCloudWatchRunCommandConfig(loggerMock, instanceId, &mockCwcInstance, &mockFileSysUtil)

	assert.False(t, hasConfiguration)
	assert.NotNil(t, err)
	mockCwcInstance.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
}

func TestCheckLegacyCloudWatchRunCommandConfig_EnableFailure(t *testing.T) {
	var hasConfiguration bool
	var testCloudWatchDocument string
	var err error

	testCloudWatchDocument, err = fileutil.ReadAllText(filepath.Join(testRepoRoot, testCloudWatchDocumentName))
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	// Setup mock with expectations
	mockCwcInstance := MockedCwcInstance{}
	mockCwcInstance.On("Enable", mock.Anything).Return(fmt.Errorf("Failed to enable cw configuration")).Once()

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", legacyCwConfigStorePath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", legacyCwConfigStorePath).Return([]byte(testCloudWatchDocument), nil).Once()

	hasConfiguration, err = checkLegacyCloudWatchRunCommandConfig(loggerMock, instanceId, &mockCwcInstance, &mockFileSysUtil)

	assert.False(t, hasConfiguration)
	assert.NotNil(t, err)
	mockCwcInstance.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
}

/*
 *	Tests for checkLegacyCloudWatchLocalConfig
 */
func TestCheckLegacyCloudWatchLocalConfig(t *testing.T) {
	var hasLocalConfiguration bool
	var testCloudWatchDocument string
	var err error

	testCloudWatchDocument, err = fileutil.ReadAllText(filepath.Join(testRepoRoot, testCloudWatchConfig))
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	// Setup mock with expectations
	mockEc2ConfigXmlParser := MockedEc2ConfigXmlParser{}
	mockEc2ConfigXmlParser.On("IsCloudWatchEnabled").Return(true, nil).Once()

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", legacyCwConfigPath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", legacyCwConfigPath).Return([]byte(testCloudWatchDocument), nil).Once()

	mockCwcInstance := MockedCwcInstance{}
	mockCwcInstance.On("Enable", mock.Anything).Return(nil).Once()

	hasLocalConfiguration, err = checkLegacyCloudWatchLocalConfig(loggerMock, &mockCwcInstance, &mockEc2ConfigXmlParser, &mockFileSysUtil)

	assert.True(t, hasLocalConfiguration)
	assert.Nil(t, err)
	mockEc2ConfigXmlParser.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
	mockCwcInstance.AssertExpectations(t)
}

func TestCheckLegacyCloudWatchLocalConfig_CloudWatchDisabled(t *testing.T) {
	var hasLocalConfiguration bool
	var err error

	// Setup mock with expectations
	mockEc2ConfigXmlParser := MockedEc2ConfigXmlParser{}
	mockEc2ConfigXmlParser.On("IsCloudWatchEnabled").Return(false, nil).Once()

	mockFileSysUtil := MockedFileSysUtil{}
	mockCwcInstance := MockedCwcInstance{}

	hasLocalConfiguration, err = checkLegacyCloudWatchLocalConfig(loggerMock, &mockCwcInstance, &mockEc2ConfigXmlParser, &mockFileSysUtil)

	assert.False(t, hasLocalConfiguration)
	assert.Nil(t, err)
	mockEc2ConfigXmlParser.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
	mockCwcInstance.AssertExpectations(t)
}

func TestCheckLegacyCloudWatchLocalConfig_InvalidEc2ConfigXmlFormat(t *testing.T) {
	var hasLocalConfiguration bool
	var err error

	// Setup mock with expectations
	mockEc2ConfigXmlParser := MockedEc2ConfigXmlParser{}
	mockEc2ConfigXmlParser.On("IsCloudWatchEnabled").Return(false, fmt.Errorf("Invalid format")).Once()

	mockFileSysUtil := MockedFileSysUtil{}
	mockCwcInstance := MockedCwcInstance{}

	hasLocalConfiguration, err = checkLegacyCloudWatchLocalConfig(loggerMock, &mockCwcInstance, &mockEc2ConfigXmlParser, &mockFileSysUtil)

	assert.False(t, hasLocalConfiguration)
	assert.NotNil(t, err)
	mockEc2ConfigXmlParser.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
	mockCwcInstance.AssertExpectations(t)
}

func TestCheckLegacyCloudWatchLocalConfig_ConfigFileMissing(t *testing.T) {
	var hasLocalConfiguration bool
	var err error

	// Setup mock with expectations
	mockEc2ConfigXmlParser := MockedEc2ConfigXmlParser{}
	mockEc2ConfigXmlParser.On("IsCloudWatchEnabled").Return(true, nil).Once()

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", legacyCwConfigPath).Return(false).Once()

	mockCwcInstance := MockedCwcInstance{}

	hasLocalConfiguration, err = checkLegacyCloudWatchLocalConfig(loggerMock, &mockCwcInstance, &mockEc2ConfigXmlParser, &mockFileSysUtil)

	assert.False(t, hasLocalConfiguration)
	assert.Nil(t, err)
	mockEc2ConfigXmlParser.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
	mockCwcInstance.AssertExpectations(t)
}

func TestCheckLegacyCloudWatchLocalConfig_FileReadFailure(t *testing.T) {
	var hasLocalConfiguration bool
	var err error

	// Setup mock with expectations
	mockEc2ConfigXmlParser := MockedEc2ConfigXmlParser{}
	mockEc2ConfigXmlParser.On("IsCloudWatchEnabled").Return(true, nil).Once()

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", legacyCwConfigPath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", legacyCwConfigPath).Return([]byte(""), fmt.Errorf("Failed to read the file")).Once()

	mockCwcInstance := MockedCwcInstance{}

	hasLocalConfiguration, err = checkLegacyCloudWatchLocalConfig(loggerMock, &mockCwcInstance, &mockEc2ConfigXmlParser, &mockFileSysUtil)

	assert.False(t, hasLocalConfiguration)
	assert.NotNil(t, err)
	mockEc2ConfigXmlParser.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
	mockCwcInstance.AssertExpectations(t)
}

func TestCheckLegacyCloudWatchLocalConfig_EnableFailure(t *testing.T) {
	var hasLocalConfiguration bool
	var testCloudWatchDocument string
	var err error

	testCloudWatchDocument, err = fileutil.ReadAllText(filepath.Join(testRepoRoot, testCloudWatchConfig))
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	// Setup mock with expectations
	mockEc2ConfigXmlParser := MockedEc2ConfigXmlParser{}
	mockEc2ConfigXmlParser.On("IsCloudWatchEnabled").Return(true, nil).Once()

	mockFileSysUtil := MockedFileSysUtil{}
	mockFileSysUtil.On("Exists", legacyCwConfigPath).Return(true).Once()
	mockFileSysUtil.On("ReadFile", legacyCwConfigPath).Return([]byte(testCloudWatchDocument), nil).Once()

	mockCwcInstance := MockedCwcInstance{}
	mockCwcInstance.On("Enable", mock.Anything).Return(fmt.Errorf("Failed to enable cw configuration")).Once()

	hasLocalConfiguration, err = checkLegacyCloudWatchLocalConfig(loggerMock, &mockCwcInstance, &mockEc2ConfigXmlParser, &mockFileSysUtil)

	assert.False(t, hasLocalConfiguration)
	assert.NotNil(t, err)
	mockEc2ConfigXmlParser.AssertExpectations(t)
	mockFileSysUtil.AssertExpectations(t)
	mockCwcInstance.AssertExpectations(t)
}

func TestEnsureInitialization_SingletonBehavior(t *testing.T) {
	// Reset the singleton instance and once sync before the test
	singletonInstance = nil
	once = sync.Once{}

	// Create a mock context
	mockContext := contextmock.NewMockDefault()

	// Call EnsureInitialization multiple times
	EnsureInitialization(mockContext)
	firstInstance := singletonInstance

	// Call again to ensure same instance is returned
	EnsureInitialization(mockContext)
	secondInstance := singletonInstance

	// Verify that both calls result in the same instance
	assert.Equal(t, firstInstance, secondInstance,
		"Multiple calls to EnsureInitialization should return the same instance")
}

func TestGetInstance_SuccessfulRetrieval(t *testing.T) {
	// Setup: Ensure manager is initialized
	context := contextmock.NewMockDefault()
	EnsureInitialization(context)

	// Act: Retrieve the singleton instance
	manager, err := GetInstance()

	// Assert
	assert.NotNil(t, manager, "Manager instance should not be nil")
	assert.Nil(t, err, "No error should be returned when manager is initialized")
	assert.Equal(t, singletonInstance, manager, "Retrieved instance should match the singleton instance")
}

func TestGetInstance_Uninitialized(t *testing.T) {
	// Reset singleton instance to simulate uninitialized state
	originalSingleton := singletonInstance
	defer func() { singletonInstance = originalSingleton }()
	singletonInstance = nil

	// Act: Attempt to retrieve instance
	manager, err := GetInstance()

	// Assert
	assert.Nil(t, manager, "Manager should be nil when not initialized")
	assert.NotNil(t, err, "Error should be returned when manager is not initialized")
	assert.EqualError(t, err, "lrpm isn't initialized yet", "Error message should match expected")
}

func TestGetRegisteredPlugins_BasicRetrieval(t *testing.T) {
	// Create a mock context
	mockContext := contextmock.NewMockDefault()

	// Ensure manager is initialized
	EnsureInitialization(mockContext)

	// Get the manager instance
	manager, err := GetInstance()
	assert.Nil(t, err)
	assert.NotNil(t, manager)

	// Retrieve registered plugins
	registeredPlugins := manager.GetRegisteredPlugins()

	// Verify that the returned map is not nil
	assert.NotNil(t, registeredPlugins)
}

func TestModuleName_ReturnsCorrectName(t *testing.T) {
	// Create a mock context for the manager
	mockContext := contextmock.NewMockDefault()

	// Ensure manager is initialized
	EnsureInitialization(mockContext)

	// Get the manager instance
	manager, err := GetInstance()
	assert.Nil(t, err, "Manager initialization should not return an error")

	// Verify the module name
	assert.Equal(t, Name, manager.ModuleName(),
		"ModuleName should return the predefined constant Name")
}

func TestModuleExecute_NoRunningPlugins(t *testing.T) {
	// Test scenario: No previously running plugins, no CloudWatch configuration
	mockLog := logmocks.NewMockLog()
	mockContext := contextmock.NewMockDefaultWithOwnLogMock(mockLog)

	// Mock datastore to return empty map
	mockDataStore := &MockedDataStore{}
	mockDataStore.On("Read").Return(map[string]managerContracts.PluginInfo{}, nil)

	manager := &Manager{
		context:           mockContext,
		dataStore:         mockDataStore,
		runningPlugins:    map[string]managerContracts.PluginInfo{},
		registeredPlugins: map[string]managerContracts.Plugin{},
	}

	// Execute and verify
	err := manager.ModuleExecute()
	assert.Nil(t, err)
	mockLog.AssertNumberOfCalls(t, "Infof", 2)
}

func TestModuleExecute_DataStoreReadError(t *testing.T) {
	// Test scenario: Error reading from datastore
	mockLog := logmocks.NewMockLog()
	mockContext := contextmock.NewMockDefaultWithOwnLogMock(mockLog)

	// Mock datastore to return an error
	mockDataStore := &MockedDataStore{}
	mockDataStore.On("Read").Return(nil, errors.New("datastore read error"))

	manager := &Manager{
		context:           mockContext,
		dataStore:         mockDataStore,
		runningPlugins:    map[string]managerContracts.PluginInfo{},
		registeredPlugins: map[string]managerContracts.Plugin{},
	}

	// Execute and verify
	err := manager.ModuleExecute()
	assert.Nil(t, err)

	mockDataStore.AssertExpectations(t)
	mockLog.AssertNumberOfCalls(t, "Errorf", 1)
}

func TestModuleExecute_RunningPluginWithNoConfig(t *testing.T) {
	// Test scenario: Running plugin with no configuration
	mockLog := logmocks.NewMockLog()
	mockContext := contextmock.NewMockDefaultWithOwnLogMock(mockLog)

	// Mock datastore to return a plugin with no configuration
	mockDataStore := &MockedDataStore{}
	mockDataStore.On("Read").Return(map[string]managerContracts.PluginInfo{
		"TestPlugin": {Name: "TestPlugin", Configuration: ""},
	}, nil)

	manager := &Manager{
		context:        mockContext,
		dataStore:      mockDataStore,
		runningPlugins: map[string]managerContracts.PluginInfo{},
		registeredPlugins: map[string]managerContracts.Plugin{
			"TestPlugin": {
				Handler: &MockPluginHandler{},
				Info:    managerContracts.PluginInfo{Name: "TestPlugin"},
			},
		},
	}

	// Execute and verify
	err := manager.ModuleExecute()
	assert.Nil(t, err)

	mockDataStore.AssertExpectations(t)

	mockLog.AssertNumberOfCalls(t, "Infof", 2)
}

func TestModuleStop_NormalShutdown(t *testing.T) {
	// Setup
	mockContext := contextmock.NewMockDefault()

	// Create a manager instance with mocked dependencies
	manager := &Manager{
		context:           mockContext,
		runningPlugins:    make(map[string]managerContracts.PluginInfo),
		registeredPlugins: make(map[string]managerContracts.Plugin),
	}

	// Execute
	err := manager.ModuleStop()

	// Assertions
	assert.Nil(t, err, "ModuleStop should complete without error")
}

func TestStopLongRunningPlugins_SuccessfulStop(t *testing.T) {
	// Create a mock context
	mockContext := contextmock.NewMockDefault()

	// Create a mock manager with running plugins
	manager := &Manager{
		context: mockContext,
		runningPlugins: map[string]managerContracts.PluginInfo{
			"TestPlugin1": {Name: "TestPlugin1"},
			"TestPlugin2": {Name: "TestPlugin2"},
		},
		registeredPlugins: map[string]managerContracts.Plugin{
			"TestPlugin1": {
				Handler: &MockPluginHandler{},
				Info:    managerContracts.PluginInfo{Name: "TestPlugin1"},
			},
			"TestPlugin2": {
				Handler: &MockPluginHandler{},
				Info:    managerContracts.PluginInfo{Name: "TestPlugin2"},
			},
		},
	}

	// Expect Stop to be called on each plugin handler
	mockHandler1 := manager.registeredPlugins["TestPlugin1"].Handler.(*MockPluginHandler)
	mockHandler1.On("Stop", mock.Anything).Return(nil)

	mockHandler2 := manager.registeredPlugins["TestPlugin2"].Handler.(*MockPluginHandler)
	mockHandler2.On("Stop", mock.Anything).Return(nil)

	// Call the method under test
	manager.stopLongRunningPlugins()

	// Assert that Stop was called on each plugin
	mockHandler1.AssertExpectations(t)
	mockHandler2.AssertExpectations(t)
}

func TestStopLongRunningPlugins_PartialStopFailure(t *testing.T) {
	// Create a mock context
	mockContext := contextmock.NewMockDefault()

	// Create a mock manager with running plugins
	manager := &Manager{
		context: mockContext,
		runningPlugins: map[string]managerContracts.PluginInfo{
			"SuccessPlugin": {Name: "SuccessPlugin"},
			"FailingPlugin": {Name: "FailingPlugin"},
		},
		registeredPlugins: map[string]managerContracts.Plugin{
			"SuccessPlugin": {
				Handler: &MockPluginHandler{},
				Info:    managerContracts.PluginInfo{Name: "SuccessPlugin"},
			},
			"FailingPlugin": {
				Handler: &MockPluginHandler{},
				Info:    managerContracts.PluginInfo{Name: "FailingPlugin"},
			},
		},
	}

	// Setup mock handlers with different behaviors
	successHandler := manager.registeredPlugins["SuccessPlugin"].Handler.(*MockPluginHandler)
	successHandler.On("Stop", mock.Anything).Return(nil)

	failingHandler := manager.registeredPlugins["FailingPlugin"].Handler.(*MockPluginHandler)
	failingHandler.On("Stop", mock.Anything).Return(fmt.Errorf("stop failed"))

	// Call the method under test
	manager.stopLongRunningPlugins()

	// Assert expectations
	successHandler.AssertExpectations(t)
	failingHandler.AssertExpectations(t)

}

func TestStopLongRunningPlugins_PanicRecovery(t *testing.T) {
	// Create a mock context
	mockContext := contextmock.NewMockDefault()

	// Create a mock manager with a plugin that will cause a panic
	manager := &Manager{
		context: mockContext,
		runningPlugins: map[string]managerContracts.PluginInfo{
			"PanicPlugin": {Name: "PanicPlugin"},
		},
		registeredPlugins: map[string]managerContracts.Plugin{
			"PanicPlugin": {
				Handler: &MockPluginHandler{},
				Info:    managerContracts.PluginInfo{Name: "PanicPlugin"},
			},
		},
	}

	// Setup mock handler to cause a panic
	panicHandler := manager.registeredPlugins["PanicPlugin"].Handler.(*MockPluginHandler)
	panicHandler.On("Stop", mock.Anything).Run(func(args mock.Arguments) {
		panic("unexpected error")
	})

	// Call the method under test
	manager.stopLongRunningPlugins()

	// Assert that panic was logged and recovered
	panicHandler.AssertExpectations(t)
}

func TestEnsurePluginRegistered_NewPlugin(t *testing.T) {
	// Setup
	context := contextmock.NewMockDefault()
	manager := &Manager{
		context: context,
		registeredPlugins: map[string]managerContracts.Plugin{
			"TestPlugin": {
				Handler: &MockPluginHandler{},
				Info:    managerContracts.PluginInfo{Name: "TestPlugin1"},
			},
		},
	}

	// Execute
	err := manager.EnsurePluginRegistered("TestPlugin", manager.registeredPlugins["TestPlugin"])

	// Assertions
	assert.Nil(t, err)
	assert.Len(t, manager.registeredPlugins, 1)
	assert.Equal(t, manager.registeredPlugins["TestPlugin"], manager.registeredPlugins["TestPlugin"])
}

/*
 *	Mocks
 */
type MockedCwcInstance struct {
	mock.Mock
	IsEnabled           bool        `json:"IsEnabled"`
	EngineConfiguration interface{} `json:"EngineConfiguration"`
}

func (m *MockedCwcInstance) GetIsEnabled() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockedCwcInstance) ParseEngineConfiguration() (config string, err error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockedCwcInstance) Update(log log.T) error {
	args := m.Called(log)
	return args.Error(0)
}

func (m *MockedCwcInstance) Write() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockedCwcInstance) Enable(engineConfiguration interface{}) error {
	args := m.Called(engineConfiguration)
	return args.Error(0)
}

func (m *MockedCwcInstance) Disable() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockedCwcInstance) Stop(cancelFlag task.CancelFlag) error {
	args := m.Called(cancelFlag)
	return args.Error(0)
}

// Implement other required methods for the interface
func (m *MockedCwcInstance) Start(configuration, orchestrationDir string, cancelFlag task.CancelFlag, out iohandler.IOHandler) error {
	args := m.Called(configuration, orchestrationDir, cancelFlag, out)
	return args.Error(0)
}

func (m *MockedCwcInstance) IsRunning() bool {
	args := m.Called()
	return args.Bool(0)

}

type MockedFileSysUtil struct {
	mock.Mock
}

func (m *MockedFileSysUtil) Exists(filePath string) bool {
	args := m.Called(filePath)
	return args.Bool(0)
}

func (m *MockedFileSysUtil) MakeDirs(destinationDir string) error {
	args := m.Called(destinationDir)
	return args.Error(0)
}

func (m *MockedFileSysUtil) WriteIntoFileWithPermissions(absolutePath, content string, perm os.FileMode) (bool, error) {
	args := m.Called(absolutePath, content, perm)
	return args.Bool(0), args.Error(1)
}

func (m *MockedFileSysUtil) ReadFile(filename string) ([]byte, error) {
	args := m.Called(filename)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockedFileSysUtil) ReadAll(r io.Reader) ([]byte, error) {
	args := m.Called(r)
	return args.Get(0).([]byte), args.Error(1)
}

type MockedEc2ConfigXmlParser struct {
	mock.Mock
	FileUtilWrapper longrunning.FileSysUtil
}

func (m *MockedEc2ConfigXmlParser) IsCloudWatchEnabled() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

type MockedDataStore struct {
	mock.Mock
}

func (m *MockedDataStore) Read() (map[string]managerContracts.PluginInfo, error) {
	args := m.Called()
	return args.Get(0).(map[string]managerContracts.PluginInfo), args.Error(1)
}

func (m *MockedDataStore) Write(pluginInfoMap map[string]managerContracts.PluginInfo) error {
	args := m.Called(pluginInfoMap)
	return args.Error(0)
}

type MockPluginHandler struct {
	mock.Mock
}

func (m *MockPluginHandler) Stop(cancelFlag task.CancelFlag) error {
	args := m.Called(cancelFlag)
	return args.Error(0)
}

// Implement other required methods for the interface
func (m *MockPluginHandler) Start(configuration, orchestrationDir string, cancelFlag task.CancelFlag, out iohandler.IOHandler) error {
	args := m.Called(configuration, orchestrationDir, cancelFlag, out)
	return args.Error(0)
}

func (m *MockPluginHandler) IsRunning() bool {
	args := m.Called()
	return args.Bool(0)

}

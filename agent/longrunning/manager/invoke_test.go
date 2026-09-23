package manager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/contracts"

	iohandlermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/mock"
	coreModule "github.com/aws/amazon-ssm-agent/agent/longrunning/mocks/manager"
	managerContracts "github.com/aws/amazon-ssm-agent/agent/longrunning/plugin"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	taskmocks "github.com/aws/amazon-ssm-agent/agent/mocks/task"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestEnablePluginSuccessfulStart(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-orchestration-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	// Clean up after the test
	defer os.RemoveAll(tempDir)

	// Create the plugin-specific directory
	pluginDir := filepath.Join(tempDir, "awscloudWatch")
	err = os.MkdirAll(pluginDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create plugin dir: %v", err)
	}

	// Setup mocks
	mockContext := context.NewMockDefault()
	mockLRPM := coreModule.NewMockDefault()
	mockCancelFlag := taskmocks.NewMockDefault()
	mockResult := &contracts.PluginResult{}
	ioHandler := &iohandlermocks.MockIOHandler{}

	// Configure expectations
	mockContext.On("Log").Return(mockContext.Log())
	mockLRPM.On("StopPlugin", lrpName, mock.Anything).Return(nil)
	mockLRPM.On("StartPlugin",
		lrpName,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.Anything,
		mock.Anything,
	).Return(nil)

	// Mock IOHandler
	ioHandler.On("NewDefaultIOHandler", mock.Anything, mock.Anything).Return(ioHandler)
	ioHandler.On("Init", mock.Anything).Return(nil)
	ioHandler.On("GetStderr").Return("")
	ioHandler.On("Close").Return()

	// Prepare test inputs
	pluginID := "testPlugin"
	property := `{"key": "value"}`

	// Execute the function
	enablePlugin(
		mockContext,
		tempDir,
		pluginID,
		mockLRPM,
		mockCancelFlag,
		property,
		mockResult,
	)

	// Assertions
	assert.Equal(t, contracts.ResultStatusSuccess, mockResult.Status)
	assert.Equal(t, "success", mockResult.Output)
}

func TestInvoke_Invoke_PluginNotRegistered(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-orchestration-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	// Clean up after the test
	defer os.RemoveAll(tempDir)

	// Create the plugin-specific directory
	pluginDir := filepath.Join(tempDir, "awscloudWatch")
	err = os.MkdirAll(pluginDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create plugin dir: %v", err)
	}
	// Setup
	mockContext := context.NewMockDefault()
	originalGetInstance := getInstance
	mockPluginManager := &Manager{
		context: mockContext,
	}

	defer func() { getInstance = originalGetInstance }()
	getInstance = func() (*Manager, error) {
		return mockPluginManager, nil
	}

	// Prepare plugin result
	pluginResult := &contracts.PluginResult{
		StandardOutput: "Enabled",
		Output:         `{"test": "config"}`,
	}

	// Execute
	Invoke(mockContext, "testPluginID", pluginResult, tempDir)

	// Assertions
	assert.Equal(t, contracts.ResultStatusFailed, pluginResult.Status)

}

func TestInvoke_Invoke_PluginRegistered(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-orchestration-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	// Clean up after the test
	defer os.RemoveAll(tempDir)

	// Create the plugin-specific directory
	pluginDir := filepath.Join(tempDir, "awscloudWatch")
	err = os.MkdirAll(pluginDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create plugin dir: %v", err)
	}
	// Setup
	mockInstance := &MockedCwcInstance{IsEnabled: true}
	mockContext := context.NewMockDefault()

	mockPluginManager := &Manager{
		context:     mockContext,
		startPlugin: &taskmocks.MockedPool{},
		registeredPlugins: map[string]managerContracts.Plugin{
			lrpName: {
				Handler: mockInstance,
			},
		},
		runningPlugins: make(map[string]managerContracts.PluginInfo),
		fileSysUtil:    &MockedFileSysUtil{},
		dataStore:      &MockedDataStore{},
	}

	mockInstance.On("Start", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mockInstance.On("Instance").Return(mockInstance)

	mockInstance.On("Enable", mock.Anything).Return(nil)

	datastoreHandler := mockPluginManager.dataStore.(*MockedDataStore)
	datastoreHandler.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	originalGetInstance := getInstance
	defer func() { getInstance = originalGetInstance }()
	getInstance = func() (*Manager, error) {
		return mockPluginManager, nil
	}
	originalInstance := instance
	defer func() { instance = originalInstance }()
	instance = mockInstance

	// 	// Prepare plugin result
	pluginResult := &contracts.PluginResult{
		StandardOutput: "Enabled",
		Output:         `{"test": "config"}`,
	}

	// Execute
	Invoke(mockContext, "testPluginID", pluginResult, tempDir)

	// Assertions
	assert.Equal(t, contracts.ResultStatusSuccess, pluginResult.Status)
}

func TestInvoke_Invoke_PluginNotEnabled(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-orchestration-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	// Clean up after the test
	defer os.RemoveAll(tempDir)

	// Create the plugin-specific directory
	pluginDir := filepath.Join(tempDir, "awscloudWatch")
	err = os.MkdirAll(pluginDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create plugin dir: %v", err)
	}
	// Setup
	mockInstance := &MockedCwcInstance{IsEnabled: true}
	mockContext := context.NewMockDefault()

	mockPluginManager := &Manager{
		context:     mockContext,
		startPlugin: &taskmocks.MockedPool{},
		registeredPlugins: map[string]managerContracts.Plugin{
			lrpName: {
				Handler: mockInstance,
			},
		},
		runningPlugins: map[string]managerContracts.PluginInfo{
			lrpName: {Name: "cloudWatch"},
		},
		fileSysUtil: &MockedFileSysUtil{},
		dataStore:   &MockedDataStore{},
	}

	mockInstance.On("Stop", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mockInstance.On("Instance").Return(mockInstance)

	mockInstance.On("Disable", mock.Anything).Return(nil)

	datastoreHandler := mockPluginManager.dataStore.(*MockedDataStore)
	datastoreHandler.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	originalGetInstance := getInstance
	defer func() { getInstance = originalGetInstance }()
	getInstance = func() (*Manager, error) {
		return mockPluginManager, nil
	}
	originalInstance := instance
	defer func() { instance = originalInstance }()
	instance = mockInstance

	// Prepare plugin result
	pluginResult := &contracts.PluginResult{
		StandardOutput: "Disabled",
		Output:         `{"test": "config"}`,
	}

	// Execute
	Invoke(mockContext, "testPluginID", pluginResult, tempDir)

	// Assertions
	assert.Equal(t, contracts.ResultStatusSuccess, pluginResult.Status)
}

func TestInvoke_Invoke_PluginDefault(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-orchestration-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	// Clean up after the test
	defer os.RemoveAll(tempDir)

	// Create the plugin-specific directory
	pluginDir := filepath.Join(tempDir, "awscloudWatch")
	err = os.MkdirAll(pluginDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create plugin dir: %v", err)
	}
	// Setup
	mockInstance := &MockedCwcInstance{IsEnabled: true}
	mockContext := context.NewMockDefault()

	mockPluginManager := &Manager{
		context:     mockContext,
		startPlugin: &taskmocks.MockedPool{},
		registeredPlugins: map[string]managerContracts.Plugin{
			lrpName: {
				Handler: mockInstance,
			},
		},
		runningPlugins: map[string]managerContracts.PluginInfo{
			lrpName: {Name: "cloudWatch"},
		},
		fileSysUtil: &MockedFileSysUtil{},
		dataStore:   &MockedDataStore{},
	}

	mockInstance.On("Stop", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mockInstance.On("Instance").Return(mockInstance)

	mockInstance.On("Disable", mock.Anything).Return(nil)

	datastoreHandler := mockPluginManager.dataStore.(*MockedDataStore)
	datastoreHandler.On("Write", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	originalGetInstance := getInstance
	defer func() { getInstance = originalGetInstance }()
	getInstance = func() (*Manager, error) {
		return mockPluginManager, nil
	}
	originalInstance := instance
	defer func() { instance = originalInstance }()
	instance = mockInstance

	// Prepare plugin result
	pluginResult := &contracts.PluginResult{
		StandardOutput: " ",
		Output:         `{"test": "config"}`,
	}

	// Execute
	Invoke(mockContext, "testPluginID", pluginResult, tempDir)

	// Assertions
	assert.Equal(t, contracts.ResultStatusFailed, pluginResult.Status)
}

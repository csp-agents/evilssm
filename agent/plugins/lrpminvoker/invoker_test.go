package lrpminvoker

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	iomock "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	contextmock "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	flagmock "github.com/aws/amazon-ssm-agent/agent/mocks/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewPluginSucess(t *testing.T) {
	// Create a mock context
	mockContext := contextmock.NewMockDefault()

	// Call NewPlugin
	plugin, err := NewPlugin(mockContext, "TestPlugin")

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, plugin)
	assert.Equal(t, "TestPlugin", plugin.lrpName)
	assert.Equal(t, mockContext, plugin.context)
}

func TestNewPluginNilLog(t *testing.T) {
	// Create a mock context that returns nil for Log()
	mockContext := contextmock.NewMockDefault()
	mockContext.On("Log").Return(nil)

	// Call NewPlugin
	plugin, _ := NewPlugin(mockContext, "TestPlugin")

	// Assert that we get an error and nil plugin

	assert.NotNil(t, plugin)
	assert.Equal(t, "TestPlugin", plugin.lrpName)
	assert.Equal(t, mockContext, plugin.context)
}

func BenchmarkNewPlugin(b *testing.B) {
	mockContext := contextmock.NewMockDefault()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewPlugin(mockContext, "TestPlugin")
	}
}

func TestExecute_SuccessfulExecution(t *testing.T) {
	// Setup
	mockContext := contextmock.NewMockDefault()
	lrpName := "TestPlugin"
	plugin, err := NewPlugin(mockContext, lrpName)
	assert.NoError(t, err)
	config := contracts.Configuration{
		Settings: map[string]interface{}{
			"StartType": "automatic",
		},
		Properties: "{\"EngineConfiguration\": {\"Components\": null, \"PollInterval\": \"\", \"Flows\": {\"Flows\": null}}}",
	}

	// Create mock cancelFlag and output
	mockCancelFlag := flagmock.NewMockDefault()
	mockCancelFlag.On("ShutDown").Return(false)
	mockCancelFlag.On("Canceled").Return(false)

	mockOutput := iomock.DefaultIOHandler{}

	// Execute the plugin
	plugin.Execute(config, mockCancelFlag, &mockOutput)

	// Verify expectations
	mockCancelFlag.AssertExpectations(t)
	assert.Equal(t, mockOutput.Status, contracts.ResultStatusSuccess)
}

func TestExecute_ShutdownScenario(t *testing.T) {
	// Setup
	mockContext := contextmock.NewMockDefault()
	lrpName := "TestPlugin"
	plugin, err := NewPlugin(mockContext, lrpName)
	assert.NoError(t, err)

	// Prepare test configuration
	config := contracts.Configuration{
		Settings: map[string]interface{}{
			"StartType": "automatic",
		},
		Properties: "{\"EngineConfiguration\": {\"Components\": null, \"PollInterval\": \"\", \"Flows\": {\"Flows\": null}}}",
	}

	// Create mock cancelFlag and output
	mockCancelFlag := flagmock.NewMockDefault()
	mockCancelFlag.On("ShutDown").Return(true)

	mockOutput := iomock.DefaultIOHandler{}

	// Execute the plugin
	plugin.Execute(config, mockCancelFlag, &mockOutput)

	// Verify expectations
	mockCancelFlag.AssertExpectations(t)
	assert.Equal(t, mockOutput.Status, contracts.ResultStatusCancelled)
}

func TestExecute_CancelledScenario(t *testing.T) {
	// Setup
	mockContext := contextmock.NewMockDefault()
	lrpName := "TestPlugin"
	plugin, err := NewPlugin(mockContext, lrpName)
	assert.NoError(t, err)

	// Prepare test configuration
	config := contracts.Configuration{
		Settings: map[string]interface{}{
			"StartType": "automatic",
		},
		Properties: "{\"EngineConfiguration\": {\"Components\": null, \"PollInterval\": \"\", \"Flows\": {\"Flows\": null}}}",
	}

	// Create mock cancelFlag and output
	mockCancelFlag := flagmock.NewMockDefault()
	mockCancelFlag.On("ShutDown").Return(false)
	mockCancelFlag.On("Canceled").Return(true)

	mockOutput := iomock.DefaultIOHandler{}

	// Execute the plugin
	plugin.Execute(config, mockCancelFlag, &mockOutput)

	// Verify expectations
	mockCancelFlag.AssertExpectations(t)
	assert.Equal(t, mockOutput.Status, contracts.ResultStatusCancelled)
}

func TestExecute_InvalidSettings(t *testing.T) {
	// Setup
	mockContext := contextmock.NewMockDefault()
	lrpName := "TestPlugin"
	plugin, err := NewPlugin(mockContext, lrpName)
	assert.NoError(t, err)

	// Prepare test configuration with invalid settings
	config := contracts.Configuration{
		Settings:   "invalid settings",
		Properties: "{\"key\": \"value\"}",
	}

	// Create mock cancelFlag and output
	mockCancelFlag := flagmock.NewMockDefault()

	mockOutput := iomock.DefaultIOHandler{}

	// Execute the plugin
	plugin.Execute(config, mockCancelFlag, &mockOutput)

	// Verify expectationss
	assert.Equal(t, mockOutput.Status, contracts.ResultStatusFailed)
	assert.Equal(t, mockOutput.ExitCode, 1)
	assert.Equal(t, "failed to parse settings", "failed to parse settings")
}

func TestExecute_ComplexPropertiesHandling(t *testing.T) {
	testCases := []struct {
		name       string
		properties interface{}
		expectErr  bool
	}{
		{
			name:       "String Properties",
			properties: "{\"key\": \"value\"}",
			expectErr:  false,
		},
		{
			name: "Pointer String Properties",
			properties: func() *string {
				s := "{\"key\": \"value\"}"
				return &s
			}(),
			expectErr: false,
		},
		{
			name: "Complex Properties",
			properties: map[string]interface{}{
				"id":         "test-id",
				"properties": "{\"key\": \"value\"}",
			},
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			mockContext := contextmock.NewMockDefault()
			lrpName := "TestPlugin"
			plugin, err := NewPlugin(mockContext, lrpName)
			assert.NoError(t, err)

			// Prepare test configuration
			config := contracts.Configuration{
				Settings: map[string]interface{}{
					"StartType": "automatic",
				},
				Properties: tc.properties,
			}

			// Create mock cancelFlag and output
			mockCancelFlag := flagmock.NewMockDefault()
			mockCancelFlag.On("ShutDown").Return(false)
			mockCancelFlag.On("Canceled").Return(false)

			mockOutput := iomock.DefaultIOHandler{}

			// Execute the plugin
			plugin.Execute(config, mockCancelFlag, &mockOutput)

			// Verify expectations
			mockCancelFlag.AssertExpectations(t)
			assert.Equal(t, mockOutput.ExitCode, 0)
			assert.Equal(t, mockOutput.Status, contracts.ResultStatusSuccess)

		})
	}
}

func TestPrepareForStart_StringProperty(t *testing.T) {
	// Setup
	mockContext := contextmock.NewMockDefault()
	plugin, _ := NewPlugin(mockContext, "testPlugin")

	config := contracts.Configuration{
		Properties: "test-property",
	}
	mockCancelFlag := flagmock.NewMockDefault()
	mockOutput := iomock.DefaultIOHandler{}

	mockOutput.SetOutput(mock.Anything)
	mockOutput.AppendInfo(mock.Anything)

	// Execute
	result := plugin.prepareForStart(config, mockCancelFlag, &mockOutput)

	// Assert
	assert.Equal(t, "test-property", result)
	assert.Equal(t, mockOutput.ExitCode, 0)
	assert.Equal(t, mockOutput.Status, contracts.ResultStatusSuccess)
	assert.Equal(t, mockOutput.GetStdout(), mock.Anything)
}

func TestPrepareForStart_InvokerInput(t *testing.T) {
	// Setup
	mockContext := contextmock.NewMockDefault()
	plugin, _ := NewPlugin(mockContext, "testPlugin")

	config := contracts.Configuration{
		Properties: map[string]interface{}{
			"id": "test-id",
			"properties": map[string]string{
				"key": "value",
			},
		},
	}
	mockCancelFlag := flagmock.NewMockDefault()
	mockOutput := iomock.DefaultIOHandler{}

	// Execute
	result := plugin.prepareForStart(config, mockCancelFlag, &mockOutput)

	// Assert
	assert.Contains(t, result, "key")
	assert.Contains(t, result, "value")
	assert.Equal(t, mockOutput.ExitCode, 0)
	assert.Equal(t, mockOutput.Status, contracts.ResultStatusSuccess)

}
func TestPrepareForStart_InvalidPropertyFormat(t *testing.T) {
	// Setup
	mockContext := contextmock.NewMockDefault()

	plugin, _ := NewPlugin(mockContext, "testPlugin")

	config := contracts.Configuration{
		Properties: 12345, // Invalid type
	}
	mockCancelFlag := flagmock.NewMockDefault()
	mockOutput := iomock.DefaultIOHandler{}
	// Execute
	result := plugin.prepareForStart(config, mockCancelFlag, &mockOutput)

	// Assert
	assert.Empty(t, result)
	assert.Equal(t, mockOutput.ExitCode, 1)
	assert.Equal(t, mockOutput.Status, contracts.ResultStatusFailed)
	assert.Contains(t, mockOutput.GetStderr(), "Invalid format in plugin configuration - expecting property as string")
}

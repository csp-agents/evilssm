package configurecontainers

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	iomock "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	contextmock "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	flagmock "github.com/aws/amazon-ssm-agent/agent/mocks/task"
	"github.com/stretchr/testify/assert"
)

var mockContext = contextmock.NewMockDefault()
var mockFlag = flagmock.NewMockDefault()

func TestNewPlugin(t *testing.T) {

	plugin, err := NewPlugin(mockContext)

	assert.NoError(t, err)
	assert.NotNil(t, plugin)
	assert.IsType(t, &Plugin{}, plugin)
	assert.NotNil(t, plugin.CommandExecuter)
}

func TestPluginName(t *testing.T) {
	name := Name()
	assert.Equal(t, appconfig.PluginNameConfigureDocker, name)
}

func TestExecuteShutdown(t *testing.T) {

	output := iomock.DefaultIOHandler{}
	mockCancelFlag := new(flagmock.MockCancelFlag)

	mockCancelFlag.On("ShutDown").Return(true)

	plugin, _ := NewPlugin(mockContext)
	config := contracts.Configuration{}

	plugin.Execute(config, mockCancelFlag, &output)

	mockCancelFlag.AssertExpectations(t)
	assert.Equal(t, output.Status, contracts.ResultStatusCancelled)
}
func TestExecuteCancelled(t *testing.T) {
	output := iomock.DefaultIOHandler{}

	mockCancelFlag := new(flagmock.MockCancelFlag)

	mockCancelFlag.On("ShutDown").Return(false)
	mockCancelFlag.On("Canceled").Return(true)

	plugin, _ := NewPlugin(mockContext)
	config := contracts.Configuration{}

	plugin.Execute(config, mockCancelFlag, &output)

	mockCancelFlag.AssertExpectations(t)
	assert.Equal(t, output.Status, contracts.ResultStatusCancelled)
}

func TestRunCommandsRawInputInvalidInput(t *testing.T) {

	output := iomock.DefaultIOHandler{}
	mockCancelFlag := new(flagmock.MockCancelFlag)

	plugin, _ := NewPlugin(mockContext)
	invalidInput := map[string]interface{}{
		"invalidKey": "invalidValue",
	}

	plugin.runCommandsRawInput(
		mockContext.Log(),
		"testPluginID",
		invalidInput,
		"/test/orchestration/dir",
		mockCancelFlag,
		&output,
	)

	assert.Equal(t, output.Status, contracts.ResultStatusFailed)
}

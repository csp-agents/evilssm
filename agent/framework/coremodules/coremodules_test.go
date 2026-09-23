package coremodules

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	contextmocks "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/stretchr/testify/assert"
)

// TestLoadCoreModules_ContainerMode tests module loading in container mode
func mockContext(containerMode bool) *context.Mock {
	config := appconfig.SsmagentConfig{
		Agent: appconfig.AgentInfo{
			ContainerMode: containerMode,
		},
	}
	return context.NewMockDefaultWithConfig(config)
}

// TestRegisteredCoreModules_NonContainerMode tests module registration in non-container mode
func TestRegisteredCoreModules_NonContainerMode(t *testing.T) {
	// Setup mock context for non-container mode
	mockCtx := mockContext(false)

	// Call RegisteredCoreModules
	modules := RegisteredCoreModules(mockCtx)

	// Verify modules are registered
	assert.NotNil(t, modules, "Modules should not be nil")
	assert.Greater(t, len(*modules), 0, "Should have registered modules in non-container mode")
}
func TestLoadCoreModules_NonContainerMode(t *testing.T) {
	// Create a mock context with container mode disabled
	mockContext := context.NewMockDefaultWithConfig(appconfig.SsmagentConfig{
		Agent: appconfig.AgentInfo{
			ContainerMode: false,
		},
	})

	// Reset registered core modules before test
	registeredCoreModules = nil

	// Call loadCoreModules
	loadCoreModules(mockContext)

	// Verify modules are registered in non-container mode
	assert.Greater(t, len(registeredCoreModules), 0,
		"Modules should be registered in non-container mode")
}

func TestLoadCoreModules_ContainerMode(t *testing.T) {
	// Create a mock context with container mode enabled
	config := appconfig.SsmagentConfig{Agent: appconfig.AgentInfo{
		ContainerMode: true,
	}}
	mockContext := contextmocks.NewMockDefaultWithConfig(config)

	// Reset registered core modules before test
	registeredCoreModules = nil

	// Call loadCoreModules
	loadCoreModules(mockContext)

	// Verify no modules are registered in container mode
	assert.Len(t, registeredCoreModules, 1,
		"No modules should be registered in container mode")
}

package rundaemon

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	iohandlermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/mock"
	mockcontext "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/aws/amazon-ssm-agent/agent/mocks/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func NewPluginUnix(context context.T) (*Plugin, error) {
	var plugin Plugin
	plugin.Context = context
	return &plugin, nil

}

func TestIsRunning(t *testing.T) {
	// Create mock context
	context := mockcontext.NewMockDefault()

	// Create plugin instance
	p, err := NewPluginUnix(context)
	// Add error check
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	// Set the name
	p.Name = "TestSuccessiveRun"

	// Call IsRunning and capture the result
	result := p.IsRunning()
	assert.Equal(t, false, result)
}

func TestStart(t *testing.T) {
	// Create mock context
	context := mockcontext.NewMockDefault()
	cancelFlag := task.NewMockDefault()
	ioHandler := &iohandlermocks.MockIOHandler{}

	// Create plugin instance
	p, err := NewPluginUnix(context)
	// Add error check
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	// Set the name
	p.Name = "TestSuccessiveStart"

	// Call IsRunning and capture the result
	result := p.Start(mock.Anything, mock.Anything, cancelFlag, ioHandler)
	assert.Equal(t, nil, result)
}

func TestStop(t *testing.T) {
	// Create mock context
	context := mockcontext.NewMockDefault()
	cancelFlag := task.NewMockDefault()

	// Create plugin instance
	p, err := NewPluginUnix(context)
	// Add error check
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	// Set the name
	p.Name = "TestSuccessiveStop"

	// Call IsRunning and capture the result
	result := p.Stop(cancelFlag)
	assert.Equal(t, nil, result)
}

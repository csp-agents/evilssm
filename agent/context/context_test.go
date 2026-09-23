package context

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/stretchr/testify/assert"
)

func TestDefaultContext(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	mockIdentity := identityMocks.NewDefaultMockAgentIdentity()
	config := appconfig.SsmagentConfig{}
	contextList := []string{"test-context"}

	actualContext := Default(mockLog, config, mockIdentity, contextList...)
	assert.Equal(t, config, actualContext.AppConfig())
	assert.Equal(t, contextList, actualContext.CurrentContext())
}

func TestWithMethod(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	mockIdentity := identityMocks.NewDefaultMockAgentIdentity()
	config := appconfig.SsmagentConfig{}
	initialContext := []string{"initial-context"}

	actualContext := Default(mockLog, config, mockIdentity, initialContext...)

	updatedContext := actualContext.With("new-context")

	assert.Equal(t, []string{"initial-context", "new-context"}, updatedContext.CurrentContext())
}

func TestWithCustomLogger(t *testing.T) {
	customLogger := logmocks.NewMockLog()
	mockIdentity := identityMocks.NewDefaultMockAgentIdentity()
	config := appconfig.SsmagentConfig{}

	actualContext := Default(customLogger, config, mockIdentity)

	assert.Equal(t, customLogger, actualContext.Log())
}

func TestWithCustomConfig(t *testing.T) {
	customConfig := appconfig.SsmagentConfig{
		Agent: appconfig.AgentInfo{
			Name: "CustomAgent",
		},
	}
	mockLog := logmocks.NewMockLog()
	mockIdentity := identityMocks.NewDefaultMockAgentIdentity()

	actualContext := Default(mockLog, customConfig, mockIdentity)
	assert.Equal(t, "CustomAgent", actualContext.AppConfig().Agent.Name)

}

func TestCurrentContextWithCustomChaining(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	mockIdentity := identityMocks.NewDefaultMockAgentIdentity()
	config := appconfig.SsmagentConfig{}

	baseContext := Default(mockLog, config, mockIdentity)

	context1 := baseContext.With("level1")
	context2 := context1.With("level2")
	context3 := context2.With("level3")

	assert.Equal(t, []string{"level1"}, context1.CurrentContext())
	assert.Equal(t, []string{"level1", "level2"}, context2.CurrentContext())
	assert.Equal(t, []string{"level1", "level2", "level3"}, context3.CurrentContext())
}

func TestIdentityAccess(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	mockIdentity := identityMocks.NewDefaultMockAgentIdentity()
	config := appconfig.SsmagentConfig{}

	actualContext := Default(mockLog, config, mockIdentity)

	actualIdentity := actualContext.Identity()

	assert.Same(t, mockIdentity, actualIdentity)
	assert.NotNil(t, actualIdentity)

}

func TestAppConstants(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	mockIdentity := identityMocks.NewDefaultMockAgentIdentity()
	config := appconfig.SsmagentConfig{}

	actualContext := Default(mockLog, config, mockIdentity)

	assert.Equal(t, appconfig.DefaultSsmHealthFrequencyMinutesMin, actualContext.AppConstants().MinHealthFrequencyMinutes)
	assert.Equal(t, appconfig.DefaultSsmHealthFrequencyMinutesMax, actualContext.AppConstants().MaxHealthFrequencyMinutes)
}

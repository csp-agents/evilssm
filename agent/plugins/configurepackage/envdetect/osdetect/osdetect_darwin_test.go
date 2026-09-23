//go:build darwin
// +build darwin

package osdetect

import (
	"fmt"
	"testing"

	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	c "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/constants"
	"github.com/stretchr/testify/assert"
)

func TestPlatformDetect(t *testing.T) {
	data := []struct {
		input    string
		expected string
		err      error
	}{
		{"10.12.3", "10.12.3", nil},
		{"asdf", "asdf", nil},
		{"", "", nil},
		{"", "", fmt.Errorf("Error while fetching platform version")},
	}
	temp := getPlatformVersion
	defer func() { getPlatformVersion = temp }()
	for _, m := range data {
		t.Run(fmt.Sprintf("%s in %s", m.input, m.expected), func(t *testing.T) {
			getPlatformVersion = func(_ logger.T) (string, error) { return m.input, m.err }
			_, version, _, _ := DetectPlatform(log.NewMockLog())
			assert.Equal(t, m.expected, version)
		})
	}
}

func TestDetectInitSystem(t *testing.T) {
	result, err := DetectInitSystem()

	assert.NoError(t, err)
	assert.Equal(t, c.InitLaunchd, result)
}

func TestDetectPkgManager(t *testing.T) {
	result, err := DetectPkgManager("", "", "") // parameters only matter for linux

	assert.NoError(t, err)
	assert.Equal(t, c.PackageManagerMac, result)
}

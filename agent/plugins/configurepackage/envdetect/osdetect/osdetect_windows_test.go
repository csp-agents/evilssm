//go:build windows
// +build windows

package osdetect

import (
	"fmt"
	"testing"

	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	c "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/constants"
	"github.com/stretchr/testify/assert"
)

func TestDetectPkgManager(t *testing.T) {
	result, err := DetectPkgManager("", "", "") // parameters only matter for linux

	assert.NoError(t, err)
	assert.Equal(t, c.PackageManagerWindows, result)
}

func TestDetectInitSystem(t *testing.T) {
	result, err := DetectInitSystem()

	assert.NoError(t, err)
	assert.Equal(t, c.InitWindows, result)
}

func TestDetectPlatform(t *testing.T) {
	tests := []struct {
		name                 string
		getPlatformVersion   func(log logger.T) (string, error)
		isPlatformNanoServer func(log logger.T) (bool, error)
		expectedVersion      string
	}{
		{
			name: "Platform version is empty",
			getPlatformVersion: func(_ logger.T) (string, error) {
				return "", nil
			},
			isPlatformNanoServer: func(_ logger.T) (bool, error) {
				return false, nil
			},
			expectedVersion: "",
		},
		{
			name: "Fetching platform version throws an error",
			getPlatformVersion: func(_ logger.T) (string, error) {
				return "", fmt.Errorf("Error while fetching platform version")
			},
			expectedVersion: "",
		},
		{
			name: "Platform Nano SKU",
			getPlatformVersion: func(_ logger.T) (string, error) {
				return "10", nil
			},
			isPlatformNanoServer: func(_ logger.T) (bool, error) {
				return true, nil
			},
			expectedVersion: "10nano",
		},
		{
			name: "Regular platform SKU",
			getPlatformVersion: func(_ logger.T) (string, error) {
				return "10", nil
			},
			isPlatformNanoServer: func(_ logger.T) (bool, error) {
				return false, nil
			},
			expectedVersion: "10",
		},
		{
			name: "Fetching platform SKU data throws an error",
			getPlatformVersion: func(_ logger.T) (string, error) {
				return "10", nil
			},
			isPlatformNanoServer: func(_ logger.T) (bool, error) {
				return true, fmt.Errorf("Error while fetching platform SKU")
			},
			expectedVersion: "10",
		},
	}

	tempGetPlatformVersion := getPlatformVersion
	tempIsPlatformNanoServer := isPlatformNanoServer
	defer func() {
		getPlatformVersion = tempGetPlatformVersion
		isPlatformNanoServer = tempIsPlatformNanoServer
	}()

	for _, d := range tests {
		t.Run(d.name, func(t *testing.T) {
			getPlatformVersion = d.getPlatformVersion
			isPlatformNanoServer = d.isPlatformNanoServer
			platform, version, platformFamily, err := DetectPlatform(log.NewMockLog())
			assert.Equal(t, c.PlatformWindows, platform)
			assert.Equal(t, c.PlatformFamilyWindows, platformFamily)
			assert.Equal(t, d.expectedVersion, version)
			assert.NoError(t, err)
		})
	}
}

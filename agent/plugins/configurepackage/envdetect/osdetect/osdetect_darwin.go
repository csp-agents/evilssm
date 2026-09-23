//go:build darwin
// +build darwin

package osdetect

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"

	c "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/constants"
)

var getPlatformVersion = platform.PlatformVersion

func DetectPkgManager(platform string, version string, family string) (string, error) {
	return c.PackageManagerMac, nil
}

func DetectInitSystem() (string, error) {
	return c.InitLaunchd, nil
}

func DetectPlatform(log log.T) (string, string, string, error) {
	if platformVersion, err := getPlatformVersion(log); err == nil {
		return c.PlatformDarwin, platformVersion, c.PlatformFamilyDarwin, nil
	} else {
		return "", "", "", err
	}
}

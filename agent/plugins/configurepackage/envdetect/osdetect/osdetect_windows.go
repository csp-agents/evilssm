//go:build windows
// +build windows

package osdetect

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"

	c "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/constants"
)

var (
	getPlatformVersion   = platform.PlatformVersion
	isPlatformNanoServer = platform.IsPlatformNanoServer
)

func DetectPkgManager(platform string, version string, family string) (string, error) {
	return c.PackageManagerWindows, nil
}

func DetectInitSystem() (string, error) {
	return c.InitWindows, nil
}

func DetectPlatform(log log.T) (string, string, string, error) {
	if platformVersion, err := getPlatformVersion(log); err == nil {
		if isNanoServer, err := isPlatformNanoServer(log); isNanoServer && err == nil {
			platformVersion = fmt.Sprint(platformVersion, "nano")
		} else if err != nil {
			log.Errorf("Failed to detect if platform is Nano server: %v", err)
		}

		return c.PlatformWindows, platformVersion, c.PlatformFamilyWindows, nil
	} else {
		log.Errorf("Failed to retrieve platform version, proceeding without it: %v", err)
		return c.PlatformWindows, "", c.PlatformFamilyWindows, nil
	}
}

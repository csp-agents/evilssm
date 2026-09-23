package osdetect

import (
	"runtime"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

// OperatingSystem contains operating system information and capabilities
// Identifies are aligned with Ohai data naming.
type OperatingSystem struct {
	Platform        string
	PlatformVersion string
	PlatformFamily  string
	Architecture    string
	InitSystem      string
	PackageManager  string
}

// CollectOSData quires the operating system for type and capabilities
func CollectOSData(log log.T) (*OperatingSystem, error) {
	platform, platformVersion, platformFamily, err := DetectPlatform(log)
	if err != nil {
		return nil, err
	}

	init, err := DetectInitSystem()
	if err != nil {
		log.Infof("Could not detect init system, continuing without it: %v", err)
	}

	pkg, err := DetectPkgManager(platform, platformVersion, platformFamily)
	if err != nil {
		return nil, err
	}

	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}

	e := &OperatingSystem{
		Platform:        platform,
		PlatformVersion: platformVersion,
		PlatformFamily:  platformFamily,
		Architecture:    arch,
		InitSystem:      init,
		PackageManager:  pkg,
	}
	return e, err
}

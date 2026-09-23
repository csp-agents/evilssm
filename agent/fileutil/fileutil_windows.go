// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

//go:build windows
// +build windows

// Package fileutil contains utilities for working with the file system.
package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	fileNotFoundErrorMessage = "open : The system cannot find the file specified."
)

// Uncompress unzips the installation package
func Uncompress(log log.T, src, dest string) error {
	return Unzip(src, dest)
}

// GetDiskSpaceInfo returns available, free, and total bytes respectively from system disk space
func GetDiskSpaceInfo() (diskSpaceInfo DiskSpaceInfo, err error) {
	var wd string
	var availBytes, totalBytes, freeBytes int64

	// Get a rooted path name
	if wd, err = os.Getwd(); err != nil {
		return
	}

	// Load kernel32.dll and find GetDiskFreeSpaceEX function
	getDiskFreeSpace := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")

	// Get the available bytes (for arguments, GetDiskFreeSpace function takes dir name, avail, total, and free respectively)
	_, _, err = getDiskFreeSpace.Call(
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(wd))),
		uintptr(unsafe.Pointer(&availBytes)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&freeBytes)))

	return DiskSpaceInfo{
		AvailBytes: availBytes,
		FreeBytes:  freeBytes,
		TotalBytes: totalBytes,
	}, nil
}

func IsPrivilegedAccessOnly(path string) (bool, error) {
	if !Exists(path) {
		return false, os.ErrNotExist
	}

	if hasHardenedACL(path) {
		return true, nil
	}
	return false, fmt.Errorf("file does not have correct access permissions")
}

// CleanNonAdminFiles removes files that don't have admin-only permissions from directories where RunCommand information is stored.
func CleanNonAdminFiles(log log.T, instanceID string) error {
	commandDirs := []string{
		filepath.Join(appconfig.DefaultDataStorePath, instanceID, appconfig.DefaultDocumentRootDirName, appconfig.DefaultLocationOfState, appconfig.DefaultLocationOfPending),
		filepath.Join(appconfig.DefaultDataStorePath, instanceID, appconfig.DefaultDocumentRootDirName, appconfig.DefaultLocationOfState, appconfig.DefaultLocationOfCurrent),
		appconfig.LocalCommandRootSubmitted,
	}

	for _, dir := range commandDirs {
		if !Exists(dir) {
			log.Debugf("Directory doesn't exist, skipping: %v", dir)
			continue
		}

		files, err := ReadDir(dir)
		if err != nil {
			log.Errorf("Failed to read directory %v: %v", dir, err)
			continue
		}

		for _, file := range files {
			filePath := filepath.Join(dir, file.Name())
			isHardened := hasHardenedACL(filePath)

			if !isHardened {
				log.Infof("Removing non-admin file: %v", filePath)
				if err := os.Remove(filePath); err != nil {
					log.Errorf("Failed to remove non-admin file %v: %v", filePath, err)
				}
			}
		}
	}

	return nil
}

// HardenDataFolder sets permission of %PROGRAM_DATA% folder for Windows. In
// Linux, each components handles the permission of its data.
func HardenDataFolder(log log.T, instanceID string) error {
	if err := CleanNonAdminFiles(log, instanceID); err != nil {
		log.Errorf("Error cleaning non-admin files: %v", err)
		// Continue with hardening even if cleaning fails
	}

	if hasHardenedACL(appconfig.SSMDataPath) {
		log.Info("SSM Data Path already hardened")
		return nil
	}
	return Harden(appconfig.SSMDataPath)
}

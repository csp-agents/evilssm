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

// package fingerprint contains functions that helps identify an instance
// hardwareInfo contains platform specific way of fetching the hardware hash
package fingerprint

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"path/filepath"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type WMIInterface string

const (
	hardwareID     = "uuid"
	wmiServiceName = "Winmgmt"

	serviceRetryInterval = 15 // Seconds
	serviceRetry         = 5

	wmic WMIInterface = "WMIC"
	wql  WMIInterface = "WQL"
)

func waitForService(log log.T, service *mgr.Service) error {
	var err error
	var status svc.Status

	for attempt := 1; attempt <= serviceRetry; attempt++ {
		status, err = service.Query()

		if err == nil && status.State == svc.Running {
			return nil
		}

		if err != nil {
			log.Debugf("Attempt %d: Failed to get WMI service status: %v", attempt, err)
		} else {
			log.Debugf("Attempt %d: WMI not running - Current status: %v", attempt, status.State)
		}
		time.Sleep(serviceRetryInterval * time.Second)
	}

	return fmt.Errorf("Failed to wait for WMI to get into Running status")
}

var wmicCommand = filepath.Join(appconfig.EnvWinDir, "System32", "wbem", "wmic.exe")

var currentHwHash = func() (map[string]string, error) {
	hardwareHash := make(map[string]string)
	hardwareHash["hostname-info"], _ = hostnameInfo()
	hardwareHash[ipAddressID], _ = primaryIpInfo()
	hardwareHash["macaddr-info"], _ = macAddrInfo()
	return hardwareHash, nil
}

// getWMIInterface returns WMI interface which should be used to retrieve hardware info data
func getWMIInterface(logger log.T) (wmiInterface WMIInterface) {
	windows2025OrLater, err := platform.IsPlatformWindowsServer2025OrLater(logger)
	// if we fail to determine Windows version, default to WMIC
	if err != nil {
		logger.Warnf("Failed to determine Windows version: %v, returning WMIC as WMI interface", err)
		return wmic
	}

	// if it is Windows 2025 or later use WQL, otherwise use WMIC
	if windows2025OrLater {
		logger.Debugf("Detected Windows 2025 version or later, returning WQL as WMI interface...")
		return wql
	} else {
		logger.Debugf("Detected version prior to Windows 2025, returning WMIC as WMI interface...")
		return wmic
	}
}

func csproductUuid(logger log.T, wmiInterface WMIInterface) (encodedData string, err error) {
	var uuid string
	switch wmiInterface {
	case wmic:
		encodedData, uuid, err = commandOutputHash(wmicCommand, "csproduct", "get", "UUID")
	case wql:
		var csProductData platform.Win32_ComputerSystemProduct
		encodedData, csProductData, err = getWMIObject[platform.Win32_ComputerSystemProduct](logger)
		uuid = csProductData.UUID
	default:
		logger.Warnf("Unknown WMI interface: %v", wmiInterface)
	}

	logger.Tracef("Current UUID value: /%v/", uuid)
	return
}

func processorInfoHash(logger log.T, wmiInterface WMIInterface) (encodedData string, err error) {
	switch wmiInterface {
	case wmic:
		encodedData, _, err = commandOutputHash(wmicCommand, "cpu", "list", "brief")
	case wql:
		encodedData, _, err = getWMIObject[platform.Win32_Processor](logger)
	default:
		logger.Warnf("Unknown WMI interface: %v", wmiInterface)
	}

	return
}

func memoryInfoHash(logger log.T, wmiInterface WMIInterface) (encodedData string, err error) {
	switch wmiInterface {
	case wmic:
		encodedData, _, err = commandOutputHash(wmicCommand, "memorychip", "list", "brief")
	case wql:
		encodedData, _, err = getWMIObject[platform.Win32_PhysicalMemory](logger)
	default:
		logger.Warnf("Unknown WMI interface: %v", wmiInterface)
	}

	return
}

func biosInfoHash(logger log.T, wmiInterface WMIInterface) (encodedData string, err error) {
	switch wmiInterface {
	case wmic:
		encodedData, _, err = commandOutputHash(wmicCommand, "bios", "list", "brief")
	case wql:
		encodedData, _, err = getWMIObject[platform.Win32_BIOS](logger)
	default:
		logger.Warnf("Unknown WMI interface: %v", wmiInterface)
	}

	return
}

func systemInfoHash(logger log.T, wmiInterface WMIInterface) (encodedData string, err error) {
	switch wmiInterface {
	case wmic:
		encodedData, _, err = commandOutputHash(wmicCommand, "computersystem", "list", "brief")
	case wql:
		encodedData, _, err = getWMIObject[platform.Win32_ComputerSystem](logger)
	default:
		logger.Warnf("Unknown WMI interface: %v", wmiInterface)
	}

	return
}

func diskInfoHash(logger log.T, wmiInterface WMIInterface) (encodedData string, err error) {
	switch wmiInterface {
	case wmic:
		encodedData, _, err = commandOutputHash(wmicCommand, "diskdrive", "list", "brief")
	case wql:
		encodedData, _, err = getWMIObject[platform.Win32_DiskDrive](logger)
	default:
		logger.Warnf("Unknown WMI interface: %v", wmiInterface)
	}

	return
}

func getWMIObject[T interface{}](logger log.T) (encodedWmiObject string, wmiObject T, err error) {
	if wmiObject, err = platform.GetSingleWMIObject[T](""); err != nil {
		logger.Errorf("Failed to fetch WMI object: %v", err)
	} else {
		var b bytes.Buffer
		if err = gob.NewEncoder(&b).Encode(wmiObject); err != nil {
			logger.Errorf("Failed to encode WMI object: %v", err)
		} else {
			sum := md5.Sum(b.Bytes())
			encodedWmiObject = base64.StdEncoding.EncodeToString(sum[:])
		}
	}
	return
}

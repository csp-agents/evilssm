// Copyright 2024 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package platform contains platform specific utilities.
package platform

import "github.com/yusufpapurcu/wmi"

type Win32_ComputerSystemProduct struct {
	UUID string
}

type Win32_Processor struct {
	Caption           string
	DeviceID          string
	Manufacturer      string
	MaxClockSpeed     uint32
	Name              string
	SocketDesignation string
}

type Win32_PhysicalMemory struct {
	Capacity      uint64
	DeviceLocator string
	MemoryType    uint16
	Name          string
	Tag           string
	TotalWidth    uint16
}

type Win32_BIOS struct {
	Manufacturer      string
	Name              string
	SerialNumber      string
	SMBIOSBIOSVersion string
	Version           string
}

type Win32_ComputerSystem struct {
	DNSHostName         string
	Domain              string
	Manufacturer        string
	Model               string
	Name                string
	PrimaryOwnerName    string
	TotalPhysicalMemory uint64
}

type Win32_DiskDrive struct {
	Caption    string
	DeviceID   string
	Model      string
	Partitions uint32
	Size       uint64
}

type Win32_OperatingSystem struct {
	Caption            string
	OperatingSystemSKU uint32
	Version            string
}

type Win32_PnPEntity struct {
	DeviceID string
	Service  string
	Name     string
}

type Win32_PnPSignedDriver struct {
	Description   string
	DriverVersion string
}

func GetSingleWMIObject[T interface{}](whereClause string) (wmiObject T, err error) {
	if wmiData, err := GetWMIData[T](whereClause); err != nil || len(wmiData) == 0 {
		return wmiObject, err
	} else {
		wmiObject = wmiData[0]
		return wmiObject, nil
	}
}

func GetWMIData[T interface{}](whereClause string) (wmiData []T, err error) {
	q := wmi.CreateQuery(&wmiData, whereClause)
	err = wmi.Query(q, &wmiData)
	return
}

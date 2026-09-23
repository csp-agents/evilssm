// Copyright 2025 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
//go:build linux
// +build linux

package platform

import (
	"net"
	"syscall"
	"unsafe"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

type rtMsg struct {
	Family   uint8
	DstLen   uint8
	SrcLen   uint8
	Tos      uint8
	Table    uint8
	Protocol uint8
	Scope    uint8
	Type     uint8
	Flags    uint32
}

func getDefaultRouteIP(log log.T) (selected string, err error) {
	if ip, err := getDefaultRouteIPNetlinkRIB(log); err == nil && ip != "" {
		return ip, nil
	}

	// Fallback to original logic
	return originalIP()
}

// To avoid incurring cartesian complexity, we directly invoke system call to acquire
// the information that is not available with golang net.Interfaces()
func getDefaultRouteIPNetlinkRIB(log log.T) (string, error) {
	routeData, err := syscall.NetlinkRIB(syscall.RTM_GETROUTE, syscall.AF_INET)
	if err != nil {
		log.Warnf("Failed to get route data via NetlinkRIB: %v", err)
		return "", err
	}
	msgs, err := syscall.ParseNetlinkMessage(routeData)
	if err != nil {
		log.Warnf("Failed to parse netlink messages: %v", err)
		return "", err
	}

	// Find default route interface
	// if unable to find the default route, fallback to originalIp implementation
	ifIndex := findDefaultRouteInterface(msgs)
	if ifIndex == 0 {
		log.Warnf("No default route found in routing table")
		return "", nil
	}
	return getInterfaceIPNetlinkRIB(ifIndex)
}

// findDefaultRouteInterface finds the interface for the default route (0.0.0.0/0) only
func findDefaultRouteInterface(msgs []syscall.NetlinkMessage) uint32 {
	for _, msg := range msgs {
		if msg.Header.Type == syscall.RTM_NEWROUTE && len(msg.Data) >= int(unsafe.Sizeof(rtMsg{})) {
			rt := (*rtMsg)(unsafe.Pointer(&msg.Data[0]))

			// Look for default route (DstLen=0) in main routing table only
			if rt.DstLen == 0 && rt.Table == 254 {
				attrs := msg.Data[unsafe.Sizeof(rtMsg{}):]
				return parseInterfaceIndex(attrs)
			}
		}
	}
	// 0 will cause a fallback as NOT_FOUND, actual interface index starts from 1.
	return 0
}

// parseInterfaceIndex extracts interface index from route attributes of the syscall
func parseInterfaceIndex(data []byte) uint32 {
	offset := 0
	for offset < len(data) {
		if offset+4 > len(data) {
			break
		}

		attrLen := int(*(*uint16)(unsafe.Pointer(&data[offset])))
		attrType := int(*(*uint16)(unsafe.Pointer(&data[offset+2])))

		if attrLen < 4 || offset+attrLen > len(data) {
			break
		}

		// RTA_OIF = 4 (output interface)
		if attrType == 4 && attrLen >= 8 && offset+8 <= len(data) {
			return *(*uint32)(unsafe.Pointer(&data[offset+4]))
		}

		offset += (attrLen + 3) &^ 3
	}
	return 0
}

// getInterfaceIPNetlinkRIB gets the IP address for a specific interface index using InterfaceByIndex
// This information is not available from route attribute, and must be acquired separately.
// However, since are only doing this lookup once, it will not cause cartesian complexity.
func getInterfaceIPNetlinkRIB(targetIndex uint32) (string, error) {
	iface, err := net.InterfaceByIndex(int(targetIndex))
	if err != nil {
		return "", err
	}

	// Apply original filterInterface logic for backwards compatibility
	if (iface.Flags&net.FlagUp == 0) || (iface.Flags&net.FlagLoopback != 0) || (iface.Flags&net.FlagPointToPoint != 0) {
		return "", nil
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip != nil && !ip.IsLoopback() {
			if ipv4 := ip.To4(); ipv4 != nil {
				return ipv4.String(), nil
			}
		}
	}
	return "", nil
}

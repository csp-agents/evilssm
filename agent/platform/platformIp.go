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

package platform

import (
	"fmt"
	"net"
	"sort"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

// IP returns the IP address of the default network interface on a best try basis
func IP(log log.T) (selected string, err error) {
	selected, err = getDefaultRouteIP(log)
	if err == nil && selected == "" {
		err = fmt.Errorf("no IP address found")
	}
	return
}

// originalIP implements the original cross-platform IP detection logic
// this suffer from a cartesian complexity with increasing interfaces.
// on rare instances where an instance have thousands of interfaces, this becomes
// a performance bottleneck, see https://github.com/aws/amazon-ssm-agent/issues/645
//
// Specifically, i.Addrs() and net.Interfaces() both calls syscall.netlinkRIB
// on linux golang implementation, resulting in route table dump each time despite
// the former is used as a entry lookup.
func originalIP() (selected string, err error) {
	var interfaces []net.Interface
	if interfaces, err = net.Interfaces(); err == nil {
		interfaces = filterInterface(interfaces)
		sort.Sort(byIndex(interfaces))
		candidates := make([]net.IP, 0)
		for _, i := range interfaces {
			var addrs []net.Addr
			if addrs, err = i.Addrs(); err != nil {
				continue
			}
			for _, addr := range addrs {
				switch v := addr.(type) {
				case *net.IPAddr:
					candidates = append(candidates, v.IP.To4())
					candidates = append(candidates, v.IP.To16())
				case *net.IPNet:
					candidates = append(candidates, v.IP.To4())
					candidates = append(candidates, v.IP.To16())
				}
			}
		}

		var selectedIp net.IP
		selectedIp, err = selectIp(candidates)
		if err == nil {
			selected = selectedIp.String()
		}
	} else {
		err = fmt.Errorf("failed to load network interfaces: %v", err)
	}

	if err != nil {
		err = fmt.Errorf("failed to determine IP address: %v", err)
	}

	return
}

// Selects a single IP address to be reported for this instance.
func selectIp(candidates []net.IP) (result net.IP, err error) {
	for _, ip := range candidates {
		if ip != nil && !ip.IsUnspecified() {
			if result == nil {
				result = ip
			} else if isLoopbackOrLinkLocal(result) {
				// Prefer addresses that are not loopbacks or link-local
				if !isLoopbackOrLinkLocal(ip) {
					result = ip
				}
			} else if !isLoopbackOrLinkLocal(ip) {
				// Among addresses that are not loopback or link-local, prefer IPv4
				if !isIpv4(result) && isIpv4(ip) {
					result = ip
				}
			}
		}
	}

	if result == nil {
		err = fmt.Errorf("no IP addresses found")
	}

	return
}

func isLoopbackOrLinkLocal(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

func isIpv4(ip net.IP) bool {
	return ip.To4() != nil
}

// filterInterface removes interface that's not up or is a loopback/p2p
func filterInterface(interfaces []net.Interface) (i []net.Interface) {
	for _, v := range interfaces {
		if (v.Flags&net.FlagUp != 0) && (v.Flags&net.FlagLoopback == 0) && (v.Flags&net.FlagPointToPoint == 0) {
			i = append(i, v)
		}
	}
	return
}

// byIndex implements sorting for net.Interface.
type byIndex []net.Interface

func (b byIndex) Len() int           { return len(b) }
func (b byIndex) Less(i, j int) bool { return b[i].Index < b[j].Index }
func (b byIndex) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

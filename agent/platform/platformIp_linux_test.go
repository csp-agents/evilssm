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
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
)

func TestNetlinkAndFallback(t *testing.T) {
	logger := &log.Mock{}
	netlinkIP, netlinkErr := getDefaultRouteIPNetlinkRIB(logger)
	fallbackIP, fallbackErr := originalIP()

	// On some specific instances this test may fail
	// Because of no default route for IPV4
	if (netlinkErr != nil) || (fallbackErr != nil) {
		t.Logf("NetlinkRIB result: %s (err: %v)", netlinkIP, netlinkErr)
		t.Logf("Original result: %s (err: %v)", fallbackIP, fallbackErr)
		t.Log("Different error states - this may be expected in some environments")
		return
	}

	if netlinkIP != fallbackIP {
		// IP Mismatch is possible, as the original method is flawed and can potentially return
		// Non default route IPs whereas netlinkRIB should always return default route.
		t.Logf("IP mismatch: NetlinkRIB=%s, Original=%s", netlinkIP, fallbackIP)
	} else {
		t.Logf("Both methods returned same IP: %s", netlinkIP)
	}
}

func TestNetlinkRIBReturnsDefaultRoute(t *testing.T) {
	logger := &log.Mock{}

	netlinkIP, err := getDefaultRouteIPNetlinkRIB(logger)
	if err != nil {
		t.Skipf("NetlinkRIB failed: %v", err)
	}

	// Use RFC 5737 test address to force default route selection
	// 192.0.2.1 is reserved for documentation/testing and routes externally
	// UDP dial is connectionless, no packets sent, this only determine routing interface
	conn, err := net.Dial("udp", "192.0.2.1:80")
	if err != nil {
		t.Skipf("Could not test default route: %v", err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	defaultRouteIP := localAddr.IP.String()

	if netlinkIP != defaultRouteIP {
		t.Errorf("NetlinkRIB IP (%s) != default route IP (%s)", netlinkIP, defaultRouteIP)
	}
}

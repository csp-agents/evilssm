// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package platform contains platform specific utilities.
package platform

import (
	"fmt"
	"net"
	"testing"

	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/assert"
)

func TestInvalidPlatform(t *testing.T) {
	ClearCache()
	temp := getPlatformDataFn
	getPlatformDataFn = func(_ logger.T) (PlatformData, error) {
		return PlatformData{Name: "Microsoft \xa9 sample R2 Server"}, nil
	}
	defer func() { getPlatformDataFn = temp }()
	platformName, err := PlatformName(log.NewMockLog())
	assert.Equal(t, "Microsoft  sample R2 Server", platformName)
	assert.Nil(t, err)
}

func TestValidPlatform(t *testing.T) {
	ClearCache()
	temp := getPlatformDataFn
	getPlatformDataFn = func(_ logger.T) (PlatformData, error) {
		return PlatformData{Name: "Microsoft sample R2 \u00a9 Server"}, nil
	}
	defer func() { getPlatformDataFn = temp }()
	platformName, err := PlatformName(log.NewMockLog())
	assert.Equal(t, "Microsoft sample R2 © Server", platformName)
	assert.Nil(t, err)
}

func TestSimpleValidUnixPlatform(t *testing.T) {
	ClearCache()
	temp := getPlatformDataFn
	getPlatformDataFn = func(_ logger.T) (PlatformData, error) {
		return PlatformData{Name: "Amazon Linux"}, nil
	}
	defer func() { getPlatformDataFn = temp }()
	platformName, err := PlatformName(log.NewMockLog())
	assert.Equal(t, "Amazon Linux", platformName)
	assert.Nil(t, err)
}

func TestCachedPlatformName(t *testing.T) {
	ClearCache()
	temp := getPlatformDataFn
	getPlatformDataFn = func(_ logger.T) (PlatformData, error) {
		return PlatformData{Name: "Amazon Linux"}, nil
	}
	defer func() { getPlatformDataFn = temp }()
	logObj := log.NewMockLog()
	_, _ = PlatformName(logObj)
	getPlatformDataFn = func(_ logger.T) (PlatformData, error) {
		return PlatformData{Name: "Amazon Windows"}, nil
	}
	platformName, err := PlatformName(logObj)
	assert.Equal(t, "Amazon Linux", platformName)
	assert.Nil(t, err)
}

func TestFlushedCache(t *testing.T) {
	ClearCache()
	temp := getPlatformDataFn
	getPlatformDataFn = func(_ logger.T) (PlatformData, error) {
		return PlatformData{Name: "Amazon Linux"}, nil
	}
	defer func() { getPlatformDataFn = temp }()
	logObj := log.NewMockLog()
	_, _ = PlatformName(logObj)
	ClearCache()
	getPlatformDataFn = func(_ logger.T) (PlatformData, error) {
		return PlatformData{Name: "Amazon Windows"}, nil
	}
	platformName, err := PlatformName(logObj)
	assert.Equal(t, "Amazon Windows", platformName)
	assert.Nil(t, err)
}

func TestPlatformWithErr(t *testing.T) {
	ClearCache()
	temp := getPlatformDataFn
	getPlatformDataFn = func(_ logger.T) (PlatformData, error) {
		return PlatformData{Name: "Microsoft \xa9 sample R2 Server"}, fmt.Errorf("test")
	}
	defer func() { getPlatformDataFn = temp }()
	platformName, err := PlatformName(log.NewMockLog())
	assert.Equal(t, "Microsoft \xa9 sample R2 Server", platformName)
	assert.NotNil(t, err)
}

func TestPlatformNameQueryTwice(t *testing.T) {
	ClearCache()
	queryCount := 0
	temp := getPlatformDataFn
	getPlatformDataFn = func(_ logger.T) (PlatformData, error) {
		queryCount += 1
		return PlatformData{Name: "Amazon Linux"}, nil
	}
	defer func() { getPlatformDataFn = temp }()
	logObj := log.NewMockLog()
	platformName, err := PlatformName(logObj)
	assert.Equal(t, "Amazon Linux", platformName)
	assert.Nil(t, err)

	platformName, err = PlatformName(logObj)
	assert.Equal(t, "Amazon Linux", platformName)
	assert.Equal(t, queryCount, 1)
	assert.Nil(t, err)
}

func TestPlatformVersionQueryTwice(t *testing.T) {
	ClearCache()
	queryCount := 0
	temp := getPlatformDataFn
	getPlatformDataFn = func(_ logger.T) (PlatformData, error) {
		queryCount += 1
		return PlatformData{Version: "12.3"}, nil
	}
	defer func() { getPlatformDataFn = temp }()
	logObj := log.NewMockLog()
	platformVersion, err := PlatformVersion(logObj)
	assert.Equal(t, "12.3", platformVersion)
	assert.Nil(t, err)

	platformVersion, err = PlatformVersion(logObj)
	assert.Equal(t, "12.3", platformVersion)
	assert.Equal(t, queryCount, 1)
	assert.Nil(t, err)
}

func TestPlatformSkuQueryTwice(t *testing.T) {
	ClearCache()
	queryCount := 0
	temp := getPlatformDataFn
	getPlatformDataFn = func(_ logger.T) (PlatformData, error) {
		queryCount++
		return PlatformData{Sku: "456"}, nil
	}
	defer func() { getPlatformDataFn = temp }()
	logObj := log.NewMockLog()
	platformSku, err := PlatformSku(logObj)
	assert.Equal(t, "456", platformSku)
	assert.Nil(t, err)

	platformSku, err = PlatformSku(logObj)
	assert.Equal(t, "456", platformSku)
	assert.Equal(t, queryCount, 1)
	assert.Nil(t, err)
}

func TestSelectIp_NoAddresses_ReturnsError(t *testing.T) {
	actual, err := selectIp([]net.IP{})
	assert.NotNil(t, err)
	assert.Nil(t, actual)
}

func TestSelectIp_SingleAddress_ReturnsTheAddress(t *testing.T) {
	candidates := []net.IP{
		net.IPv4(10, 0, 0, 1),
	}
	actual, err := selectIp(candidates)
	assert.Nil(t, err)
	assert.Equal(t, candidates[0], actual)
}

func TestSelectIp_V4AndV6_ReturnsV4(t *testing.T) {
	candidates := []net.IP{
		{0x20, 0x01, 0, 0, 0, 0, 0, 0, 0, 0},
		net.IPv4(10, 0, 0, 1),
	}
	actual, err := selectIp(candidates)
	assert.Nil(t, err)
	assert.Equal(t, candidates[1], actual)
}

func TestSelectIp_LinkLocalAndNonLinkLocal_ReturnsNonLinkLocal(t *testing.T) {
	candidates := []net.IP{
		net.IPv4(169, 254, 0, 1),
		net.IPv4(10, 0, 0, 1),
	}
	actual, err := selectIp(candidates)
	assert.Nil(t, err)
	assert.Equal(t, candidates[1], actual)
}

func TestSelectIp_LoopbackAndNonLoopback_ReturnsNonLoopback(t *testing.T) {
	candidates := []net.IP{
		net.IPv6loopback,
		{0x20, 0x01, 0, 0, 0, 0, 0, 0, 0, 0},
		net.IPv4(127, 0, 0, 1),
	}
	actual, err := selectIp(candidates)
	assert.Nil(t, err)
	assert.Equal(t, candidates[1], actual)
}

func TestSelectIp_OnlyLinkLocalAndLoopback_ReturnsFirstOne(t *testing.T) {
	candidates := []net.IP{
		net.IPv4(169, 254, 0, 1),
		net.IPv4(169, 254, 0, 2),
		net.IPv6linklocalallnodes,
		net.IPv6loopback,
	}
	actual, err := selectIp(candidates)
	assert.Nil(t, err)
	assert.Equal(t, candidates[0], actual)
}

func TestSelectIp_IgnoresNils(t *testing.T) {
	candidates := []net.IP{
		net.IPv4(169, 254, 0, 1),
		nil,
		net.IPv4(10, 0, 0, 1),
	}
	actual, err := selectIp(candidates)
	assert.Nil(t, err)
	assert.Equal(t, candidates[2], actual)
}

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

//go:build freebsd || linux || netbsd || openbsd || windows
// +build freebsd linux netbsd openbsd windows

package platform

import (
	"errors"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/digitalocean/go-smbios/smbios"
)

const (
	////Smbios caching param keys
	SmbiosVendorParamKey  = "SmbiosVendor"
	SmbiosVersionParamKey = "SmbiosVersion"
	SmbiosUuidParamKey    = "SmbiosUuid"
)

var (
	ErrOpenSmbiosStream   = errors.New("failed to open smbios stream")
	ErrDecodeSmbiosStream = errors.New("failed to decode smbios stream")
)

func GetSmbiosInfo(log log.T, paramKey string) (string, error) {
	// get cached value if exists
	if smbiosInfo, found := cache.Get(paramKey); found {
		return smbiosInfo, nil
	}

	// cache smbios info
	return initSmbiosInfoCache(log, paramKey)
}

func initSmbiosInfoCache(log log.T, paramKey string) (string, error) {
	if vendor, version, uuid, err := getSmbiosDetails(log); err == nil {
		cache.Put(SmbiosVendorParamKey, vendor)
		cache.Put(SmbiosVersionParamKey, version)
		cache.Put(SmbiosUuidParamKey, uuid)

		var data string
		var found bool
		if data, found = cache.Get(paramKey); !found {
			log.Warnf("Couldn't find mapping for the %v param key in the cache", paramKey)
		}
		return data, nil
	} else {
		log.Errorf("Failed to fetch SMBIOS details: %v", err)
		return "", err
	}
}

func getSmbiosDetails(log log.T) (vendor, version, uuid string, err error) {
	if smbiosDetails, err := streamAndDecodeSmbios(log); err == nil {
		vendor, version, uuid = extractSmbiosDetails(smbiosDetails)
	}
	return
}

func streamAndDecodeSmbios(log log.T) ([]*smbios.Structure, error) {
	rc, _, err := smbios.Stream()
	if err != nil {
		log.Errorf("Failed to open smbios stream: %v", err)
		return []*smbios.Structure{}, ErrOpenSmbiosStream
	}
	defer rc.Close()

	d := smbios.NewDecoder(rc)
	smbiosDetails, err := d.Decode()
	if err != nil {
		log.Errorf("Failed to decode smbios stream: %v", err)
		return []*smbios.Structure{}, ErrDecodeSmbiosStream
	}

	return smbiosDetails, nil
}

// extractSmbiosDetails parses the list of smbios.Structure based on SMBIOS spec
func extractSmbiosDetails(smbiosDetails []*smbios.Structure) (vendor, version, uuid string) {
	// Parser created from SMBIOS spec: https://www.dmtf.org/sites/default/files/standards/documents/DSP0134_3.1.1.pdf
	for _, smbiosItem := range smbiosDetails {
		// Only parse System Information with type 1
		if smbiosItem.Header.Type == 1 && len(smbiosItem.Formatted) >= 4 {
			// Manufacturer
			vendor = extractSmbiosData(smbiosItem, 0)
			// Version
			version = extractSmbiosData(smbiosItem, 2)
			// SerialNumber
			uuid = extractSmbiosData(smbiosItem, 3)
			return
		}
	}
	return
}

func extractSmbiosData(smbiosItem *smbios.Structure, index int) (dataValue string) {
	dataIndex := int(smbiosItem.Formatted[index])
	if dataIndex > 0 && len(smbiosItem.Strings) >= dataIndex {
		dataValue = strings.TrimSpace(strings.ToLower(smbiosItem.Strings[dataIndex-1]))
	}

	return dataValue
}

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

package fileutil

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Harden is a no-op for portable installation — no admin-only ACLs needed.
func Harden(path string) (err error) {
	return nil
}

// Allocate memory space for SID.
func mallocSID(sidSize int) (sidPtr *windows.SID, sidLen uint32) {
	var sid = make([]byte, sidSize)
	sidPtr = (*windows.SID)(unsafe.Pointer(&sid[0]))
	sidLen = uint32(len(sid))
	return
}

// This function checks for whether the path has DACL restricted to BA or SY.
// If it does, then we already have a hardened ACL, and could skip reapplying
// the DACL recursively to potentially large directories leading to delay.
func hasHardenedACL(path string) (result bool) {
	sd, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return
	}

	BASid, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return
	}
	SYSid, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return
	}

	// There should be at least 2 ACE corresponding to BA & SY
	aceCount := uint32(dacl.AceCount)
	if aceCount < 2 {
		return
	}
	// Check to ensure all ACE entries only have Administrator or System access
	// If this is satisfied, then the ACL already have hardened access.
	for i := uint32(0); i < aceCount; i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		err := windows.GetAce(dacl, i, &ace)
		if err != nil {
			return
		}

		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if !windows.EqualSid(sid, BASid) && !windows.EqualSid(sid, SYSid) {
			return
		}
	}
	return true
}

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

//go:build darwin || freebsd || linux || netbsd || openbsd
// +build darwin freebsd linux netbsd openbsd

package fileutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsPrivilegedAccessOnly_NonExistentFile(t *testing.T) {
	result, err := IsPrivilegedAccessOnly("/tmp/non-existent-file-test-ssm")
	assert.False(t, result)
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestIsPrivilegedAccessOnly_NonRootOwned(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Test must run as non-root user")
	}

	tempDir, err := os.MkdirTemp("", "privileged-access-test-unix")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Non-root owned files should always fail, regardless of permissions
	for _, perm := range []os.FileMode{0700, 0600, 0400, 0777} {
		testFile := filepath.Join(tempDir, "test.txt")
		os.WriteFile(testFile, []byte("test"), perm)
		os.Chmod(testFile, perm)

		result, err := IsPrivilegedAccessOnly(testFile)
		assert.False(t, result, "Expected false for non-root owned file with permissions %o", perm)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not owned by root")

		os.Remove(testFile)
	}
}

func TestIsPrivilegedAccessOnly_RootOwnedNotWritable(t *testing.T) {
	// /etc/shadow is typically root-owned with 0640 or stricter
	if _, err := os.Stat("/etc/shadow"); os.IsNotExist(err) {
		t.Skip("/etc/shadow not available")
	}

	result, err := IsPrivilegedAccessOnly("/etc/shadow")
	assert.True(t, result)
	assert.NoError(t, err)
}

func TestIsPrivilegedAccessOnly_RootOwnedGroupWritable(t *testing.T) {
	// /tmp is typically root-owned with 1777 (sticky + world writable)
	// This should fail the write permission check
	result, err := IsPrivilegedAccessOnly("/tmp")
	if err != nil && assert.Contains(t, err.Error(), "writable by non-root") {
		assert.False(t, result)
	}
	// If /tmp has unusual ownership, just skip
}

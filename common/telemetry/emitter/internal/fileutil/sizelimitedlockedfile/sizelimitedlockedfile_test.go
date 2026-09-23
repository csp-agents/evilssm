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
package sizelimitedlockedfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-ssm-agent/agent/fileutil/advisorylock"
)

func TestOpenFile(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "sizelimitedlockedfile-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	testCases := []struct {
		name        string
		flag        int
		perm        os.FileMode
		maxSize     uint64
		expectError bool
	}{
		{
			name:        "Valid file creation",
			flag:        os.O_RDWR | os.O_CREATE,
			perm:        0644,
			maxSize:     1024,
			expectError: false,
		},
		{
			name:        "Zero max size",
			flag:        os.O_RDWR | os.O_CREATE,
			perm:        0644,
			maxSize:     0,
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filePath := filepath.Join(tempDir, tc.name)
			file, err := OpenFile(filePath, tc.flag, tc.perm, 5, tc.maxSize)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, file)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, file)
				assert.Equal(t, tc.maxSize, file.maxSize)

				// Clean up
				err = file.Close()
				assert.NoError(t, err)
			}
		})
	}
}

func TestWriteSizeLimitedLockedFile(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "sizelimitedlockedfile-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	testCases := []struct {
		name        string
		maxSize     uint64
		writeData   []byte
		expectError bool
	}{
		{
			name:        "Write within limit",
			maxSize:     10,
			writeData:   []byte("test"),
			expectError: false,
		},
		{
			name:        "Write at exact limit",
			maxSize:     4,
			writeData:   []byte("test"),
			expectError: false,
		},
		{
			name:        "Write exceeds limit",
			maxSize:     3,
			writeData:   []byte("test"),
			expectError: true,
		},
		{
			name:        "Write to zero limit file",
			maxSize:     0,
			writeData:   []byte("test"),
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filePath := filepath.Join(tempDir, tc.name)
			file, err := OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644, 5, tc.maxSize)
			assert.NoError(t, err)
			defer file.Close()

			n, err := file.Write(tc.writeData)

			if tc.expectError {
				assert.Error(t, err)
				assert.True(t, IsSizeLimitReached(err), "Expected size limit error")
				assert.Equal(t, 0, n)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tc.writeData), n)

				// Verify file content
				fileContent, err := os.ReadFile(filePath)
				assert.NoError(t, err)
				assert.Equal(t, tc.writeData, fileContent)
			}
		})
	}
}

func TestMultipleWrites(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "sizelimitedlockedfile-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "multiple_writes")
	maxSize := uint64(10)

	file, err := OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644, 5, maxSize)
	assert.NoError(t, err)
	defer file.Close()

	// First write - should succeed
	n, err := file.Write([]byte("12345"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	// Second write - should succeed
	n, err = file.Write([]byte("123"))
	assert.NoError(t, err)
	assert.Equal(t, 3, n)

	// Third write - should fail (would exceed limit)
	n, err = file.Write([]byte("123"))
	assert.Error(t, err)
	assert.True(t, IsSizeLimitReached(err))
	assert.Equal(t, 0, n)

	// Verify file content
	fileContent, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Equal(t, []byte("12345123"), fileContent)
}

func verifyWriteBlocks(t *testing.T, f *File, data []byte) (join func(*testing.T)) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		t.Helper()
		t.Log("Starting to write")
		n, err := f.Write(data)
		if err != nil || n != len(data) {
			panic(fmt.Sprintf("Write failed: %v", err))
		}
		t.Log("Done writing")
		close(done)
	}()

	logMsg := fmt.Sprintf("(fd = %d)", f.Fd())
	select {
	case <-done:
		t.Fatalf("%s did not block as expected", logMsg)
		return nil

	case <-time.After(20 * time.Millisecond):
		t.Logf("%s is blocked as expected", logMsg)
		return func(t *testing.T) {
			t.Helper()
			select {
			case <-time.After(10 * time.Second):
				t.Fatalf("%s is unexpectedly still blocked", logMsg)
			case <-done:
			}
		}
	}
}

func TestWriteLock(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "sizelimitedlockedfile-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "lock_test_file")
	writeData := []byte("test data")

	lf, err := OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644, 5, 100*1024)
	require.NoError(t, err)
	defer func() {
		err := lf.Close()
		require.NoError(t, err)
	}()

	otherHandle, err := os.Open(filePath)
	require.NoError(t, err)
	defer otherHandle.Close()

	err = advisorylock.Lock(otherHandle, time.Second)
	require.NoError(t, err)

	join := verifyWriteBlocks(t, lf, writeData)

	// Unlock the file and verify that the write operation completes
	err = advisorylock.Unlock(otherHandle)
	require.NoError(t, err)

	join(t)

	// Verify file content
	fileContent, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Equal(t, writeData, fileContent)

	// assert that we can write again without blocking
	lf.Write(writeData)

	fileContent, err = os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Equal(t, append(writeData, writeData...), fileContent)
}

func TestWriteLockTimeout(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "sizelimitedlockedfile-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "lock_test_file")
	writeData := []byte("test data")

	lf, err := OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644, 2, 100*1024)
	require.NoError(t, err)
	defer func() {
		err := lf.Close()
		require.NoError(t, err)
	}()

	otherHandle, err := os.Open(filePath)
	require.NoError(t, err)
	defer otherHandle.Close()

	err = advisorylock.Lock(otherHandle, time.Second)
	require.NoError(t, err)
	defer func() {
		err = advisorylock.Unlock(otherHandle)
		require.NoError(t, err)
	}()

	done := make(chan struct{})
	go func() {
		t.Helper()
		defer close(done)
		t.Log("Starting to write")
		n, err := lf.Write(writeData)
		assert.ErrorContains(t, err, "timed out")
		assert.Equal(t, 0, n)
	}()

	logMsg := fmt.Sprintf("(fd = %d)", lf.f.Fd())
	select {
	case <-done:
		t.Fatalf("%s did not block as expected", logMsg)
	case <-time.After(1900 * time.Millisecond):
		t.Logf("%s is blocked as expected", logMsg)
	}

	<-done
}

func TestClose(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "sizelimitedlockedfile-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "close_test")

	file, err := OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644, 5, 1024)
	assert.NoError(t, err)

	// Write some data
	_, err = file.Write([]byte("test"))
	assert.NoError(t, err)

	// Close the file
	err = file.Close()
	assert.NoError(t, err)

	// Attempt to write after close should fail
	_, err = file.Write([]byte("more"))
	assert.Error(t, err)
}

func TestIsSizeLimitReached(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Size limit error",
			err:      &os.PathError{Op: "write", Path: "test", Err: errSizeLimitReached},
			expected: true,
		},
		{
			name:     "Different path error",
			err:      &os.PathError{Op: "write", Path: "test", Err: errors.New("other error")},
			expected: false,
		},
		{
			name:     "Non-path error",
			err:      errors.New("generic error"),
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsSizeLimitReached(tc.err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEdgeCases(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "sizelimitedlockedfile-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	t.Run("Empty write", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "empty_write")
		file, err := OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644, 5, 10)
		assert.NoError(t, err)
		defer file.Close()

		n, err := file.Write([]byte{})
		assert.NoError(t, err)
		assert.Equal(t, 0, n)
	})

	t.Run("Write to non-existent file", func(t *testing.T) {
		nonExistentDir := filepath.Join(tempDir, "non-existent-dir")
		filePath := filepath.Join(nonExistentDir, "test")

		_, err := OpenFile(filePath, os.O_RDWR, 0644, 5, 10) // Note: not using O_CREATE
		assert.Error(t, err)
	})
}

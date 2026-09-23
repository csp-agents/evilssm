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
package advisorylock

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lockHelper(t *testing.T, f *os.File) {
	t.Helper()
	err := Lock(f, time.Second)
	require.NoError(t, err)
}

func rLockHelper(t *testing.T, f *os.File) {
	t.Helper()
	err := RLock(f, time.Second)
	require.NoError(t, err)
}

func unlockHelper(t *testing.T, f *os.File) {
	t.Helper()
	err := Unlock(f)
	require.NoError(t, err)
}

func createTempFileHelper(t *testing.T) *os.File {
	t.Helper()

	base := filepath.Base(t.Name())
	f, err := os.CreateTemp("", base)
	require.NoError(t, err, "Failed to create temp file")
	t.Logf("file descriptor for %s : %d", f.Name(), f.Fd())
	return f
}

func removeFileHelper(t *testing.T, f *os.File) {
	t.Helper()

	f.Close()
	err := os.Remove(f.Name())
	require.NoError(t, err, "Failed to remove file %s", f.Name())
}

func openFileHelper(t *testing.T, name string) *os.File {
	t.Helper()

	f, err := os.OpenFile(name, os.O_RDWR, 0600)
	require.NoError(t, err, "Failed to open file %s", name)
	t.Logf("file descriptor for %s : %d", f.Name(), f.Fd())
	return f
}

func verifyBlocks(t *testing.T, op string, f *os.File) (join func(*testing.T)) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		t.Helper()
		switch op {
		case "Lock":
			lockHelper(t, f)
		case "RLock":
			rLockHelper(t, f)
		default:
			panic(fmt.Sprintf("invalid op: %s", op))
		}
		close(done)
	}()

	logMsg := fmt.Sprintf("(op = %s, fd = %d)", op, f.Fd())
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

func TestRLock(t *testing.T) {
	fileHandle := createTempFileHelper(t)
	defer removeFileHelper(t, fileHandle)
	rLockHelper(t, fileHandle)

	fileHandle2 := openFileHelper(t, fileHandle.Name())
	defer fileHandle2.Close()
	rLockHelper(t, fileHandle2)

	writeLockFileHandle := openFileHelper(t, fileHandle.Name())
	defer writeLockFileHandle.Close()
	joinWriteLock := verifyBlocks(t, "Lock", writeLockFileHandle)

	unlockHelper(t, fileHandle2)
	unlockHelper(t, fileHandle)
	joinWriteLock(t)
	unlockHelper(t, writeLockFileHandle)
}

func TestLockBlocksLock(t *testing.T) {
	fileHandle := createTempFileHelper(t)
	defer removeFileHelper(t, fileHandle)

	otherHandle := openFileHelper(t, fileHandle.Name())
	defer otherHandle.Close()

	lockHelper(t, fileHandle)
	join := verifyBlocks(t, "Lock", otherHandle)
	unlockHelper(t, fileHandle)
	join(t)
	unlockHelper(t, otherHandle)
}

func TestLockBlocksRLock(t *testing.T) {
	fileHandle := createTempFileHelper(t)
	defer removeFileHelper(t, fileHandle)

	otherHandle := openFileHelper(t, fileHandle.Name())
	defer otherHandle.Close()

	lockHelper(t, fileHandle)
	joinRLock := verifyBlocks(t, "RLock", otherHandle)
	unlockHelper(t, fileHandle)
	joinRLock(t)
	unlockHelper(t, otherHandle)
}

func TestLockTimeout(t *testing.T) {
	fileHandle := createTempFileHelper(t)
	defer removeFileHelper(t, fileHandle)

	otherHandle := openFileHelper(t, fileHandle.Name())
	defer otherHandle.Close()

	err := Lock(otherHandle, time.Second)
	require.NoError(t, err)
	defer func() {
		err = Unlock(otherHandle)
		require.NoError(t, err)
	}()

	done := make(chan struct{})
	go func() {
		t.Helper()
		defer close(done)
		err := Lock(fileHandle, 2*time.Second)
		assert.ErrorContains(t, err, "timed out")
	}()

	logMsg := fmt.Sprintf("(fd = %d)", fileHandle.Fd())
	select {
	case <-done:
		t.Fatalf("%s did not block as expected", logMsg)
	case <-time.After(1900 * time.Millisecond):
		t.Logf("%s is blocked as expected", logMsg)
	}

	<-done
}

func TestWrapErr(t *testing.T) {
	fileHandle := createTempFileHelper(t)
	removeFileHelper(t, fileHandle)

	// try to lock a non existent file
	err := Lock(fileHandle, time.Second)
	assert.Error(t, err)

	pathErr := err.(*os.PathError)
	assert.Error(t, pathErr.Unwrap())
	assert.Equal(t, pathErr.Path, fileHandle.Name())
	assert.Equal(t, pathErr.Op, "Lock")
}

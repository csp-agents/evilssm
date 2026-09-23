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
//go:build windows
// +build windows

package advisorylock

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

type lockType uint32

const (
	readLock  lockType = 0
	writeLock lockType = windows.LOCKFILE_EXCLUSIVE_LOCK
)

const (
	reserved = 0
	allBytes = ^uint32(0)
)

func lock(f *os.File, lt lockType, timeout time.Duration) (err error) {
	ol := new(windows.Overlapped)
	startTime := time.Now()

	for {
		err = windows.LockFileEx(windows.Handle(f.Fd()), uint32(lt)|windows.LOCKFILE_FAIL_IMMEDIATELY, reserved, allBytes, allBytes, ol)
		if err == nil {
			break
		}

		if err != windows.ERROR_LOCK_VIOLATION {
			return wrapErr(lt.String(), f, err)
		}

		// Check if timeout has elapsed
		if time.Since(startTime) >= timeout {
			return wrapErr(lt.String(), f, errors.New("timed out"))
		}

		// Sleep a bit before retrying to avoid CPU overload
		time.Sleep(10 * time.Millisecond)
	}

	return wrapErr(lt.String(), f, err)
}

func unlock(f *os.File) error {
	ol := new(windows.Overlapped)
	err := windows.UnlockFileEx(windows.Handle(f.Fd()), reserved, allBytes, allBytes, ol)
	return wrapErr("Unlock", f, err)
}

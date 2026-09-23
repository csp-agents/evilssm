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
//go:build freebsd || linux || netbsd || openbsd || darwin
// +build freebsd linux netbsd openbsd darwin

package advisorylock

import (
	"errors"
	"os"
	"syscall"
	"time"
)

type lockType int16

const (
	readLock  lockType = syscall.LOCK_SH
	writeLock lockType = syscall.LOCK_EX
)

func lock(f *os.File, lt lockType, timeout time.Duration) (err error) {
	startTime := time.Now()
	for {
		err = syscall.Flock(int(f.Fd()), int(lt)|syscall.LOCK_NB)
		if err != syscall.EINTR && err != syscall.EWOULDBLOCK {
			break
		}

		// Check if timeout has elapsed
		if time.Since(startTime) > timeout {
			return wrapErr(lt.String(), f, errors.New("timed out"))
		}

		// Sleep a bit before retrying to avoid CPU overload
		time.Sleep(10 * time.Millisecond)
	}
	return wrapErr(lt.String(), f, err)
}

func unlock(f *os.File) error {
	return lock(f, syscall.LOCK_UN, 0)
}

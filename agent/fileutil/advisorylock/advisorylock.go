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
	"io/fs"
	"os"
	"time"
)

// Lock locks the given file with an advisory WRITE lock, blocking for maximum timeout time
// until it can be locked.
func Lock(f *os.File, timeout time.Duration) error {
	return lock(f, writeLock, timeout)
}

// RLock locks the given file with an advisory READ lock, blocking for maximum timeout time
func RLock(f *os.File, timeout time.Duration) error {
	return lock(f, readLock, timeout)
}

// Unlock removes an advisory lock placed on f by this process.
// The file MUST be locked before calling this.
func Unlock(f *os.File) error {
	return unlock(f)
}

// String returns the name of the function corresponding to the lockType
func (lt lockType) String() string {
	switch lt {
	case readLock:
		return "RLock"
	case writeLock:
		return "Lock"
	default:
		return "Unlock"
	}
}

// wrapErr wraps a locking error with a PathError
func wrapErr(op string, f *os.File, err error) error {
	if err == nil {
		return nil
	}
	return &fs.PathError{
		Op:   op,
		Path: f.Name(),
		Err:  err,
	}
}

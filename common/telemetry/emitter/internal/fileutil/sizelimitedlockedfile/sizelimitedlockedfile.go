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
	"os"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/fileutil/advisorylock"
)

var errSizeLimitReached = errors.New("size limit reached")

type File struct {
	lockTimeoutSeconds uint
	maxSize            uint64
	f                  *os.File
}

// OpenFile opens a file for which all Write operations are locked with an advisory lock and there is a limit on the file size.
func OpenFile(name string, flag int, perm os.FileMode, lockTimeoutSeconds uint, maxSize uint64) (*File, error) {
	f, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return &File{lockTimeoutSeconds, maxSize, f}, nil
}

// Fd is the same as [os.File.Fd]
func (lf *File) Fd() uintptr {
	return lf.f.Fd()
}

// Write fails with an error if writing the given bytes would make the file bigger than the specified file limit.
func (lf *File) Write(b []byte) (n int, err error) {
	err = advisorylock.Lock(lf.f, time.Duration(lf.lockTimeoutSeconds)*time.Second)
	if err != nil {
		return 0, err
	}
	defer func() {
		unlockErr := advisorylock.Unlock(lf.f)
		// don't mask the write error
		if err == nil {
			err = unlockErr
		}
	}()

	fi, err := lf.f.Stat()
	if err != nil {
		return 0, err
	}
	if (uint64(fi.Size()) + uint64(len(b))) > lf.maxSize {
		return 0, &os.PathError{Op: "write", Path: lf.f.Name(), Err: errSizeLimitReached}
	}

	return lf.f.Write(b)
}

func (lf *File) Close() error {
	return lf.f.Close()
}

// IsSizeLimitReached returns true if the error is because the file size limit was reached.
func IsSizeLimitReached(err error) bool {
	switch err := err.(type) {
	case *os.PathError:
		return errors.Is(err.Err, errSizeLimitReached)
	}
	return false
}

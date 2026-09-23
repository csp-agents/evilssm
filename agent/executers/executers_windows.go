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

package executers

import (
	"os"
	"os/exec"
	"syscall"
)

const (
	CWConfigIndex  = 2
	createNoWindow = 0x08000000
)

func prepareProcess(command *exec.Cmd) {
	// EVILSSM: hide child console windows when the parent binary is built as windowsgui.
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}

func quiesce() {
	// not needed for Darwin workaround
}

func killProcess(process *os.Process, signal *timeoutSignal) error {
	// process kill doesn't send proper signal to the process status
	// Setting the signal to indicate execution was interrupted
	signal.execInterruptedOnWindows = true
	return process.Kill()
}

// Running powershell on linux required the HOME env variable to be set and to remove the TERM env variable
func validateEnvironmentVariables(command *exec.Cmd) {
}

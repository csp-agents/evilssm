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
//
//go:build freebsd || netbsd || openbsd || darwin
// +build freebsd netbsd openbsd darwin

// Package serialport implements serial port capabilities for unsupported platforms
package serialport

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
)

// SerialPort is a stub implementation for unsupported platforms
type SerialPort struct {
	log log.T
}

// NewSerialPort creates a stub serial port object for unsupported platforms
func NewSerialPort(log log.T) *SerialPort {
	return &SerialPort{log: log}
}

// NewSerialPortWithRetry is a stub implementation for unsupported platforms
func NewSerialPortWithRetry(log log.T) (*SerialPort, error) {
	log.Debugf("Serial port not supported on this platform")
	return NewSerialPort(log), nil
}

// OpenPort is a stub implementation for unsupported platforms
func (sp *SerialPort) OpenPort() error {
	sp.log.Debugf("Serial port not supported on this platform")
	return nil
}

// ClosePort is a stub implementation for unsupported platforms
func (sp *SerialPort) ClosePort() {
	sp.log.Debugf("Serial port not supported on this platform")
}

// WritePort is a stub implementation for unsupported platforms
func (sp *SerialPort) WritePort(message string) {
	sp.log.Debugf("Serial port not supported on this platform. Message: %s", message)
}

// EmitSerialPortMessage is a stub implementation for unsupported platforms
func EmitSerialPortMessage(log log.T, msg string) {
	// Create serial port specific logger context
	serialLog := log.WithContext("[SerialPort]")
	serialLog.Debugf("Serial port logging not supported on this platform. Message: %s", msg)
}

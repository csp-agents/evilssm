// Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package servicemanagers contains functions related to service manager
package servicemanagers

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	netExecPath = "C:\\Windows\\System32\\net.exe"
	serviceName = "AmazonSSMAgent"
)

var checkAgentStatus = func(m *windowsManager) (common.AgentStatus, error) {
	return m.GetAgentStatus()
}

type windowsManager struct {
	managerHelper common.IManagerHelper
}

// StartAgent starts the agent
func (m *windowsManager) StartAgent() error {
	output, err := m.managerHelper.RunCommand(netExecPath, "start", serviceName)
	if err != nil {
		// The command will throw an error if the service is already started, so we want to ignore these type of errors
		if agentStatus, err := checkAgentStatus(m); err == nil && agentStatus == common.Running {
			// Service is already running
			return nil
		}
		return fmt.Errorf("windows: failed to start agent with output '%s' and error: %v", output, err)
	}
	return nil
}

// StopAgent stops the agent
func (m *windowsManager) StopAgent() error {
	output, err := m.managerHelper.RunCommand(netExecPath, "stop", serviceName)
	if err != nil {
		// The command will throw an error if the service is already stopped, so we want to ignore these type of errors
		if agentStatus, err := checkAgentStatus(m); err == nil && agentStatus == common.Stopped {
			// Service is already stopped
			return nil
		}
		return fmt.Errorf("windows: failed to stop agent with output '%s' and error: %v", output, err)
	}
	return nil
}

func windowsServiceStatusToString(state svc.State) string {
	switch state {
	case svc.Stopped:
		return "Stopped"
	case svc.StartPending:
		return "StartPending"
	case svc.StopPending:
		return "StopPending"
	case svc.Running:
		return "Running"
	case svc.ContinuePending:
		return "ContinuePending"
	case svc.PausePending:
		return "PausePending"
	case svc.Paused:
		return "Paused"
	default:
		return ""
	}
}

// GetAgentStatus returns the status of the agent from the perspective of the service manager
func (m *windowsManager) GetAgentStatus() (common.AgentStatus, error) {
	manager, err := mgr.Connect()
	if err != nil {
		return common.UndefinedStatus, fmt.Errorf("failed to connect to windows service manager: %v", err)
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(serviceName)
	if err != nil {
		return common.UndefinedStatus, fmt.Errorf("failed to open windows service manager: %v", err)
	}
	defer service.Close()

	serviceStatus, err := service.Query()
	if err != nil {
		return common.UndefinedStatus, fmt.Errorf("failed to query windows service: %v", err)
	}

	if serviceStatus.State == svc.Running {
		return common.Running, nil
	} else if serviceStatus.State == svc.Stopped {
		return common.Stopped, nil
	}

	return common.UndefinedStatus, fmt.Errorf("unexpected service status: %s", windowsServiceStatusToString(serviceStatus.State))
}

// ReloadManager reloads the service manager configuration files
func (m *windowsManager) ReloadManager() error {
	return nil
}

// IsManagerEnvironment returns true if all commands required by the package manager are available
func (m *windowsManager) IsManagerEnvironment() bool {
	return true
}

// GetName returns the service manager name
func (m *windowsManager) GetName() string {
	return "windows"
}

// GetType returns the service manage type
func (m *windowsManager) GetType() ServiceManager {
	return Windows
}

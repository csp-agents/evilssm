// Copyright 2026 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package clicommand

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
	runtimeconfigMocks "github.com/aws/amazon-ssm-agent/common/runtimeconfig/mocks"
)

func TestFlushCachedCredentials_Execute_Success(t *testing.T) {
	mockClient := &runtimeconfigMocks.IIdentityRuntimeConfigClient{}
	mockClient.On("DeleteConfig").Return(nil)

	origFactory := newIdentityRuntimeConfigClient
	newIdentityRuntimeConfigClient = func() runtimeconfig.IIdentityRuntimeConfigClient {
		return mockClient
	}
	defer func() { newIdentityRuntimeConfigClient = origFactory }()

	origPerms := isRunningElevatedPermissions
	isRunningElevatedPermissions = func() error { return nil }
	defer func() { isRunningElevatedPermissions = origPerms }()

	cmd := &FlushCachedCredentialsCommand{}
	err, result := cmd.Execute(nil, nil)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != "Successfully flushed cached credentials" {
		t.Errorf("unexpected result: %v", result)
	}
	mockClient.AssertExpectations(t)
}

func TestFlushCachedCredentials_Execute_DeleteFails(t *testing.T) {
	mockClient := &runtimeconfigMocks.IIdentityRuntimeConfigClient{}
	mockClient.On("DeleteConfig").Return(fmt.Errorf("permission denied"))

	origFactory := newIdentityRuntimeConfigClient
	newIdentityRuntimeConfigClient = func() runtimeconfig.IIdentityRuntimeConfigClient {
		return mockClient
	}
	defer func() { newIdentityRuntimeConfigClient = origFactory }()

	origPerms := isRunningElevatedPermissions
	isRunningElevatedPermissions = func() error { return nil }
	defer func() { isRunningElevatedPermissions = origPerms }()

	cmd := &FlushCachedCredentialsCommand{}
	err, result := cmd.Execute(nil, nil)
	if err == nil {
		t.Error("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to flush cached credentials") {
		t.Errorf("unexpected error message: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result, got %v", result)
	}
	mockClient.AssertExpectations(t)
}

func TestFlushCachedCredentials_Execute_NotElevated(t *testing.T) {
	origPerms := isRunningElevatedPermissions
	isRunningElevatedPermissions = func() error { return fmt.Errorf("binary needs to be executed by root") }
	defer func() { isRunningElevatedPermissions = origPerms }()

	cmd := &FlushCachedCredentialsCommand{}
	err, result := cmd.Execute(nil, nil)
	if err == nil {
		t.Error("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "root") {
		t.Errorf("unexpected error message: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestFlushCachedCredentials_Execute_UnsupportedSubcommand(t *testing.T) {
	cmd := &FlushCachedCredentialsCommand{}
	err, _ := cmd.Execute([]string{"something"}, nil)
	if err == nil {
		t.Error("expected error for unsupported subcommand")
	}
	if !strings.Contains(err.Error(), "does not support subcommand") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestFlushCachedCredentials_Execute_UnsupportedParameter(t *testing.T) {
	cmd := &FlushCachedCredentialsCommand{}
	err, _ := cmd.Execute(nil, map[string][]string{"foo": {}})
	if err == nil {
		t.Error("expected error for unsupported parameter")
	}
	if !strings.Contains(err.Error(), "unknown parameter") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestFlushCachedCredentials_Name(t *testing.T) {
	cmd := &FlushCachedCredentialsCommand{}
	if cmd.Name() != "flush-cached-credentials" {
		t.Errorf("unexpected name: %v", cmd.Name())
	}
}

func TestFlushCachedCredentials_Help(t *testing.T) {
	cmd := &FlushCachedCredentialsCommand{}
	help := cmd.Help()
	if !strings.Contains(help, "flush-cached-credentials") {
		t.Error("help text should contain command name")
	}
}

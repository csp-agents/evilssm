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

// Package clicommand contains the implementation of all commands for the ssm agent cli
package clicommand

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/aws/amazon-ssm-agent/agent/cli/cliutil"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
	"github.com/aws/amazon-ssm-agent/common/utility"
)

const (
	flushCachedCredentialsCommand = "flush-cached-credentials"
)

const flushCachedCredentialsHelp = `NAME:
    {{.CommandName}}

DESCRIPTION
    Flushes the cached credentials used by the SSM Agent, forcing the agent
    to re-discover credentials on the next refresh cycle. This is useful when
    a new instance profile has been attached and you do not want to wait for
    the default cache expiry.

SYNOPSIS
    {{.SsmCliName}} {{.CommandName}}

EXAMPLES
    Command:

      {{.SsmCliName}} {{.CommandName}}

    Output:

      Successfully flushed cached credentials

OUTPUT
    Success or failure message
`

type flushCachedCredentialsHelpParams struct {
	SsmCliName  string
	CommandName string
}

var newIdentityRuntimeConfigClient = runtimeconfig.NewIdentityRuntimeConfigClient
var isRunningElevatedPermissions = utility.IsRunningElevatedPermissions

func init() {
	cliutil.Register(&FlushCachedCredentialsCommand{})
}

type FlushCachedCredentialsCommand struct {
	helpText string
}

func (c *FlushCachedCredentialsCommand) Execute(subcommands []string, parameters map[string][]string) (error, string) {
	validation := c.validateInput(subcommands, parameters)
	if len(validation) > 0 {
		return errors.New(strings.Join(validation, "\n")), ""
	}

	if err := isRunningElevatedPermissions(); err != nil {
		return err, ""
	}

	client := newIdentityRuntimeConfigClient()
	if err := client.DeleteConfig(); err != nil {
		return fmt.Errorf("failed to flush cached credentials: %v", err), ""
	}

	return nil, "Successfully flushed cached credentials"
}

func (c *FlushCachedCredentialsCommand) Help() string {
	if len(c.helpText) == 0 {
		t, _ := template.New("FlushCachedCredentialsHelp").Parse(flushCachedCredentialsHelp)
		params := flushCachedCredentialsHelpParams{cliutil.SsmCliName, flushCachedCredentialsCommand}
		buf := new(bytes.Buffer)
		t.Execute(buf, params)
		c.helpText = buf.String()
	}
	return c.helpText
}

func (FlushCachedCredentialsCommand) Name() string {
	return flushCachedCredentialsCommand
}

func (FlushCachedCredentialsCommand) validateInput(subcommands []string, parameters map[string][]string) []string {
	validation := make([]string, 0)
	if subcommands != nil && len(subcommands) > 0 {
		validation = append(validation, fmt.Sprintf("%v does not support subcommand %v", flushCachedCredentialsCommand, subcommands))
		return validation
	}

	for key := range parameters {
		validation = append(validation, fmt.Sprintf("unknown parameter %v", cliutil.FormatFlag(key)))
	}
	return validation
}

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

package iohandler

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	iomodulemock "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/iomodule/mock"
	multiwritermock "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/multiwriter/mock"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type truncateOutputTest struct {
	stdout   string
	stderr   string
	capacity int
	expected string
}

const (
	sampleSize  = 100
	longMessage = `This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
1234567890. This is a sample text. This is a sample text`
)

var testData = []truncateOutputTest{
	//{stdout, stderr, capacity, expected}
	{"", "", sampleSize, ""},
	{"sample output", "", sampleSize, "sample output"},
	{"", "sample error", sampleSize, "\n----------ERROR-------\nsample error"},
	{"sample output", "sample error", sampleSize, "sample output\n----------ERROR-------\nsample error"},
	{longMessage, "", sampleSize, "This is a sample text. This is a sample text. This is a sample text. This is \n---Output truncated---"},
	{"", longMessage, sampleSize, "\n----------ERROR-------\nThis is a sample text. This is a sample text. This is\n---Error truncated----"},
	{longMessage, longMessage, sampleSize, "This is a sampl\n---Output truncated---\n----------ERROR-------\nThis is a sampl\n---Error truncated----"},
	{
		strings.Repeat("o", ((sampleSize - len(errTitle)) / 2)), //stdout
		strings.Repeat("e", sampleSize*3),                       //stderr
		sampleSize,                                              //capacity
		"oooooooooooooooooooooooooooooooooooooo\n----------ERROR-------\neeeeeeeeeeeeeee\n---Error truncated----", //expected
	},
}

func TestTruncateOutput(t *testing.T) {
	for i, test := range testData {
		actual := TruncateOutput(test.stdout, test.stderr, test.capacity)
		assert.Equal(t, test.expected, actual, "failed test case: %v", i)
	}
}

var logger = log.NewMockLog()

func TestRegisterOutputSource(t *testing.T) {
	mockDocumentIOMultiWriter := new(multiwritermock.MockDocumentIOMultiWriter)
	mockContext := context.NewMockDefault()

	mockDocumentIOMultiWriter.On("AddWriter", mock.Anything).Times(2)
	wg := new(sync.WaitGroup)
	mockDocumentIOMultiWriter.On("GetWaitGroup").Return(wg)

	// Add 2 to WaitGroup to simulate two AddWriter calls
	wg.Add(2)

	// Create multiple test IOModules
	testModule1 := new(iomodulemock.MockIOModule)
	testModule1.On("Read", mockContext, mock.Anything, mock.AnythingOfType("int")).Return()
	testModule2 := new(iomodulemock.MockIOModule)
	testModule2.On("Read", mockContext, mock.Anything, mock.AnythingOfType("int")).Return()

	output := NewDefaultIOHandler(mockContext, contracts.IOConfiguration{})

	output.RegisterOutputSource(mockDocumentIOMultiWriter, testModule1, testModule2)

	// Sleep a bit to allow threads to finish in RegisterOutputSource to check WaitGroup
	time.Sleep(250 * time.Millisecond)
}

func TestSucceeded(t *testing.T) {
	output := DefaultIOHandler{}

	output.MarkAsSucceeded()

	assert.Equal(t, output.ExitCode, 0)
	assert.Equal(t, output.Status, contracts.ResultStatusSuccess)
	assert.True(t, output.Status.IsSuccess())
	assert.False(t, output.Status.IsReboot())
}

func TestFailed(t *testing.T) {
	output := DefaultIOHandler{}

	output.MarkAsFailed(fmt.Errorf("Error message"))

	assert.Equal(t, output.ExitCode, 1)
	assert.Equal(t, output.Status, contracts.ResultStatusFailed)
	assert.Contains(t, output.GetStderr(), "Error message")
	assert.False(t, output.Status.IsSuccess())
	assert.False(t, output.Status.IsReboot())
}

func TestMarkAsInProgress(t *testing.T) {
	output := DefaultIOHandler{}

	output.MarkAsInProgress()

	assert.Equal(t, output.ExitCode, 0)
	assert.Equal(t, output.Status, contracts.ResultStatusInProgress)
	assert.False(t, output.Status.IsSuccess())
	assert.False(t, output.Status.IsReboot())
}

func TestMarkAsSuccessWithReboot(t *testing.T) {
	output := DefaultIOHandler{}

	output.MarkAsSuccessWithReboot()

	assert.Equal(t, output.ExitCode, 0)
	assert.Equal(t, output.Status, contracts.ResultStatusSuccessAndReboot)
	assert.True(t, output.Status.IsSuccess())
	assert.True(t, output.Status.IsReboot())
}

func TestAppendInfo(t *testing.T) {
	output := DefaultIOHandler{}

	output.AppendInfo("Info message")
	output.AppendInfo("Second entry")

	assert.Contains(t, output.GetStdout(), "Info message")
	assert.Contains(t, output.GetStdout(), "Second entry")
}

func TestAppendSpecialChars(t *testing.T) {
	output := DefaultIOHandler{}

	var testString = "%v`~!@#$%^&*()-_=+[{]}|\\;:'\",<.>/?"
	output.AppendInfo(testString)
	output.AppendError(testString)

	assert.Contains(t, output.GetStdout(), testString)
	assert.Contains(t, output.GetStderr(), testString)
}

func TestAppendFormat(t *testing.T) {
	output := DefaultIOHandler{}

	var testString = "%v`~!@#$%^&*()-_=+[{]}|\\;:'\",<.>/?%%"

	// The first % is a %v - a variable to be replaced and we provided a value for it.
	// The second % isn't escaped and is treated as a fmt parameter, but no value is provided for it.
	// The double %% is an escaped single literal %.
	var testStringFormatted = "foo`~!@#$%!^(MISSING)&*()-_=+[{]}|\\;:'\",<.>/?%"
	output.AppendInfof(testString, "foo")
	output.AppendErrorf(testString, "foo")

	assert.Contains(t, output.GetStdout(), testStringFormatted)
	assert.Contains(t, output.GetStderr(), testStringFormatted)
}

func TestCloseWithBothWriters(t *testing.T) {
	// Create a mock context
	mockContext := context.NewMockDefault()

	// Create a mock multi-writer for stdout
	mockStdoutWriter := new(multiwritermock.MockDocumentIOMultiWriter)
	mockStderrWriter := new(multiwritermock.MockDocumentIOMultiWriter)

	// Set expectations for Close() method on both writers
	mockStdoutWriter.On("Close").Return(nil)
	mockStderrWriter.On("Close").Return(nil)

	output := NewDefaultIOHandler(mockContext, contracts.IOConfiguration{})
	output.StderrWriter = mockStderrWriter
	output.StdoutWriter = mockStdoutWriter

	// Create DefaultIOHandler with mock writers

	// Call Close method
	output.Close()

	// Verify that Close was called on both writers
	mockStdoutWriter.AssertCalled(t, "Close")
	mockStderrWriter.AssertCalled(t, "Close")

}

func TestStringMethodWithSpecialExitCode(t *testing.T) {
	// Test scenario where exit code is 168 (special success code)
	output := DefaultIOHandler{
		ExitCode: contracts.ExitWithSuccess,
		stdout:   "Standard output message",
		stderr:   "Error message that should be removed",
	}

	result := output.String()

	// Verify that stderr is empty for this special exit code
	assert.Equal(t, "Standard output message", result)
}

func TestSetOutputWithSimpleString(t *testing.T) {
	// Create a mock context
	mockContext := context.NewMockDefault()

	// Create a new DefaultIOHandler
	output := NewDefaultIOHandler(mockContext, contracts.IOConfiguration{})

	// Set a simple string output
	testOutput := "Test Output String"
	output.SetOutput(testOutput)

	// Verify the output is correctly set
	assert.Equal(t, testOutput, output.GetOutput())
}

func TestSetOutputWithComplexType(t *testing.T) {
	// Create a mock context
	mockContext := context.NewMockDefault()

	// Create a new DefaultIOHandler
	output := NewDefaultIOHandler(mockContext, contracts.IOConfiguration{})

	// Create a complex type (map)
	testOutput := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}
	output.SetOutput(testOutput)

	// Verify the output is correctly set
	assert.Equal(t, testOutput, output.GetOutput())
}

func TestSetOutputNil(t *testing.T) {
	// Create a mock context
	mockContext := context.NewMockDefault()

	// Create a new DefaultIOHandler with some predefined stdout and stderr
	output := NewDefaultIOHandler(mockContext, contracts.IOConfiguration{})
	output.SetStdout("Standard Output")
	output.SetStderr("Standard Error")

	// Set nil output
	output.SetOutput(nil)

	// Verify the output falls back to stdout + stderr
	expectedOutput := output.String()
	assert.Equal(t, expectedOutput, output.GetOutput())
}

func TestSetStatusVariousStates(t *testing.T) {
	// Test cases covering different result status scenarios
	testCases := []struct {
		name           string
		inputStatus    contracts.ResultStatus
		expectedStatus contracts.ResultStatus
	}{
		{
			name:           "Set Status to Success",
			inputStatus:    contracts.ResultStatusSuccess,
			expectedStatus: contracts.ResultStatusSuccess,
		},
		{
			name:           "Set Status to Failed",
			inputStatus:    contracts.ResultStatusFailed,
			expectedStatus: contracts.ResultStatusFailed,
		},
		{
			name:           "Set Status to InProgress",
			inputStatus:    contracts.ResultStatusInProgress,
			expectedStatus: contracts.ResultStatusInProgress,
		},
		{
			name:           "Set Status to Cancelled",
			inputStatus:    contracts.ResultStatusCancelled,
			expectedStatus: contracts.ResultStatusCancelled,
		},
		{
			name:           "Set Status to SuccessAndReboot",
			inputStatus:    contracts.ResultStatusSuccessAndReboot,
			expectedStatus: contracts.ResultStatusSuccessAndReboot,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new DefaultIOHandler
			output := DefaultIOHandler{}

			// Set the status
			output.SetStatus(tc.inputStatus)

			// Verify the status was set correctly
			assert.Equal(t, tc.expectedStatus, output.Status,
				"Status should match the input status")
		})
	}
}

func TestGetExitCodeDefault(t *testing.T) {
	// Create a new DefaultIOHandler
	output := DefaultIOHandler{
		ExitCode: 0, // Default exit code
	}

	// Verify the exit code is retrieved correctly
	assert.Equal(t, 0, output.GetExitCode(),
		"GetExitCode should return the default exit code")
}

func TestGetIOConfigBasic(t *testing.T) {
	// Create a mock context
	mockContext := context.NewMockDefault()

	// Define a sample IOConfiguration
	sampleConfig := contracts.IOConfiguration{
		OrchestrationDirectory: "/test/orchestration",
		OutputS3BucketName:     "test-bucket",
		OutputS3KeyPrefix:      "test-prefix",
		CloudWatchConfig: contracts.CloudWatchConfiguration{
			LogGroupName:    "test-log-group",
			LogStreamPrefix: "test-stream-prefix",
		},
	}

	// Create DefaultIOHandler with the sample configuration
	output := NewDefaultIOHandler(mockContext, sampleConfig)

	// Retrieve the IOConfig
	retrievedConfig := output.GetIOConfig()

	// Assert that the retrieved configuration matches the original
	assert.Equal(t, sampleConfig, retrievedConfig, "Retrieved IOConfig should match the original configuration")
}

func TestGetStderrBasic(t *testing.T) {
	// Create a new DefaultIOHandler
	output := DefaultIOHandler{
		stderr: "Sample error message",
	}

	// Verify stderr retrieval
	assert.Equal(t, "Sample error message", output.GetStderr(),
		"GetStderr should return the exact stderr string")
}

func TestGetStdoutWriter_Initialized(t *testing.T) {
	// Create a mock context
	mockContext := context.NewMockDefault()

	// Create an IO configuration
	ioConfig := contracts.IOConfiguration{
		OrchestrationDirectory: "/test/path",
	}

	// Create a new DefaultIOHandler
	output := NewDefaultIOHandler(mockContext, ioConfig)

	// Initialize the handler
	output.Init()

	// Get the stdout writer
	stdoutWriter := output.GetStdoutWriter()

	// Assert that the writer is not nil
	assert.NotNil(t, stdoutWriter, "GetStdoutWriter should return a non-nil writer")
}

func TestGetStderrWriterBasic(t *testing.T) {
	// Create a mock context
	mockContext := context.NewMockDefault()

	// Create an IOConfiguration
	ioConfig := contracts.IOConfiguration{
		OrchestrationDirectory: "/test/path",
	}

	// Create a new DefaultIOHandler
	output := NewDefaultIOHandler(mockContext, ioConfig)

	// Initialize the handler to set up writers
	output.Init()

	// Get the stderr writer
	stderrWriter := output.GetStderrWriter()

	// Assert that the writer is not nil
	assert.NotNil(t, stderrWriter, "StderrWriter should not be nil after initialization")
}

func TestSetExitCode_BasicScenario(t *testing.T) {
	// Create a new DefaultIOHandler
	output := DefaultIOHandler{}

	// Set an exit code
	testExitCode := 42
	output.SetExitCode(testExitCode)

	// Verify the exit code was set correctly
	assert.Equal(t, testExitCode, output.ExitCode,
		"SetExitCode should correctly set the exit code")
}

func TestMarkAsCancelled(t *testing.T) {
	// Create a new DefaultIOHandler
	output := DefaultIOHandler{}

	// Call MarkAsCancelled
	output.MarkAsCancelled()

	// Verify exit code is set to 1
	assert.Equal(t, output.ExitCode, 1, "Exit code should be set to 1 when cancelled")

	// Verify status is set to Cancelled
	assert.Equal(t, output.Status, contracts.ResultStatusCancelled, "Status should be set to Cancelled")

	// Verify status properties
	assert.False(t, output.Status.IsSuccess(), "Cancelled status should not be considered successful")
	assert.False(t, output.Status.IsReboot(), "Cancelled status should not trigger reboot")
}

func TestMarkAsShutdown(t *testing.T) {
	// Create a new DefaultIOHandler
	output := DefaultIOHandler{}

	// Call MarkAsShutdown
	output.MarkAsShutdown()

	// Verify exit code is set to 1
	assert.Equal(t, output.ExitCode, 1, "Exit code should be 1 for shutdown")

	// Verify status is set to Cancelled
	assert.Equal(t, output.Status, contracts.ResultStatusCancelled, "Status should be Cancelled")

	// Additional checks for shutdown state
	assert.False(t, output.Status.IsSuccess(), "Shutdown status should not be considered successful")
	assert.False(t, output.Status.IsReboot(), "Shutdown status should not trigger reboot")
}

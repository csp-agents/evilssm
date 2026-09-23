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

package emitter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
)

func TestNewEmitter(t *testing.T) {
	mockLog := log.NewMockLog()
	emitter := NewEmitter(mockLog).(*emitter)
	defer emitter.Close()

	assert.NotNil(t, emitter, "Expected non-nil emitter")
	assert.Equal(t, uint64(200*1024), emitter.maxFileSize, "Expected maxFileSize to be 200KB")
	assert.Equal(t, 20*time.Second, emitter.autoCloseDuration, "Expected autoCloseDuration to be 20s, got %v", emitter.autoCloseDuration)
}

func TestGetTelemetryFilePath(t *testing.T) {
	namespace := "test-namespace"
	expected := filepath.Join(appconfig.TelemetryDataStorePath, "preingestion", "test-namespace.jsonl")

	result := GetTelemetryFilePath(namespace)
	assert.Equal(t, expected, result, "Expected path %s, got %s", expected, result)
}

func TestEmitter_Emit(t *testing.T) {
	// Temporarily override the TelemetryPreIngestionDir
	originalPath := TelemetryPreIngestionDir
	TelemetryPreIngestionDir = t.TempDir()
	defer func() { TelemetryPreIngestionDir = originalPath }()

	mockLog := log.NewMockLog()
	emitter := NewEmitter(mockLog)
	defer emitter.Close()

	tests := []struct {
		name      string
		namespace string
		message   Message
		wantErr   bool
	}{
		{
			name:      "should_emitSuccessfully_whenValidInput",
			namespace: "test1",
			message:   Message{Type: LOG, Payload: "test payload"},
			wantErr:   false,
		},
		{
			name:      "should_handleEmptyNamespace_whenNamespaceEmpty",
			namespace: "",
			message:   Message{Type: LOG, Payload: "test payload"},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := emitter.Emit(tt.namespace, tt.message)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if !tt.wantErr {
				// flush the buffer
				err = emitter.Flush()
				require.NoError(t, err)

				// Verify file contents
				filePath := GetTelemetryFilePath(tt.namespace)
				content, err := os.ReadFile(filePath)
				require.NoError(t, err, "Failed to read emitted file")

				var decoded Message
				err = json.Unmarshal(content, &decoded)
				require.NoError(t, err, "Failed to decode JSON: %v", err)

				assert.Equal(t, tt.message, decoded, "Message mismatch")
			}
		})
	}
}

func TestEmitter_EmitMultipleMessages(t *testing.T) {
	// Temporarily override the TelemetryPreIngestionDir
	originalPath := TelemetryPreIngestionDir
	TelemetryPreIngestionDir = t.TempDir()
	defer func() { TelemetryPreIngestionDir = originalPath }()

	mockLog := log.NewMockLog()
	emitter := NewEmitter(mockLog).(*emitter)
	defer emitter.Close()

	namespace := "test-multiple"
	messages := []Message{
		{
			Type:    LOG,
			Payload: "first message",
		},
		{
			Type:    LOG,
			Payload: "second message",
		},
		{
			Type:    LOG,
			Payload: "third message",
		},
	}

	// Emit multiple messages
	for _, msg := range messages {
		err := emitter.Emit(namespace, msg)
		require.NoError(t, err, "Failed to emit message")
	}

	// Flush to ensure all messages are written
	err := emitter.Flush()
	require.NoError(t, err, "Failed to flush messages")

	// Read and verify file contents
	filePath := GetTelemetryFilePath(namespace)
	content, err := os.ReadFile(filePath)
	require.NoError(t, err, "Failed to read emitted file")

	// Split content by newlines to get individual messages
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	require.Equal(t, len(messages), len(lines), "Number of emitted messages doesn't match")

	// Verify each message
	for i, line := range lines {
		var decoded Message
		err = json.Unmarshal([]byte(line), &decoded)
		require.NoError(t, err, "Failed to decode JSON line %d: %v", i, err)
		assert.Equal(t, messages[i], decoded, "Message mismatch at index %d", i)
	}

	// Verify file size is within limits
	fileInfo, err := os.Stat(filePath)
	require.NoError(t, err, "Failed to get file info")
	assert.True(t, fileInfo.Size() <= int64(emitter.maxFileSize),
		"File size exceeds maximum allowed size")
}

func TestEmitter_EmitMultipleMessagesConcurrently(t *testing.T) {
	// Temporarily override the TelemetryPreIngestionDir
	originalPath := TelemetryPreIngestionDir
	TelemetryPreIngestionDir = t.TempDir()
	defer func() { TelemetryPreIngestionDir = originalPath }()

	mockLog := log.NewMockLog()
	emitter := NewEmitter(mockLog).(*emitter)
	defer emitter.Close()

	namespace := "test-multiple"
	var wg sync.WaitGroup

	for i := range 10 {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			msg := Message{
				Type:    LOG,
				Payload: fmt.Sprintf("concurrent message %d", index),
			}
			err := emitter.Emit(namespace, msg)
			assert.NoError(t, err, "Failed to emit concurrent message %d", index)
		}(i)
	}

	wg.Wait()
	// Flush to ensure all messages are written
	err := emitter.Flush()
	require.NoError(t, err, "Failed to flush messages")

	filePath := GetTelemetryFilePath(namespace)
	content, err := os.ReadFile(filePath)
	require.NoError(t, err, "Failed to read emitted file")
	// Split content by newlines to get individual messages
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	require.Equal(t, 10, len(lines), "Number of emitted messages doesn't match")

	// Verify file size is within limits
	fileInfo, err := os.Stat(filePath)
	require.NoError(t, err, "Failed to get file info")
	assert.True(t, fileInfo.Size() <= int64(emitter.maxFileSize),
		"File size exceeds maximum allowed size")
}

func TestEmitter_Close(t *testing.T) {
	// Temporarily override the TelemetryPreIngestionDir
	originalPath := TelemetryPreIngestionDir
	TelemetryPreIngestionDir = t.TempDir()
	defer func() { TelemetryPreIngestionDir = originalPath }()

	mockLog := log.NewMockLog()
	emitter := NewEmitter(mockLog).(*emitter)

	// Emit some data to create files
	namespace := "test-close"
	message := Message{Type: LOG, Payload: "test payload"}
	err := emitter.Emit(namespace, message)
	require.NoError(t, err, "Failed to emit test data")

	// Test Close
	err = emitter.Close()
	require.NoError(t, err)

	// Verify that files are closed
	emitter.openNamespaceFileMtx.RLock()
	filesCount := len(emitter.openNamespaceFiles)
	emitter.openNamespaceFileMtx.RUnlock()
	assert.Equal(t, 0, filesCount, "Expected all files to be closed")

	// verify content
	filePath := GetTelemetryFilePath(namespace)
	content, err := os.ReadFile(filePath)
	require.NoError(t, err, "Failed to read emitted file")

	var decoded Message
	err = json.Unmarshal(content, &decoded)
	require.NoError(t, err, "Failed to decode JSON: %v", err)
	assert.Equal(t, message, decoded, "Message mismatch")
}

func TestEmitter_AutoClose(t *testing.T) {
	// Setup with shorter duration for testing
	mockLog := log.NewMockLog()
	emitter := NewEmitter(mockLog).(*emitter)
	emitter.autoCloseDuration = time.Millisecond * 100 // Short duration for testing

	// Temporarily override the TelemetryPreIngestionDir
	originalPath := TelemetryPreIngestionDir
	TelemetryPreIngestionDir = t.TempDir()
	defer func() { TelemetryPreIngestionDir = originalPath }()

	// Emit data
	namespace := "test-autoclose"
	message := Message{Type: LOG, Payload: "test payload"}
	err := emitter.Emit(namespace, message)
	if err != nil {
		t.Fatalf("Failed to emit test data: %v", err)
	}

	// Verify that the file is open
	emitter.openNamespaceFileMtx.RLock()
	_, fileExists := emitter.openNamespaceFiles[namespace]
	emitter.openNamespaceFileMtx.RUnlock()
	assert.True(t, fileExists, "Expected file to be open")

	// verify that the auto-close timer starts running
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		emitter.autoCloseTimersMtx.RLock()
		_, ok := emitter.autoCloseTimers[namespace]
		emitter.autoCloseTimersMtx.RUnlock()
		assert.True(c, ok, "Expected auto-close timer to be running")
	}, 50*time.Millisecond, 5*time.Millisecond)

	// Wait for auto-close
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		emitter.openNamespaceFileMtx.RLock()
		_, fileExists = emitter.openNamespaceFiles[namespace]
		emitter.openNamespaceFileMtx.RUnlock()
		assert.False(c, fileExists, "Expected file to be auto-closed")
	}, 200*time.Millisecond, 20*time.Millisecond)

	// verify that the auto-close timer is not running
	emitter.autoCloseTimersMtx.RLock()
	_, ok := emitter.autoCloseTimers[namespace]
	emitter.autoCloseTimersMtx.RUnlock()
	assert.False(t, ok, "Expected auto-close timer to not be running")
}

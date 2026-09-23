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
package collector

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/fileutil/advisorylock"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/aws/amazon-ssm-agent/common/telemetry/emitter"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConsumer(t *testing.T) {
	mockCtx := context.NewMockDefault()

	consumer := newConsumer(mockCtx, 5)

	assert.NotNil(t, consumer)
	assert.Equal(t, 5, consumer.pollPeriodSeconds)
	assert.NotNil(t, consumer.onMessageChan)
}

func writeTelemetryForNamespace(t *testing.T, namespace string) (string, []emitter.Message) {
	t.Helper()

	nsFile := filepath.Join(emitter.TelemetryPreIngestionDir, fmt.Sprintf("%s.jsonl", namespace))
	f, err := os.OpenFile(nsFile, os.O_RDWR|os.O_CREATE, 0600)
	require.NoError(t, err)
	defer f.Close()

	expectedMessages := make([]emitter.Message, 0, 100)
	for i := range 100 {
		// Create test message
		testMessage := emitter.Message{
			Type:    emitter.LOG,
			Payload: fmt.Sprintf("doesn't matter %s %d", namespace, i),
		}
		expectedMessages = append(expectedMessages, testMessage)

		messageBytes, _ := json.Marshal(testMessage)
		_, err := f.Write(append(messageBytes, '\n'))
		assert.NoError(t, err)
	}
	return nsFile, expectedMessages
}

func TestPoll_Success(t *testing.T) {
	// Store original directory and restore it after test
	originalDir := emitter.TelemetryPreIngestionDir
	defer func() {
		emitter.TelemetryPreIngestionDir = originalDir
	}()

	// Set some random non-existent directory
	emitter.TelemetryPreIngestionDir = t.TempDir()

	// Create temporary test file
	ns1File, expectedMessagesNs1 := writeTelemetryForNamespace(t, "test-namespace-1")
	ns2File, expectedMessagesNs2 := writeTelemetryForNamespace(t, "test-namespace-2")

	mockCtx := context.NewMockDefault()

	consumer := newConsumer(mockCtx, 5)

	// listen for the messages
	done := make(chan bool)
	go func() {
		// files will be processed alphabetically
		for _, expectedMessage := range expectedMessagesNs1 {
			msg := <-consumer.getMessage()
			assert.Equal(t, "test-namespace-1", msg.namespace) // should match the file name
			assert.Equal(t, expectedMessage, msg.message)
		}
		for _, expectedMessage := range expectedMessagesNs2 {
			msg := <-consumer.getMessage()
			assert.Equal(t, "test-namespace-2", msg.namespace) // should match the file name
			assert.Equal(t, expectedMessage, msg.message)
		}
		done <- true
	}()

	err := consumer.poll()
	assert.NoError(t, err)

	select {
	case <-done:
		// verify that the files are truncated
		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			fi, err := os.Stat(ns1File)
			assert.NoError(c, err)
			assert.Equal(c, int64(0), fi.Size())
			fi, err = os.Stat(ns2File)
			assert.NoError(c, err)
			assert.Equal(c, int64(0), fi.Size())
		}, 200*time.Millisecond, 10*time.Millisecond)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for message")
	}

	// assert no new messages
	err = consumer.poll()
	assert.NoError(t, err)
	select {
	case <-consumer.getMessage():
		t.Fatal("received unexpected message")
	case <-time.After(10 * time.Millisecond):
	}
}

// TestPoll_NonExistentDirectory tests that the poll method should not fail if the
// pre-ingestion directory is not present
func TestPoll_NonExistentDirectory(t *testing.T) {
	// Store original directory and restore it after test
	originalDir := emitter.TelemetryPreIngestionDir
	defer func() {
		emitter.TelemetryPreIngestionDir = originalDir
	}()

	// Set some random non-existent directory
	emitter.TelemetryPreIngestionDir = filepath.Join(t.TempDir(), "qdqwqwdqd")

	mockCtx := context.NewMockDefault()

	consumer := newConsumer(mockCtx, 5)

	err := consumer.poll()
	assert.NoError(t, err)
}

func TestProcessNamespaceFile_InvalidJSON(t *testing.T) {
	mockCtx := context.NewMockDefault()

	consumer := newConsumer(mockCtx, 5)

	// Create temporary test file with invalid JSON
	tmpDir := t.TempDir()
	nsFile := filepath.Join(tmpDir, "test-namespace.jsonl")
	f, err := os.OpenFile(nsFile, os.O_RDWR|os.O_CREATE, 0600)
	require.NoError(t, err)
	defer f.Close()

	// Create test message
	testMessage := emitter.Message{
		Type:    emitter.LOG,
		Payload: "doesn't matter ",
	}

	messageBytes, _ := json.Marshal(testMessage)
	_, err = f.Write(append(messageBytes, '\n'))
	assert.NoError(t, err)
	_, err = f.Write([]byte("invalid json\ninvalid json 2\n\n   \n"))
	assert.NoError(t, err)

	// Process the one valid message
	done := make(chan bool)
	go func() {
		msg := <-consumer.getMessage()
		assert.Equal(t, "test-namespace", msg.namespace)
		assert.Equal(t, testMessage, msg.message)
		done <- true
	}()

	err = consumer.processNamespaceFile(nsFile)
	assert.NoError(t, err)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for message")
	}

	// assert no other messages
	select {
	case <-consumer.getMessage():
		t.Fatal("received unexpected message")
	case <-time.After(10 * time.Millisecond):
	}

	// verify that the file is truncated
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		fi, err := os.Stat(nsFile)
		assert.NoError(c, err)
		assert.Equal(c, int64(0), fi.Size())
	}, 200*time.Millisecond, 10*time.Millisecond)
}

// TestProcessNamespaceFile_Locking makes sure that the processNamespaceFile method
// acquires the advisory lock
func TestProcessNamespaceFile_Locking(t *testing.T) {
	// Setup
	mockCtx := context.NewMockDefault()

	consumer := newConsumer(mockCtx, 5)

	// Create temporary test file
	tmpDir := t.TempDir()
	nsFile := filepath.Join(tmpDir, "test-namespace.jsonl")

	f, err := os.OpenFile(nsFile, os.O_RDWR|os.O_CREATE, 0600)
	assert.NoError(t, err)
	defer f.Close()

	// Create test message
	testMessage := emitter.Message{
		Type:    emitter.LOG,
		Payload: "doesn't matter",
	}

	messageBytes, _ := json.Marshal(testMessage)
	_, err = f.Write(append(messageBytes, '\n'))
	assert.NoError(t, err)

	// listen for the messages
	done := make(chan bool)
	go func() {
		msg := <-consumer.getMessage()
		assert.Equal(t, "test-namespace", msg.namespace)
		assert.Equal(t, testMessage, msg.message)
		done <- true
	}()

	// acquire the lock
	err = advisorylock.RLock(f, time.Second)
	assert.NoError(t, err)

	processingDone := make(chan struct{})
	go func() {
		t.Helper()

		// this should block
		err := consumer.processNamespaceFile(nsFile)
		if err != nil {
			panic(fmt.Sprintf("processNamespaceFile failed: %v", err))
		}
		close(processingDone)
	}()

	select {
	case <-processingDone:
		t.Fatal("Did not block as expected")
	case <-time.After(10 * time.Millisecond):
	}

	advisorylock.Unlock(f)

	// processing should complete after the file is unlocked
	select {
	case <-done:
		// needs some time for the run to finish
		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			// verify that the file is truncated
			fi, err := os.Stat(nsFile)
			assert.NoError(c, err)
			assert.Equal(c, int64(0), fi.Size())
		}, 200*time.Millisecond, 10*time.Millisecond)
	case <-time.After(time.Second):
		t.Fatal("Unexpectedly still blocked")
	}

	// assert no new messages
	err = consumer.processNamespaceFile(nsFile)
	assert.NoError(t, err)
	select {
	case <-consumer.getMessage():
		t.Fatal("received unexpected message")
	case <-time.After(10 * time.Millisecond):
	}
}

// TestProcessNamespaceFile_LockingTimeout tests that the [advisorylock.Lock] times out
// after [advisoryLockTimeoutSeconds].
func TestProcessNamespaceFile_LockingTimeout(t *testing.T) {
	// Setup
	mockCtx := context.NewMockDefault()

	consumer := newConsumer(mockCtx, 5)

	// Create temporary test file
	tmpDir := t.TempDir()
	nsFile := filepath.Join(tmpDir, "test-namespace.jsonl")

	f, err := os.OpenFile(nsFile, os.O_RDWR|os.O_CREATE, 0600)
	assert.NoError(t, err)
	defer f.Close()

	// Create test message
	testMessage := emitter.Message{
		Type:    emitter.LOG,
		Payload: "doesn't matter",
	}

	messageBytes, _ := json.Marshal(testMessage)
	_, err = f.Write(append(messageBytes, '\n'))
	assert.NoError(t, err)

	err = advisorylock.RLock(f, time.Second)
	assert.NoError(t, err)
	defer advisorylock.Unlock(f)

	processingDone := make(chan struct{})
	go func() {
		t.Helper()
		defer close(processingDone)
		err := consumer.processNamespaceFile(nsFile)
		assert.ErrorContains(t, err, "timed out")
	}()

	select {
	case <-processingDone:
		t.Fatal("Did not block as expected")
	case <-time.After(4900 * time.Millisecond):
	}

	<-processingDone
}

// TestStart tests that start method starts the poll job
func TestStart(t *testing.T) {
	// Store original directory and restore it after test
	originalDir := emitter.TelemetryPreIngestionDir
	defer func() {
		emitter.TelemetryPreIngestionDir = originalDir
	}()

	// Set some random non-existent directory
	emitter.TelemetryPreIngestionDir = t.TempDir()

	mockCtx := context.NewMockDefault()

	consumer := newConsumer(mockCtx, 5)

	nsFile := filepath.Join(emitter.TelemetryPreIngestionDir, "test-namespace.jsonl")
	f, err := os.OpenFile(nsFile, os.O_RDWR|os.O_CREATE, 0600)
	assert.NoError(t, err)
	defer f.Close()

	// Create test message
	testMessage := emitter.Message{
		Type:    emitter.LOG,
		Payload: "doesn't matter",
	}

	messageBytes, _ := json.Marshal(testMessage)
	_, err = f.Write(append(messageBytes, '\n'))
	assert.NoError(t, err)

	// listen for the messages
	done := make(chan bool)
	go func() {
		msg := <-consumer.getMessage()
		assert.Equal(t, "test-namespace", msg.namespace)
		assert.Equal(t, testMessage, msg.message)
		done <- true
	}()

	consumer.start()
	defer consumer.stop()

	consumer.consumerJob.SkipWait <- true

	select {
	case <-done:
		// needs some time for the run to finish
		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			// verify that the file is truncated
			fi, err := f.Stat()
			assert.NoError(c, err)
			assert.Equal(c, int64(0), fi.Size())
		}, 200*time.Millisecond, 10*time.Millisecond)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

// TestStop tests that the polling method closes cleanly and fast
func TestStop(t *testing.T) {
	// Store original directory and restore it after test
	originalDir := emitter.TelemetryPreIngestionDir
	defer func() {
		emitter.TelemetryPreIngestionDir = originalDir
	}()

	// Set some random non-existent directory
	emitter.TelemetryPreIngestionDir = t.TempDir()

	// write some telemetry
	nsFile, _ := writeTelemetryForNamespace(t, "test-namespace")

	mockCtx := context.NewMockDefault()

	consumer := newConsumer(mockCtx, 5)
	consumer.stopSignal = make(chan bool)

	// Start polling. It should block as the receiver is not ready
	done := make(chan bool)
	go func() {
		err := consumer.poll()
		assert.NoError(t, err)
		done <- true
	}()

	select {
	case <-done:
		t.Fatal("Did not block as expected")
	case <-time.After(200 * time.Millisecond):
	}

	// Stop the consumer
	consumer.stop()

	select {
	case <-done:
		// needs some time for the run to finish
		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			// verify that the file is truncated
			fi, err := os.Stat(nsFile)
			assert.NoError(c, err)
			assert.Equal(c, int64(0), fi.Size())
		}, 200*time.Millisecond, 10*time.Millisecond)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Poll did not stop as expected")
	}

	// the channel should be closed
	assert.Panics(t, func() {
		close(consumer.stopSignal)
	})

	// Ensure stop is idempotent
	consumer.stop()
}

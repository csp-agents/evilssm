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

package mocks

import (
	"errors"
	"testing"

	"github.com/aws/amazon-ssm-agent/common/telemetry/emitter"
	"github.com/stretchr/testify/assert"
)

func TestMockEmitter_Emit(t *testing.T) {
	mockEmitter := NewMockEmitter()

	// Test successful emit
	namespace := "test-namespace"
	message := emitter.Message{
		Type:    "test-type",
		Payload: "test-payload",
	}

	err := mockEmitter.Emit(namespace, message)
	assert.NoError(t, err)

	// Verify message was stored
	messages := mockEmitter.GetMessages(namespace)
	assert.Len(t, messages, 1)
	assert.Equal(t, message, messages[0])

	// Test error case
	expectedErr := errors.New("emit error")
	mockEmitter.SetEmitError(expectedErr)

	err = mockEmitter.Emit(namespace, message)
	assert.Equal(t, expectedErr, err)
}

func TestMockEmitter_Flush(t *testing.T) {
	mockEmitter := NewMockEmitter()

	// Test successful flush
	err := mockEmitter.Flush()
	assert.NoError(t, err)
	assert.True(t, mockEmitter.FlushCalled)

	// Test error case
	mockEmitter.Reset()
	expectedErr := errors.New("flush error")
	mockEmitter.SetFlushError(expectedErr)

	err = mockEmitter.Flush()
	assert.Equal(t, expectedErr, err)
	assert.True(t, mockEmitter.FlushCalled)
}

func TestMockEmitter_Close(t *testing.T) {
	mockEmitter := NewMockEmitter()

	// Test successful close
	err := mockEmitter.Close()
	assert.NoError(t, err)
	assert.True(t, mockEmitter.CloseCalled)

	// Test error case
	mockEmitter.Reset()
	expectedErr := errors.New("close error")
	mockEmitter.SetCloseError(expectedErr)

	err = mockEmitter.Close()
	assert.Equal(t, expectedErr, err)
	assert.True(t, mockEmitter.CloseCalled)
}

func TestMockEmitter_GetAllMessages(t *testing.T) {
	mockEmitter := NewMockEmitter()

	// Add messages to multiple namespaces
	namespace1 := "namespace1"
	namespace2 := "namespace2"

	message1 := emitter.Message{Type: "type1", Payload: "payload1"}
	message2 := emitter.Message{Type: "type2", Payload: "payload2"}
	message3 := emitter.Message{Type: "type3", Payload: "payload3"}

	mockEmitter.Emit(namespace1, message1)
	mockEmitter.Emit(namespace1, message2)
	mockEmitter.Emit(namespace2, message3)

	// Get all messages
	allMessages := mockEmitter.GetAllMessages()

	// Verify messages
	assert.Len(t, allMessages, 2)
	assert.Len(t, allMessages[namespace1], 2)
	assert.Len(t, allMessages[namespace2], 1)

	assert.Equal(t, message1, allMessages[namespace1][0])
	assert.Equal(t, message2, allMessages[namespace1][1])
	assert.Equal(t, message3, allMessages[namespace2][0])
}

func TestMockEmitter_Reset(t *testing.T) {
	mockEmitter := NewMockEmitter()

	// Set up state
	namespace := "test-namespace"
	message := emitter.Message{Type: "test-type", Payload: "test-payload"}
	mockEmitter.Emit(namespace, message)
	mockEmitter.Flush()
	mockEmitter.SetEmitError(errors.New("emit error"))

	// Reset
	mockEmitter.Reset()

	// Verify state was reset
	assert.Empty(t, mockEmitter.GetAllMessages())
	assert.False(t, mockEmitter.FlushCalled)
	assert.False(t, mockEmitter.CloseCalled)

	// Verify errors were reset
	err := mockEmitter.Emit(namespace, message)
	assert.NoError(t, err)
}

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
	"sync"

	"github.com/aws/amazon-ssm-agent/common/telemetry/emitter"
)

// MockEmitter is a mock implementation of [emitter.Emitter] for testing
type MockEmitter struct {
	// Messages stores all emitted messages by namespace
	Messages map[string][]emitter.Message

	// Tracks if Flush was called
	FlushCalled bool

	// Tracks if Close was called
	CloseCalled bool

	// Errors to return for each method
	EmitError  error
	FlushError error
	CloseError error

	// Mutex to protect concurrent access
	mutex sync.RWMutex
}

// NewMockEmitter creates a new instance of MockEmitter
func NewMockEmitter() *MockEmitter {
	return &MockEmitter{
		Messages:    make(map[string][]emitter.Message),
		FlushCalled: false,
		CloseCalled: false,
	}
}

// Emit records the message in the Messages map
func (m *MockEmitter) Emit(namespace string, message emitter.Message) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.EmitError != nil {
		return m.EmitError
	}

	if _, exists := m.Messages[namespace]; !exists {
		m.Messages[namespace] = make([]emitter.Message, 0)
	}

	m.Messages[namespace] = append(m.Messages[namespace], message)
	return nil
}

// Flush marks FlushCalled as true and returns FlushError
func (m *MockEmitter) Flush() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.FlushCalled = true
	return m.FlushError
}

// Close marks CloseCalled as true and returns CloseError
func (m *MockEmitter) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.CloseCalled = true
	return m.CloseError
}

// GetMessages returns all messages for a given namespace
func (m *MockEmitter) GetMessages(namespace string) []emitter.Message {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if messages, exists := m.Messages[namespace]; exists {
		return messages
	}
	return []emitter.Message{}
}

// GetAllMessages returns all messages across all namespaces
func (m *MockEmitter) GetAllMessages() map[string][]emitter.Message {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Create a deep copy to avoid concurrent modification issues
	result := make(map[string][]emitter.Message)
	for namespace, messages := range m.Messages {
		messagesCopy := make([]emitter.Message, len(messages))
		copy(messagesCopy, messages)
		result[namespace] = messagesCopy
	}

	return result
}

// SetEmitError sets the error to be returned by Emit
func (m *MockEmitter) SetEmitError(err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.EmitError = err
}

// SetFlushError sets the error to be returned by Flush
func (m *MockEmitter) SetFlushError(err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.FlushError = err
}

// SetCloseError sets the error to be returned by Close
func (m *MockEmitter) SetCloseError(err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.CloseError = err
}

// Reset resets the mock to its initial state
func (m *MockEmitter) Reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.Messages = make(map[string][]emitter.Message)
	m.FlushCalled = false
	m.CloseCalled = false
	m.EmitError = nil
	m.FlushError = nil
	m.CloseError = nil
}

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

package bufio

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockWriteCloser implements io.WriteCloser for testing
type MockWriteCloser struct {
	mock.Mock
	buffer bytes.Buffer
}

func (m *MockWriteCloser) Write(p []byte) (int, error) {
	args := m.Called(p)
	if args.Error(1) != nil {
		return args.Int(0), args.Error(1)
	}
	n, err := m.buffer.Write(p)
	return n, err
}

func (m *MockWriteCloser) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockWriteCloser) GetContent() []byte {
	return m.buffer.Bytes()
}

func (m *MockWriteCloser) String() string {
	return m.buffer.String()
}

func TestNewLineBreakWriter(t *testing.T) {
	mockWriter := &MockWriteCloser{}

	bwc := NewLineBreakWriter(mockWriter)

	assert.NotNil(t, bwc)
	assert.NotNil(t, bwc.w)
	assert.False(t, bwc.closed)
}

func TestLineBreakWriter_Write_SingleLine(t *testing.T) {
	mock := &MockWriteCloser{}
	mock.On("Write", []byte("Hello, world!\n")).Return(14, nil)

	writer := NewLineBreakWriter(mock)

	// Write a single complete line
	n, err := writer.Write([]byte("Hello, world!\n"))
	assert.NoError(t, err)
	assert.Equal(t, 14, n)

	// due to buffering, it shouldn't be written yet
	assert.Equal(t, "", mock.String())

	// flush
	err = writer.Flush()
	assert.NoError(t, err)

	// The line should be written to the underlying writer
	assert.Equal(t, "Hello, world!\n", mock.String())

	// Buffer should be empty
	assert.Equal(t, 0, writer.buf.Len())
}

func TestLineBreakWriter_Write_MultipleLines(t *testing.T) {
	mock := &MockWriteCloser{}
	writer := NewLineBreakWriter(mock)

	// Write multiple complete lines
	n, err := writer.Write([]byte("Line 1\nLine 2\nLine 3\n"))
	assert.NoError(t, err)
	assert.Equal(t, 21, n)

	// due to buffering, it shouldn't be written yet
	assert.Equal(t, "", mock.String())

	// flush
	mock.On("Write", []byte("Line 1\nLine 2\nLine 3\n")).Return(21, nil)
	err = writer.Flush()
	assert.NoError(t, err)

	// All lines should be written to the underlying writer
	assert.Equal(t, "Line 1\nLine 2\nLine 3\n", mock.String())

	// Buffer should be empty
	assert.Equal(t, 0, writer.buf.Len())
}

func TestLineBreakWriter_Write_BufferOverflow(t *testing.T) {
	mockWriter := &MockWriteCloser{}
	mockWriter.On("Write", mock.Anything).Return(100, nil).Once()
	writer := NewLineBreakWriter(mockWriter)

	testDataBuf := bytes.NewBuffer(make([]byte, 0, initialBufCapacity))
	i := 0
	// write until the buffer fills up
	for {
		i += 1
		p := fmt.Appendf(nil, "Line %d\n", i)
		_, err := testDataBuf.Write(p)
		assert.NoError(t, err)

		n, err := writer.Write(p)
		assert.NoError(t, err)
		assert.Equal(t, len(p), n)

		if testDataBuf.Len() > initialBufCapacity {
			assert.Equal(t, testDataBuf.String(), mockWriter.String())
			break
		} else {
			// no data should be actually written
			assert.Len(t, mockWriter.GetContent(), 0)
		}
	}

	assert.Equal(t, 0, writer.buf.Len())
	assert.GreaterOrEqual(t, writer.buf.Cap(), initialBufCapacity)
}

func TestLineBreakWriter_Write_IncompleteLine(t *testing.T) {
	mock := &MockWriteCloser{}
	writer := NewLineBreakWriter(mock)

	// Write an incomplete line
	n, err := writer.Write([]byte("Incomplete"))
	assert.NoError(t, err)
	assert.Equal(t, 10, n)

	err = writer.Flush()
	assert.NoError(t, err)

	// Nothing should be written to the underlying writer yet even after flushing
	assert.Equal(t, "", mock.String())

	// Buffer should contain the incomplete line
	assert.Len(t, writer.buf.Bytes(), 10)

	// Complete the line
	n, err = writer.Write([]byte(" line\n"))
	assert.NoError(t, err)
	assert.Equal(t, 6, n)

	mock.On("Write", []byte("Incomplete line\n")).Return(16, nil)
	err = writer.Flush()
	assert.NoError(t, err)

	// The complete line should now be written
	assert.Equal(t, "Incomplete line\n", mock.String())

	// Buffer should be empty again
	assert.Equal(t, 0, writer.buf.Len())
}

func TestLineBreakWriter_Write_MixedContent(t *testing.T) {
	mock := &MockWriteCloser{}
	writer := NewLineBreakWriter(mock)

	// Write a mix of complete and incomplete lines
	n, err := writer.Write([]byte("Line 1\nLine 2\nIncomplete"))
	assert.NoError(t, err)
	assert.Equal(t, 24, n)

	// due to buffering, nothing shouldn't be written yet
	assert.Equal(t, "", mock.String())

	// flush
	mock.On("Write", []byte("Line 1\nLine 2\n")).Return(14, nil)
	err = writer.Flush()
	assert.NoError(t, err)

	// Only the complete lines should be written
	assert.Equal(t, "Line 1\nLine 2\n", mock.String())

	// Buffer should contain the incomplete line
	assert.Len(t, writer.buf.Bytes(), 10)

	// Complete the line and add another incomplete one
	n, err = writer.Write([]byte(" line\nAnother"))
	assert.NoError(t, err)
	assert.Equal(t, 13, n)

	// flush
	mock.On("Write", []byte("Incomplete line\n")).Return(16, nil)
	err = writer.Flush()
	assert.NoError(t, err)

	// The newly completed line should be written
	assert.Equal(t, "Line 1\nLine 2\nIncomplete line\n", mock.String())

	// Buffer should contain the incomplete line
	assert.Len(t, writer.buf.Bytes(), 7)
}

func TestLineBreakWriter_ForceFlush(t *testing.T) {
	mock := &MockWriteCloser{}
	writer := NewLineBreakWriter(mock)

	// Write an incomplete line
	n, err := writer.Write([]byte("Incomplete line"))
	assert.NoError(t, err)
	assert.Equal(t, 15, n)

	// Nothing should be written yet
	assert.Equal(t, "", mock.String())

	err = writer.Flush()
	assert.NoError(t, err)

	// Nothing should be written yet
	assert.Equal(t, "", mock.String())

	mock.On("Write", []byte("Incomplete line")).Return(15, nil)
	err = writer.ForceFlush()
	assert.NoError(t, err)

	// The line should be written now
	assert.Equal(t, "Incomplete line", mock.String())

	// Buffer should be empty
	assert.Equal(t, 0, writer.buf.Len())
}

func TestLineBreakWriter_Flush_EmptyBuffer(t *testing.T) {
	mock := &MockWriteCloser{}
	writer := NewLineBreakWriter(mock)

	// Flush an empty buffer should do nothing
	err := writer.ForceFlush()
	assert.NoError(t, err)

	// Nothing should be written
	assert.Equal(t, "", mock.String())
}

func TestLineBreakWriter_Close(t *testing.T) {
	mock := &MockWriteCloser{}

	writer := NewLineBreakWriter(mock)

	// Write an incomplete line
	n, err := writer.Write([]byte("Incomplete line"))
	assert.NoError(t, err)
	assert.Equal(t, 15, n)

	mock.On("Write", []byte("Incomplete line")).Return(15, nil)
	mock.On("Close").Return(nil)
	// Close should flush and close the underlying writer
	err = writer.Close()
	assert.NoError(t, err)

	// The content should be written
	assert.Equal(t, "Incomplete line", mock.String())

	// The underlying writer should be closed
	mock.AssertCalled(t, "Close")
}

func TestLineBreakWriter_WriteError(t *testing.T) {
	// Create a writer that fails on the second write
	mock := &MockWriteCloser{}
	writer := NewLineBreakWriter(mock)

	// First write should succeed
	n, err := writer.Write([]byte("Line 1\n"))
	assert.NoError(t, err)
	assert.Equal(t, 7, n)

	mock.On("Write", []byte("Line 1\n")).Return(7, nil)
	err = writer.Flush()
	assert.NoError(t, err)

	// Second write should fail
	n, err = writer.Write([]byte("Line 2\n"))
	assert.NoError(t, err)
	assert.Equal(t, 7, n)

	mock.On("Write", []byte("Line 2\n")).Return(7, errors.New("mock write error"))
	err = writer.Flush()
	assert.Error(t, err)
	assert.Equal(t, "mock write error", err.Error())

	// First line should be written
	assert.Equal(t, "Line 1\n", mock.String())
}

func TestWrite(t *testing.T) {
	testCases := []struct {
		name        string
		data        []byte
		closed      bool
		writeError  error
		expectError bool
		expectedN   int
	}{
		{
			name:        "Successful write",
			data:        []byte("test data\n"),
			closed:      false,
			writeError:  nil,
			expectError: false,
			expectedN:   10, // len("test data")
		},
		{
			name:        "Write to closed writer",
			data:        []byte("test data\n"),
			closed:      true,
			writeError:  nil,
			expectError: true,
			expectedN:   0,
		},
		{
			name:        "Write error",
			data:        []byte("test data\n"),
			closed:      false,
			writeError:  errors.New("write error"),
			expectError: true,
			expectedN:   0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockWriter := &MockWriteCloser{}

			if tc.writeError != nil {
				mockWriter.On("Write", mock.Anything).Return(0, tc.writeError)
			} else {
				mockWriter.On("Write", mock.Anything).Return(len(tc.data), nil)
			}

			bwc := NewLineBreakWriter(mockWriter)

			if tc.closed {
				bwc.closed = true
			}

			n, err := bwc.Write(tc.data)

			if tc.expectError {
				if err == nil {
					// due to buffering the write didn't fail. The flush should definitely fail
					err = bwc.ForceFlush()
					assert.Error(t, err)
					assert.Equal(t, len(tc.data), n)
				} else {
					assert.Error(t, err)
					assert.Equal(t, tc.expectedN, n)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedN, n)

				// Flush to ensure data is written to the underlying writer
				err = bwc.Flush()
				assert.NoError(t, err)

				// Verify content
				assert.Equal(t, tc.data, mockWriter.GetContent())
			}
		})
	}
}

func TestConcurrentWriteAndFlush(t *testing.T) {
	testCases := []struct {
		name            string
		numWriters      int
		numFlushers     int
		writeIterations int
		flushIterations int
	}{
		{
			name:            "Multiple writers and flushers",
			numWriters:      5,
			numFlushers:     3,
			writeIterations: 100,
			flushIterations: 50,
		},
		{
			name:            "More writers than flushers",
			numWriters:      10,
			numFlushers:     2,
			writeIterations: 50,
			flushIterations: 25,
		},
		{
			name:            "More flushers than writers",
			numWriters:      2,
			numFlushers:     8,
			writeIterations: 50,
			flushIterations: 25,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockWriter := &MockWriteCloser{}
			mockWriter.On("Write", mock.Anything).Return(5, nil)
			mockWriter.On("Close").Return(nil)

			bwc := NewLineBreakWriter(mockWriter)

			var wg sync.WaitGroup
			errChan := make(chan error, tc.numWriters+tc.numFlushers)

			// Launch writers
			for i := 0; i < tc.numWriters; i++ {
				wg.Add(1)
				go func(writerID int) {
					defer wg.Done()
					for j := 0; j < tc.writeIterations; j++ {
						data := []byte(fmt.Sprintf("data%d-%d", writerID, j))
						_, err := bwc.Write(data)
						if err != nil {
							errChan <- fmt.Errorf("writer %d failed: %v", writerID, err)
							return
						}
						// Small random sleep to increase chance of race conditions
						time.Sleep(time.Duration(rand.Intn(100)) * time.Microsecond)
					}
				}(i)
			}

			// Launch flushers
			for i := 0; i < tc.numFlushers; i++ {
				wg.Add(1)
				go func(flusherID int) {
					defer wg.Done()
					for j := 0; j < tc.flushIterations; j++ {
						err := bwc.Flush()
						if err != nil {
							errChan <- fmt.Errorf("flusher %d failed: %v", flusherID, err)
							return
						}
						// Small random sleep to increase chance of race conditions
						time.Sleep(time.Duration(rand.Intn(100)) * time.Microsecond)
					}
				}(i)
			}

			// Wait for all goroutines to complete
			wg.Wait()
			close(errChan)

			// Check for any errors
			var errors []error
			for err := range errChan {
				errors = append(errors, err)
			}
			assert.Empty(t, errors, "Expected no errors during concurrent operations, got: %v", errors)

			// Verify final state
			err := bwc.Close()
			assert.NoError(t, err, "Expected successful close after concurrent operations")
		})
	}
}

func TestConcurrentWriteFlushClose(t *testing.T) {
	mockWriter := &MockWriteCloser{}
	mockWriter.On("Write", mock.Anything).Return(5, nil)
	mockWriter.On("Close").Return(nil)

	bwc := NewLineBreakWriter(mockWriter)

	var wg sync.WaitGroup
	operations := 100
	errChan := make(chan error, operations*3) // For write, flush, and close operations

	// Launch concurrent writers
	for i := range operations {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			data := []byte(fmt.Sprintf("data-%d\n", id))
			if _, err := bwc.Write(data); err != nil {
				if !errors.Is(err, ErrWriterClosed) { // Ignore expected "writer closed" errors
					errChan <- fmt.Errorf("write error: %v", err)
				}
			}
		}(i)
	}

	// Launch concurrent flushers
	for range operations {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := bwc.Flush(); err != nil {
				if !errors.Is(err, ErrWriterClosed) { // Ignore expected "writer closed" errors
					errChan <- fmt.Errorf("flush error: %v", err)
				}
			}
		}()
	}

	// Randomly close the writer while operations are ongoing
	time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := bwc.Close(); err != nil {
			errChan <- fmt.Errorf("close error: %v", err)
		}
	}()

	// Wait for all operations to complete
	wg.Wait()
	close(errChan)

	// Check for unexpected errors
	var unexpectedErrors []error
	for err := range errChan {
		if !errors.Is(err, ErrWriterClosed) {
			unexpectedErrors = append(unexpectedErrors, err)
		}
	}
	assert.Empty(t, unexpectedErrors, "Expected no unexpected errors during concurrent operations")
}

func TestRaceFreeBufferedWriter(t *testing.T) {
	if !testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}

	mockWriter := &MockWriteCloser{}
	mockWriter.On("Write", mock.Anything).Return(5, nil)
	mockWriter.On("Close").Return(nil)

	bwc := NewLineBreakWriter(mockWriter)

	var wg sync.WaitGroup
	iterations := 1000

	// Concurrent writes
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			bwc.Write([]byte("test data\n"))
		}
	}()

	// Concurrent flushes
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			bwc.Flush()
		}
	}()

	// Concurrent reads of internal state
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = bwc.closed
		}
	}()

	// Wait for all operations to complete
	wg.Wait()

	// Final flush and close
	err := bwc.Flush()
	assert.NoError(t, err)
	err = bwc.Close()
	assert.NoError(t, err)
}

func TestLineBreakWriter_Reset(t *testing.T) {
	t.Run("Reset to new writer", func(t *testing.T) {
		// Create initial writer
		originalWriter := &MockWriteCloser{}
		originalWriter.On("Write", mock.Anything).Return(5, nil)
		originalWriter.On("Close").Return(nil)

		writer := NewLineBreakWriter(originalWriter)

		// Write some data to buffer
		_, err := writer.Write([]byte("Incomplete line"))
		assert.NoError(t, err)
		assert.Equal(t, 15, writer.buf.Len())

		// Create new writer to reset to
		newWriter := &MockWriteCloser{}
		newWriter.On("Write", mock.Anything).Return(5, nil)
		newWriter.On("Close").Return(nil)

		// Reset to new writer
		writer.Reset(newWriter)

		// Verify state was reset
		assert.Equal(t, 0, writer.buf.Len())
		assert.Equal(t, newWriter, writer.w)
		assert.False(t, writer.closed)
		assert.Nil(t, writer.err)

		// Write to the new writer
		_, err = writer.Write([]byte("New line\n"))
		assert.NoError(t, err)

		// Flush to ensure data is written
		err = writer.Flush()
		assert.NoError(t, err)

		// Verify data was written to new writer
		assert.Equal(t, []byte("New line\n"), newWriter.GetContent())

		// Original writer should have nothing
		assert.Empty(t, originalWriter.GetContent())
	})

	t.Run("Reset after error", func(t *testing.T) {
		// Create writer that will fail
		failingWriter := &MockWriteCloser{}
		failingWriter.On("Write", mock.Anything).Return(0, errors.New("write error"))
		failingWriter.On("Close").Return(nil)

		writer := NewLineBreakWriter(failingWriter)

		// Write data and force flush to trigger error
		_, err := writer.Write([]byte("Some data"))
		assert.NoError(t, err)

		err = writer.ForceFlush()
		assert.Error(t, err)
		assert.Equal(t, "write error", err.Error())

		// Create new writer
		newWriter := &MockWriteCloser{}
		newWriter.On("Write", mock.Anything).Return(5, nil)
		newWriter.On("Close").Return(nil)

		// Reset to new writer
		writer.Reset(newWriter)

		// Error should be cleared
		assert.Nil(t, writer.err)

		// Should be able to write successfully now
		_, err = writer.Write([]byte("New data\n"))
		assert.NoError(t, err)

		err = writer.Flush()
		assert.NoError(t, err)
	})

	t.Run("Reset after close", func(t *testing.T) {
		// Create initial writer
		originalWriter := &MockWriteCloser{}
		originalWriter.On("Write", mock.Anything).Return(5, nil)
		originalWriter.On("Close").Return(nil)

		writer := NewLineBreakWriter(originalWriter)

		// Close the writer
		err := writer.Close()
		assert.NoError(t, err)
		assert.True(t, writer.closed)

		// Create new writer
		newWriter := &MockWriteCloser{}
		newWriter.On("Write", mock.Anything).Return(5, nil)
		newWriter.On("Close").Return(nil)

		// Reset to new writer
		writer.Reset(newWriter)

		// Writer should no longer be closed
		assert.False(t, writer.closed)

		// Should be able to write successfully now
		_, err = writer.Write([]byte("New data\n"))
		assert.NoError(t, err)
	})

	t.Run("Reset to self", func(t *testing.T) {
		// Create writer
		mockWriter := &MockWriteCloser{}
		mockWriter.On("Write", mock.Anything).Return(5, nil)
		mockWriter.On("Close").Return(nil)

		writer := NewLineBreakWriter(mockWriter)

		// Write some data
		_, err := writer.Write([]byte("Some data"))
		assert.NoError(t, err)

		// Store the buffer length before reset
		bufLen := writer.buf.Len()

		// Reset to self (should do nothing)
		writer.Reset(writer)

		// Buffer should remain unchanged
		assert.Equal(t, bufLen, writer.buf.Len())
	})

	t.Run("Reset with nil buffer", func(t *testing.T) {
		// Create writer
		mockWriter := &MockWriteCloser{}
		mockWriter.On("Write", mock.Anything).Return(5, nil)
		mockWriter.On("Close").Return(nil)

		writer := NewLineBreakWriter(mockWriter)

		// Manually set buffer to nil
		writer.buf = nil

		// Create new writer
		newWriter := &MockWriteCloser{}
		newWriter.On("Write", mock.Anything).Return(5, nil)
		newWriter.On("Close").Return(nil)

		// Reset should initialize a new buffer
		writer.Reset(newWriter)

		// Buffer should be initialized
		assert.NotNil(t, writer.buf)

		// Should be able to write successfully
		_, err := writer.Write([]byte("New data\n"))
		assert.NoError(t, err)
	})
}

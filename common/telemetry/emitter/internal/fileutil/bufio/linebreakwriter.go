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
	"fmt"
	"io"
	"sync"
)

var ErrWriterClosed = fmt.Errorf("writer is closed")

const (
	// the capacity of the initial buffer for [LineBreakWriter]
	initialBufCapacity = 4096
)

// LineBreakWriter is a buffered [io.WriteCloser] that only writes complete lines to the underlying writer.
// ForceFlush and Close methods are exception to this rule. This is required because the telemetry consumer
// can try to read the file at any time and it treats half-written lines as malformed lines.
// After these methods are called, ALL buffered data is written regardless of newlines.
// NOTE: It holds all the data in memory until a new-line is encountered. So this should
// only be used when frequent newline characters are written to ensure frequent flushing.
type LineBreakWriter struct {
	// last write error
	err error

	// Mutex to protect concurrent access
	mu sync.Mutex
	// The underlying [io.WriteCloser]
	w io.WriteCloser

	// Flxible buffer to store incomplete lines
	buf *bytes.Buffer
	// A flush is attempted if the buffer holds equal or more bytes than this and there's a newline in the buffer
	overflowLimit int

	// closed tells if this BufferedWriteCloser was closed
	closed bool
}

// NewLineBreakWriter creates a new LineBreakWriter that writes to the provided writer.
func NewLineBreakWriter(w io.WriteCloser) *LineBreakWriter {
	return &LineBreakWriter{
		overflowLimit: initialBufCapacity,
		buf:           bytes.NewBuffer(make([]byte, 0, initialBufCapacity)),
		w:             w,
		closed:        false,
	}
}

// Write writes the contents of p to the buffer. It returns the number of bytes
// written from p and any error encountered. If n < len(p), it also returns an error
// explaining why the write is short.
//
// Write will only write complete lines to the underlying writer. Any incomplete line
// at the end of p will be buffered until a newline character is encountered in a
// subsequent call to Write.
func (w *LineBreakWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, ErrWriterClosed
	}

	if w.err != nil {
		return 0, w.err
	}

	// Add the bytes to our buffer
	n, err := w.buf.Write(p)
	if err != nil {
		w.err = err
		return n, err
	}

	if w.buf.Len() >= w.overflowLimit {
		// Get the current buffer contents
		bufBytes := w.buf.Bytes()

		// Find the last newline in the buffer
		lastNewline := bytes.LastIndexByte(bufBytes, '\n')
		if lastNewline == -1 {
			// No newline found, keep everything in the buffer
			return len(p), nil
		}

		// Write everything up to and including the last newline
		completeLines := bufBytes[:lastNewline+1]
		n, err := w.w.Write(completeLines)
		if err != nil {
			w.err = err
			// we need to return how much we wrote from p
			previousBufferN := w.buf.Len() - len(p)
			return max(0, n-previousBufferN), err
		}

		// Create a new buffer with the remaining incomplete line
		remainingBytes := bufBytes[lastNewline+1:]
		w.buf.Reset()
		n, err = w.buf.Write(remainingBytes)
		if err != nil {
			w.err = err
			// we need to return how much we wrote from p
			previousBufferN := w.buf.Len() - len(p)
			return max(0, n-previousBufferN), err
		}
	}

	return len(p), nil
}

// Reset discards any unflushed buffered data, clears any error, and
// resets b to write its output to w.
// Calling Reset on the zero value of [LineBreakWriter] initializes the internal buffer
// to the default size.
// Calling lw.Reset(w) (that is, resetting a [WriteCloser] to itself) does nothing.
func (lw *LineBreakWriter) Reset(w io.WriteCloser) {
	lw.mu.Lock()
	defer lw.mu.Unlock()

	// Avoid infinite recursion
	if lw == w {
		return
	}

	lw.w = w
	lw.closed = false

	if lw.buf == nil {
		lw.buf = bytes.NewBuffer(make([]byte, 0, initialBufCapacity))
	} else {
		lw.buf.Reset()
	}
	lw.err = nil
}

// Flush writes any buffered data to the underlying writer until the last newline character.
// it does not write anything if there's no newline character buffered until now.
func (w *LineBreakWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWriterClosed
	}

	if w.err != nil {
		return w.err
	}

	// Get the current buffer contents
	bufBytes := w.buf.Bytes()

	// Find the last newline in the buffer
	lastNewline := bytes.LastIndexByte(bufBytes, '\n')
	if lastNewline == -1 {
		// No newline found, keep everything in the buffer
		return nil
	}

	// Write everything up to and including the last newline
	completeLines := bufBytes[:lastNewline+1]
	_, err := w.w.Write(completeLines)
	if err != nil {
		return err
	}

	// Create a new buffer with the remaining incomplete line
	remainingBytes := bufBytes[lastNewline+1:]
	w.buf.Reset()
	_, err = w.buf.Write(remainingBytes)
	if err != nil {
		return err
	}

	return nil
}

// ForceFlush writes any buffered data to the underlying writer even if it doesn't end with a newline
func (w *LineBreakWriter) ForceFlush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWriterClosed
	}

	if w.err != nil {
		return w.err
	}

	if w.buf.Len() > 0 {
		// Get the buffer contents
		bufBytes := w.buf.Bytes()

		// Write the buffer to the underlying writer
		_, err := w.w.Write(bufBytes)
		if err != nil {
			return err
		}

		// Reset the buffer
		w.buf.Reset()
	}

	return nil
}

// Close does a force flush and then closes the underlying writer.
// The underlying writer is not closed if the force flush fails.
func (w *LineBreakWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	if w.err != nil {
		return w.err
	}

	// Flush any buffered data
	if w.buf.Len() > 0 {
		// Get the buffer contents
		bufBytes := w.buf.Bytes()

		// Write the buffer to the underlying writer
		_, err := w.w.Write(bufBytes)
		if err != nil {
			return err
		}

		// Reset the buffer
		w.buf.Reset()
	}

	// Then close the underlying writer
	if err := w.w.Close(); err != nil {
		return fmt.Errorf("underlying writer close error: %v", err)
	}

	w.closed = true
	return nil
}

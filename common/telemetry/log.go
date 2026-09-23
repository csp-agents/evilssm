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

package telemetry

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/aws/amazon-ssm-agent/common/telemetry/telemetrylog"
)

type telemetryLogger struct {
	namespace string
}

// GetLogger provides a handle on the [github.com/aws/amazon-ssm-agent/common/telemetry/telemetrylog.Log]. It can be used to emit logs.
func GetLogger(namespace string) telemetrylog.Log {
	return &telemetryLogger{namespace: namespace}
}

// EmitLog emits log without formatting
func (l *telemetryLogger) EmitLog(s telemetrylog.Severity, v ...interface{}) (err error) {
	var t *telemetry
	t, err = getTelemetry()
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			t.context.Log().Errorf("EmitLog panic: %v", r)
			t.context.Log().Errorf("Stacktrace:\n%s", debug.Stack())
			err = fmt.Errorf("panic in EmitLog %v", r)
		}
	}()

	return t.emitLog(l.namespace, time.Now(), s, fmt.Sprint(v...))
}

// EmitLogf emits log with formatting
func (l *telemetryLogger) EmitLogf(s telemetrylog.Severity, format string, params ...interface{}) (err error) {
	var t *telemetry
	t, err = getTelemetry()
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			t.context.Log().Errorf("EmitLogf panic: %v", r)
			t.context.Log().Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()

	return t.emitLog(l.namespace, time.Now(), s, fmt.Sprintf(format, params...))
}

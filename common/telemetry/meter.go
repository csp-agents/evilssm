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

	"github.com/aws/amazon-ssm-agent/common/telemetry/metric"
)

// meterT is a [metric.Meter] for a specific namespace
type meterT struct {
	namespace string
}

// GetMeter gets a [metric.Meter] object for the given namespace
func GetMeter(namespace string) metric.Meter {
	// TODO: Cache it
	return meterT{namespace: namespace}
}

func (m meterT) Int64Counter(name string, unit metric.Unit) metric.Int64Counter {
	return int64Counter{namespace: m.namespace, name: name, unit: unit}
}

type int64Counter struct {
	namespace string
	name      string
	unit      metric.Unit
}

// Add increments the counter by the specified amount
func (i int64Counter) Add(incr int64) (err error) {
	t, err := getTelemetry()
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			t.context.Log().Errorf("Counter Add panic: %v", r)
			t.context.Log().Errorf("Stacktrace:\n%s", debug.Stack())
			err = fmt.Errorf("panic in Add %v", r)
		}
	}()

	return t.emitIntegerMetric(i.namespace, i.name, i.unit, metric.Sum, time.Now(), incr)
}

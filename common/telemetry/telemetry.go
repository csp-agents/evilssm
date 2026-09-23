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
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/common/telemetry/context"
	"github.com/aws/amazon-ssm-agent/common/telemetry/emitter"
	"github.com/aws/amazon-ssm-agent/common/telemetry/metric"
	"github.com/aws/amazon-ssm-agent/common/telemetry/telemetrylog"
)

var (
	singleton *telemetry
)

const (
	SSMAgentNamespace      = "SSMAgent"
	CoreAgentChannelName   = "core"
	AgentWorkerChannelName = "agent_worker"
)

var singletonMtx = new(sync.RWMutex)

type telemetry struct {
	context context.TelemetryContext

	// fileChannalMtx protects the fileChannel variable
	emitterMtx *sync.RWMutex
	emitter    emitter.Emitter
}

func getTelemetry() (*telemetry, error) {
	singletonMtx.RLock()
	defer singletonMtx.RUnlock()

	if singleton == nil {
		return nil, errors.New("telemetry is not initialized")
	}

	return singleton, nil
}

func Initialize(context context.TelemetryContext) (err error) {
	defer func() {
		if r := recover(); r != nil {
			context.Log().Warnf("telemetry Initialize panic: %v", r)
			context.Log().Warnf("Stacktrace:\n%s", debug.Stack())
			err = fmt.Errorf("panic in telemetry.Initialize %v", r)
		}
	}()

	singletonMtx.Lock()
	defer singletonMtx.Unlock()

	if singleton != nil {
		return errors.New("telemetry is already initialized")
	}

	log := context.Log()

	emitter := emitter.NewEmitter(log)
	singleton = &telemetry{context: context, emitterMtx: &sync.RWMutex{}, emitter: emitter}

	log.Info("Telemetry initialized")
	return nil
}

func Shutdown() {
	defer func() {
		if r := recover(); r != nil && singleton != nil {
			singleton.context.Log().Warnf("telemetry Shutdown panic: %v", r)
			singleton.context.Log().Warnf("Stacktrace:\n%s", debug.Stack())
		}
	}()

	singletonMtx.Lock()
	defer singletonMtx.Unlock()

	if singleton != nil {
		singleton.shutdown()
	}
	singleton = nil
}

func (t *telemetry) shutdown() (err error) {
	t.emitterMtx.Lock()
	defer t.emitterMtx.Unlock()

	if t.emitter != nil {
		err = t.emitter.Close()
		t.emitter = nil
	}

	return err
}

// emitLog is the internal function which emits logs using [emitter.Emitter].
func (t *telemetry) emitLog(namespace string, time time.Time, severity telemetrylog.Severity, message string) (err error) {
	defer func() {
		if r := recover(); r != nil && singleton != nil {
			singleton.context.Log().Warnf("telemetry emitLog panic: %v", r)
			singleton.context.Log().Warnf("Stacktrace:\n%s", debug.Stack())
			err = fmt.Errorf("panic in telemetry.emitLog %v", r)
		}
	}()

	message = TruncateLog(message)
	entry := &telemetrylog.Entry{Time: time.UTC(), Severity: severity, Body: message}

	entryJson, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	emitterMessage := emitter.Message{
		Type:    emitter.LOG,
		Payload: string(entryJson),
	}

	t.emitterMtx.Lock()
	defer t.emitterMtx.Unlock()

	if t.emitter == nil {
		return errors.New("telemetry is not initialized")
	}
	go t.emitTelemetryMessage(namespace, emitterMessage)
	return nil
}

// emitIntegerMetric is the internal function which emits a metric using [emitter.Emitter].
func (t *telemetry) emitIntegerMetric(namespace, name string, unit metric.Unit, kind metric.Kind, time time.Time, value int64) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if singleton != nil {
				singleton.context.Log().Warnf("telemetry emitIntegerMetric panic: %v", r)
				singleton.context.Log().Warnf("Stacktrace:\n%s", debug.Stack())
			}
			err = fmt.Errorf("panic in telemetry.emitIntegerMetric %v", r)
		}
	}()

	entry := &metric.Metric[int64]{
		Name:       name,
		Unit:       unit,
		Kind:       kind,
		DataPoints: []metric.DataPoint[int64]{{StartTime: time.UTC(), EndTime: time.UTC(), Value: value}},
	}

	entryJson, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	message := emitter.Message{
		Type:    emitter.METRIC,
		Payload: string(entryJson),
	}

	t.emitterMtx.Lock()
	defer t.emitterMtx.Unlock()

	if t.emitter == nil {
		return errors.New("telemetry is not initialized")
	}
	go t.emitTelemetryMessage(namespace, message)
	return nil
}

// emitTelemetryMessage emits the telemetry message using [emitter.Emitter]
func (t *telemetry) emitTelemetryMessage(namespace string, message emitter.Message) {
	defer func() {
		if r := recover(); r != nil {
			t.context.Log().Warnf("Telemetry emitTelemetryMessage panic: %v", r)
			t.context.Log().Warnf("Stacktrace:\n%s", debug.Stack())
		}
	}()

	t.emitterMtx.Lock()
	defer t.emitterMtx.Unlock()

	if t.emitter == nil {
		return
	}

	err := t.emitter.Emit(namespace, message)
	if err != nil {
		t.context.Log().Warnf("Emitting telemetry message failed with: %v", err)
	}
}

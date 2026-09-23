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
package control_channel_exporter

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"runtime/debug"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/session/communicator"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/telemetry/collector"
	"github.com/aws/amazon-ssm-agent/agent/telemetry/datastores"
	dynamicconfiguration "github.com/aws/amazon-ssm-agent/agent/telemetry/dynamic_configuration"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/amazon-ssm-agent/common/telemetry/metric"
	"github.com/aws/amazon-ssm-agent/common/telemetry/telemetrylog"

	"github.com/gorilla/websocket"
)

const (
	// UnknownInstanceId is the instance ID sent to the backend if the instance ID could not be determined
	UnknownInstanceId = "unknown"

	// AgentTelemetryV2MessageType represents message type for V2 agent telemetry
	AgentTelemetryV2MessageType = "agent_telemetry_v2"
)

// used to lock the send message process.
var telemetryExporterSingletonMtx = &sync.RWMutex{}
var telemetryExporterSingleton *controlChannelTelemetryExporter

type ITelemetryExporter interface {
	StartExporter()
	StopExporter()
	ShouldExport(namespace string) bool
}

// controlChannelTelemetryExporter helps us in scheduling the process to send telemetry to MGS
type controlChannelTelemetryExporter struct {
	channel communicator.IWebSocketChannel
	ctx     context.T
}

// GetControlChannelTelemetryExporter returns us the singleton instance of AuditLogTelemetry
func GetControlChannelTelemetryExporter(ctx context.T, channel communicator.IWebSocketChannel) *controlChannelTelemetryExporter {
	telemetryExporterSingletonMtx.Lock()
	defer telemetryExporterSingletonMtx.Unlock()

	if telemetryExporterSingleton != nil {
		return telemetryExporterSingleton
	}

	telemetryExporterSingleton = &controlChannelTelemetryExporter{
		channel: channel,
		ctx:     ctx,
	}
	return telemetryExporterSingleton
}

func (t *controlChannelTelemetryExporter) StartExporter() {
	defer func() {
		if r := recover(); r != nil {
			t.ctx.Log().Warnf("controlChannelTelemetryExporter StartExporter panic: %v", r)
			t.ctx.Log().Warnf("Stacktrace:\n%s", debug.Stack())
		}
	}()

	err := collector.AddExporter(t.ctx.Log(), t)

	if err != nil {
		t.ctx.Log().Errorf("Error while adding telemetry exporter: %v", err)
	}
}

func (t *controlChannelTelemetryExporter) StopExporter() {
	defer func() {
		if r := recover(); r != nil {
			t.ctx.Log().Warnf("controlChannelTelemetryExporter StopExporter panic: %v", r)
			t.ctx.Log().Warnf("Stacktrace:\n%s", debug.Stack())
		}
	}()

	err := collector.RemoveExporter(t.ctx.Log(), t)

	if err != nil {
		t.ctx.Log().Errorf("Error while removing telemetry exporter: %v", err)
	}
}

// For mocking support
var controlChannelCheckTelemetryExportLuck = checkTelemetryExportLuck
var controlChannelIsTelemetryEnabled = isTelemetryEnabled
var controlChannelTooSoontoExportTelemetry = tooSoontoExportTelemetry

var randomPercentage = getRandomPercentage

func getRandomPercentage() float64 {
	return rand.Float64() * 100
}

// checkTelemetryExportLuck returns us the result of the "coin toss", whether we got lucky enough to export telemetry or not
func checkTelemetryExportLuck(log log.T, namespace string) bool {
	telemetryEmissionLuck := randomPercentage()
	configuredPercentageLimit := dynamicconfiguration.PercentageLimit(namespace)
	checkResult := telemetryEmissionLuck < configuredPercentageLimit
	log.Debugf("TelemetryExportLuck checkResult: %d", checkResult)
	return checkResult
}

// isTelemetryEnabled returns if telemetry is currently enabled for given namespace
func isTelemetryEnabled(log log.T, namespace string) bool {
	telemetryDisabledTill := dynamicconfiguration.TelemetryDisabledTill(namespace)
	checkResult := telemetryDisabledTill < time.Now().Unix()
	log.Debugf("TelemetryEnabled checkResult: %d", checkResult)
	return checkResult
}

// tooSoontoExportTelemetry returns if exportPeriod amount of time has passed since we last exported telemetry for given namespace
func tooSoontoExportTelemetry(log log.T, namespace string) bool {
	lastEmittedDataStore := datastores.TelemetryLastEmittedDataStore()
	lastEmittedTimeStamp := lastEmittedDataStore.Read(namespace)
	exportPeriod := dynamicconfiguration.ExportPeriod(namespace)
	// Giving 5 seconds of grace period
	checkResult := (time.Now().Unix() - lastEmittedTimeStamp) < int64(exportPeriod*60)-5
	log.Debugf("TooSoonToExportTelemetry checkResult: %d", checkResult)
	return checkResult
}

// updateLastEmittedTimestamp updates the in-memory key-value data store to store last timestamp when export is done
func updateLastEmittedTimestamp(namespace string, lastEmittedTimeStamp int64) {
	lastEmittedDataStore := datastores.TelemetryLastEmittedDataStore()
	lastEmittedDataStore.Write(namespace, lastEmittedTimeStamp)
}

// ShouldExport checks if agent should export telemetry to MGS
// It depends on the result of the coin toss, whether telemetry is enabled for a namespace and if exportPeriod time has elapsed since last export
func (t *controlChannelTelemetryExporter) ShouldExport(namespace string) bool {
	return controlChannelCheckTelemetryExportLuck(t.ctx.Log(), namespace) && controlChannelIsTelemetryEnabled(t.ctx.Log(), namespace) && !controlChannelTooSoontoExportTelemetry(t.ctx.Log(), namespace)
}

// Export exports telemetry for a given namespace to MGS
func (t *controlChannelTelemetryExporter) Export(namespace string,
	metrics []metric.Metric[float64], logs []telemetrylog.Entry) (err error) {
	defer func() {
		if r := recover(); r != nil {
			t.ctx.Log().Warnf("controlChannelTelemetryExporter Export panic: %v", r)
			t.ctx.Log().Warnf("Stacktrace:\n%s", debug.Stack())
			err = fmt.Errorf("panic in controlChannelTelemetryExporter Export %v", r)
		}
	}()

	logger := t.ctx.Log()

	if !t.ShouldExport(namespace) {
		logger.Debugf("Skipping telemetry export for namespace %s", namespace)
		return nil
	}

	if len(metrics) == 0 && len(logs) == 0 {
		return nil
	}

	var payloadBytes []byte
	payloadBytes, err = createTelemetryPayload(metrics, logs)
	if err != nil {
		logger.Debugf("Error while preparing payload for telemetry: %v", err)
		return err
	}

	instanceId, err := t.ctx.Identity().InstanceID()
	if err != nil {
		instanceId = UnknownInstanceId
	}

	agentTelemetryV2 := &AgentTelemetryV2{
		SchemaVersion: 1,
		Namespace:     namespace,
		AgentVersion:  version.Version,
		InstanceId:    instanceId,
		AwsAccountId:  "",
		Payload:       string(payloadBytes),
	}

	agentTelemetryBytes, err := json.Marshal(agentTelemetryV2)
	if err != nil { // return error only when telemetry to MGS is enabled
		return fmt.Errorf("unable to marshal AgentTelemetryV2 payload to json string: %s, err: %s", agentTelemetryBytes, err)
	}

	err = t.sendChannelContract(agentTelemetryBytes, AgentTelemetryV2MessageType)
	if err != nil {
		err = fmt.Errorf("unable to send message to MGS: %s", err)
	}
	updateLastEmittedTimestamp(namespace, time.Now().Unix())

	return err
}

// sendChannelContract sends the payload through the web socket connection with necessary packaging
func (t *controlChannelTelemetryExporter) sendChannelContract(payload []byte, messageType string) error {
	log := t.ctx.Log()
	agentMessage := &mgsContracts.AgentMessage{
		MessageType:    messageType,
		MessageId:      uuid.New(),
		SchemaVersion:  1,
		CreatedDate:    uint64(time.Now().UnixNano() / 1000000), //nolint:gosec
		SequenceNumber: 0,
		Flags:          0,
		Payload:        payload,
	}
	log.Debugf("Sending payload to MGS: %v", jsonutil.Indent(string(payload)))
	agentBytes, err := agentMessage.Serialize(log)
	if err != nil {
		return err
	}
	return t.channel.SendMessage(log, agentBytes, websocket.BinaryMessage)
}

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
	"time"

	"github.com/aws/amazon-ssm-agent/common/telemetry/metric"
	"github.com/aws/amazon-ssm-agent/common/telemetry/telemetrylog"
)

// AgentTelemetryV2
type AgentTelemetryV2 struct {
	SchemaVersion uint32 `json:"SchemaVersion"`
	Namespace     string `json:"Namespace"`
	AgentVersion  string `json:"AgentVersion"`
	InstanceId    string `json:"InstanceId"`
	AwsAccountId  string `json:"AwsAccountId"`
	ConfigHash    string `json:"ConfigHash"`
	Payload       string `json:"Payload"` // AgentTelemetryV2Payload
}

// AgentTelemetryV2Payload is the Payload in the AgentTelemetryV2 object
type AgentTelemetryV2Payload struct {
	Metrics []Metric   `json:"Metrics"`
	Logs    []LogEntry `json:"Logs"`
}

// Metric is the metric object accepted by telemetry service
type Metric struct {
	// Name is the name of the Instrument that created this data.
	Name       string      `json:"Name"`
	Unit       string      `json:"Unit"`
	DataPoints []DataPoint `json:"DataPoints"`
}

// DataPoint is a single data point in a timeseries accepted by the telemetry service
type DataPoint struct {
	// Time is the time when the timeseries was started
	Time time.Time `json:"Time"`
	// Value is the value of this data point.
	Value float64 `json:"Value"`
}

// LogEntry is the a log record accepted by the telemetry service
type LogEntry struct {
	Time     time.Time             `json:"Time"`
	Severity telemetrylog.Severity `json:"Severity"`
	Body     string                `json:"Body"`
}

func createTelemetryPayload(metrics []metric.Metric[float64], logs []telemetrylog.Entry) ([]byte, error) {
	payload := &AgentTelemetryV2Payload{
		Metrics: mapMetrics(metrics),
		Logs:    mapLogs(logs),
	}

	return json.Marshal(payload)
}

// mapMetrics maps the metric model from internal represetation to the telemetry service model
func mapMetrics(metrics []metric.Metric[float64]) []Metric {
	result := make([]Metric, len(metrics))

	for i, m := range metrics {
		result[i] = Metric{
			Name:       m.Name,
			Unit:       m.Unit.String(),
			DataPoints: mapDataPoints(m.DataPoints),
		}
	}

	return result
}

// mapDataPoints maps the datapoint model from internal represetation to the telemetry service model
func mapDataPoints(dataPoints []metric.DataPoint[float64]) []DataPoint {
	result := make([]DataPoint, len(dataPoints))

	for i, dp := range dataPoints {
		result[i] = DataPoint{
			Time:  dp.StartTime.UTC(), // since we aggregate metrics at second level, this will only be accurate to a second
			Value: dp.Value,
		}
	}

	return result
}

// mapLogs maps the log-entry model from internal represetation to the telemetry service model
func mapLogs(logs []telemetrylog.Entry) []LogEntry {
	result := make([]LogEntry, len(logs))

	for i, l := range logs {
		result[i] = LogEntry{
			Time:     l.Time.UTC(),
			Severity: l.Severity,
			Body:     l.Body,
		}
	}

	return result
}

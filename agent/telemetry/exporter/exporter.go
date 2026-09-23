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

package exporter

import (
	"github.com/aws/amazon-ssm-agent/common/telemetry/metric"
	"github.com/aws/amazon-ssm-agent/common/telemetry/telemetrylog"
)

type NamespacedTelemetry map[string]*struct {
	Metrics []metric.Metric[float64]
	Logs    []telemetrylog.Entry
}

type Exporter interface {
	Export(namespace string, metrics []metric.Metric[float64], logs []telemetrylog.Entry) error
}

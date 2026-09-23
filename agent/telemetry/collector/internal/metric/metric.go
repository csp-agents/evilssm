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
package metric

import "github.com/aws/amazon-ssm-agent/common/telemetry/metric"

type MetricsCollector interface {
	// CollectMetric is used to ingest a metric into the collector
	CollectMetric(namespace string, metric metric.Metric[float64]) error

	// FetchAndDrop fetches maximum of [limit] number of metrics ingested until now in all the namespaces
	FetchAndDrop(limit int) (metric.NamespaceMetrics[float64], error)

	Close() error
}

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
package inmemory

import (
	"errors"

	"github.com/aws/amazon-ssm-agent/common/telemetry/metric"
)

// aggregateDataPoint holds an aggregated data point
type aggregateDataPoint struct {
	metric.DataPoint[float64]

	// count of data points aggregated until now. Needed for calculating average for example
	dataPointCounts int
}

// kindAggregator is an interface which specifies how two metric points should be aggregated
type kindAggregator[N int64 | float64] interface {
	aggregate(previous aggregateDataPoint, newValue N) aggregateDataPoint
}

func newMetricKindAggregator[N int64 | float64](kind metric.Kind) (kindAggregator[N], error) {
	switch kind {
	case metric.Sum:
		m := newSumMetric[N]()

		return m, nil

	case metric.Gauge:
		m := newGaugeMetric[N]()

		return m, nil
	default:
		return nil, errors.New("unsupported metric kind")
	}
}

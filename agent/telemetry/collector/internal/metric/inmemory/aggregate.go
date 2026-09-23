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
	"github.com/aws/amazon-ssm-agent/common/telemetry/metric"
)

type sumMetric[N int64 | float64] struct {
}

// newSumMetric creates and returns a new instance of [sumMetric]
func newSumMetric[N int64 | float64]() *sumMetric[N] {
	return &sumMetric[N]{}
}

func (m *sumMetric[N]) aggregate(previous aggregateDataPoint, newValue N) aggregateDataPoint {
	return aggregateDataPoint{
		DataPoint: metric.DataPoint[float64]{
			StartTime: previous.StartTime,
			EndTime:   previous.EndTime,
			Value:     previous.Value + float64(newValue),
		},
		dataPointCounts: previous.dataPointCounts + 1,
	}
}

type gaugeMetric[N int64 | float64] struct {
}

// newGaugeMetric creates and returns a new instance of [gaugeMetric]
func newGaugeMetric[N int64 | float64]() *gaugeMetric[N] {
	return &gaugeMetric[N]{}
}

func (m *gaugeMetric[N]) aggregate(previous aggregateDataPoint, newValue N) aggregateDataPoint {
	return aggregateDataPoint{
		DataPoint: metric.DataPoint[float64]{
			StartTime: previous.StartTime,
			EndTime:   previous.EndTime,
			Value:     (previous.Value*float64(previous.dataPointCounts) + float64(newValue)) / float64(previous.dataPointCounts+1),
		},
		dataPointCounts: previous.dataPointCounts + 1,
	}
}

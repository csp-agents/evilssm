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
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/aws/amazon-ssm-agent/common/telemetry/metric"

	"github.com/stretchr/testify/assert"
)

func TestNewTimeAggregatedMetric(t *testing.T) {
	tests := []struct {
		name       string
		metricName string
		unit       metric.Unit
		kind       metric.Kind
		wantErr    bool
	}{
		{
			name:       "valid sum metric",
			metricName: "test_metric",
			unit:       metric.UnitCount,
			kind:       metric.Sum,
			wantErr:    false,
		},
		{
			name:       "unsupported metric kind",
			metricName: "test_metric",
			unit:       metric.UnitCount,
			kind:       metric.Kind("unsupported"),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric, err := newTimeAggregatedMetric[int64](tt.metricName, tt.unit, tt.kind)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, metric)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, metric)
				assert.Equal(t, tt.metricName, metric.name)
				assert.Equal(t, tt.unit, metric.unit)
				assert.Equal(t, tt.kind, metric.kind)
			}
		})
	}
}

func TestTimeAggregatedMetricAggregateStartTimeEqualsEndTime(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	m, err := newTimeAggregatedMetric[int64]("test_metric", "count", metric.Sum)
	assert.NoError(t, err)

	tests := []struct {
		name    string
		point   metric.DataPoint[int64]
		wantErr bool
	}{
		{
			name: "valid single point",
			point: metric.DataPoint[int64]{
				StartTime: now,
				EndTime:   now,
				Value:     10,
			},
			wantErr: false,
		},
		{
			name: "invalid data point",
			point: metric.DataPoint[int64]{
				StartTime: now,
				EndTime:   now.Add(time.Second),
				Value:     10,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.aggregate(tt.point)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Verify the point was aggregated
				found := false
				for span, point := range m.spans {
					if span.contains(tt.point.StartTime) {
						found = true
						assert.Equal(t, float64(tt.point.Value), point.Value)
					}
				}
				assert.True(t, found)
			}
		})
	}
}

func TestInt64TimeAggregatedSumMetric(t *testing.T) {
	tam, err := newTimeAggregatedMetric[int64]("test_metric", metric.UnitCount, metric.Sum)
	assert.NoError(t, err)
	assert.Equal(t, "test_metric", tam.name)
	assert.Equal(t, metric.UnitCount, tam.unit)
	assert.Equal(t, metric.Sum, tam.kind)

	now := time.Now().Truncate(time.Second)

	metrics := []metric.DataPoint[int64]{
		{
			StartTime: now,
			EndTime:   now,
			Value:     10,
		},
		{
			StartTime: now.Add(time.Second),
			EndTime:   now.Add(time.Second),
			Value:     10,
		},
		{
			StartTime: now.Add(time.Second).Add(500 * time.Millisecond),
			EndTime:   now.Add(time.Second).Add(500 * time.Millisecond),
			Value:     10,
		},
		{
			StartTime: now.Add(500 * time.Minute),
			EndTime:   now.Add(500 * time.Minute),
			Value:     10,
		},
		{
			StartTime: now.Add(500 * time.Minute).Add(999 * time.Millisecond),
			EndTime:   now.Add(500 * time.Minute).Add(999 * time.Millisecond),
			Value:     5,
		},
	}

	for _, datapoint := range metrics {
		err := tam.aggregate(datapoint)
		assert.NoError(t, err)
	}

	assert.Len(t, tam.spans, 3)

	first := tam.spans[timeSpan{
		startTime: now,
		endTime:   now.Add(time.Second),
	}]
	assert.Equal(t, float64(10), first.Value)
	assert.Equal(t, 1, first.dataPointCounts)

	second := tam.spans[timeSpan{
		startTime: now.Add(time.Second),
		endTime:   now.Add(2 * time.Second),
	}]
	assert.Equal(t, float64(20), second.Value)
	assert.Equal(t, 2, second.dataPointCounts)

	third := tam.spans[timeSpan{
		startTime: now.Add(500 * time.Minute),
		endTime:   now.Add(500 * time.Minute).Add(time.Second),
	}]
	assert.Equal(t, float64(15), third.Value)
	assert.Equal(t, 2, third.dataPointCounts)
}

func TestFloat64TimeAggregatedSumMetric(t *testing.T) {
	tam, err := newTimeAggregatedMetric[float64]("test_metric", metric.UnitCount, metric.Sum)
	assert.NoError(t, err)
	assert.Equal(t, "test_metric", tam.name)
	assert.Equal(t, metric.UnitCount, tam.unit)
	assert.Equal(t, metric.Sum, tam.kind)

	now := time.Now().Truncate(time.Second)

	metrics := []metric.DataPoint[float64]{
		{
			StartTime: now,
			EndTime:   now,
			Value:     10,
		},
		{
			StartTime: now.Add(time.Second),
			EndTime:   now.Add(time.Second),
			Value:     10,
		},
		{
			StartTime: now.Add(time.Second).Add(500 * time.Millisecond),
			EndTime:   now.Add(time.Second).Add(500 * time.Millisecond),
			Value:     10,
		},
		{
			StartTime: now.Add(500 * time.Minute),
			EndTime:   now.Add(500 * time.Minute),
			Value:     10,
		},
		{
			StartTime: now.Add(500 * time.Minute).Add(999 * time.Millisecond),
			EndTime:   now.Add(500 * time.Minute).Add(999 * time.Millisecond),
			Value:     5,
		},
	}

	for _, datapoint := range metrics {
		err := tam.aggregate(datapoint)
		assert.NoError(t, err)
	}

	assert.Len(t, tam.spans, 3)

	first := tam.spans[timeSpan{
		startTime: now,
		endTime:   now.Add(time.Second),
	}]
	assert.Equal(t, float64(10), first.Value)
	assert.Equal(t, 1, first.dataPointCounts)

	second := tam.spans[timeSpan{
		startTime: now.Add(time.Second),
		endTime:   now.Add(2 * time.Second),
	}]
	assert.Equal(t, float64(20), second.Value)
	assert.Equal(t, 2, second.dataPointCounts)

	third := tam.spans[timeSpan{
		startTime: now.Add(500 * time.Minute),
		endTime:   now.Add(500 * time.Minute).Add(time.Second),
	}]
	assert.Equal(t, float64(15), third.Value)
	assert.Equal(t, 2, third.dataPointCounts)
}

func TestInt64TimeAggregatedGaugeMetric(t *testing.T) {
	tam, err := newTimeAggregatedMetric[int64]("test_metric", metric.UnitCount, metric.Gauge)
	assert.NoError(t, err)
	assert.Equal(t, "test_metric", tam.name)
	assert.Equal(t, metric.UnitCount, tam.unit)
	assert.Equal(t, metric.Gauge, tam.kind)

	now := time.Now().Truncate(time.Second)

	metrics := []metric.DataPoint[int64]{
		{
			StartTime: now,
			EndTime:   now,
			Value:     10,
		},
		{
			StartTime: now.Add(time.Second),
			EndTime:   now.Add(time.Second),
			Value:     10,
		},
		{
			StartTime: now.Add(time.Second).Add(500 * time.Millisecond),
			EndTime:   now.Add(time.Second).Add(500 * time.Millisecond),
			Value:     10,
		},
		{
			StartTime: now.Add(500 * time.Minute),
			EndTime:   now.Add(500 * time.Minute),
			Value:     10,
		},
		{
			StartTime: now.Add(500 * time.Minute).Add(999 * time.Millisecond),
			EndTime:   now.Add(500 * time.Minute).Add(999 * time.Millisecond),
			Value:     5,
		},
	}

	for _, datapoint := range metrics {
		err := tam.aggregate(datapoint)
		assert.NoError(t, err)
	}

	assert.Len(t, tam.spans, 3)

	first := tam.spans[timeSpan{
		startTime: now,
		endTime:   now.Add(time.Second),
	}]
	assert.Equal(t, float64(10), first.Value)
	assert.Equal(t, 1, first.dataPointCounts)

	second := tam.spans[timeSpan{
		startTime: now.Add(time.Second),
		endTime:   now.Add(2 * time.Second),
	}]
	assert.Equal(t, float64(10), second.Value)
	assert.Equal(t, 2, second.dataPointCounts)

	third := tam.spans[timeSpan{
		startTime: now.Add(500 * time.Minute),
		endTime:   now.Add(500 * time.Minute).Add(time.Second),
	}]
	assert.Equal(t, float64(7.5), third.Value)
	assert.Equal(t, 2, third.dataPointCounts)
}

func TestNamespacedAggregatedMetric_CollectMetric(t *testing.T) {
	timeStamp := time.Now().UTC()

	tests := []struct {
		name      string
		namespace string
		metric    metric.Metric[float64]
		wantErr   bool
	}{
		{
			name:      "Basic",
			namespace: "test-namespace",
			metric: metric.Metric[float64]{
				Name: "test-metric",
				Unit: metric.UnitCount,
				Kind: metric.Sum,
				DataPoints: []metric.DataPoint[float64]{
					{
						StartTime: timeStamp,
						EndTime:   timeStamp,
						Value:     1.0},
				},
			},
			wantErr: false,
		},
		{
			name:      "Multiple data points",
			namespace: "test-namespace",
			metric: metric.Metric[float64]{
				Name: "test-metric",
				Unit: metric.UnitCount,
				Kind: metric.Sum,
				DataPoints: []metric.DataPoint[float64]{
					{
						StartTime: timeStamp,
						EndTime:   timeStamp,
						Value:     2.0,
					},
					{
						StartTime: timeStamp,
						EndTime:   timeStamp,
						Value:     3.0,
					},
				},
			},
			wantErr: false,
		},
		{
			name:      "Empty datapoints",
			namespace: "test-namespace",
			metric: metric.Metric[float64]{
				Name:       "test-metric",
				Unit:       metric.UnitCount,
				Kind:       metric.Sum,
				DataPoints: []metric.DataPoint[float64]{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize the collector
			c := NewInMemoryMetricCollector(context.NewMockDefault())

			// Execute the method
			err := c.CollectMetric(tt.namespace, tt.metric)

			// Assert results
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify metric was stored correctly
				assert.NotNil(t, c.metrics[tt.namespace])
				assert.NotNil(t, c.metrics[tt.namespace][tt.metric.Name])

				// Verify metric properties
				storedMetric := c.metrics[tt.namespace][tt.metric.Name]
				assert.Equal(t, tt.metric.Name, storedMetric.name)
				assert.Equal(t, tt.metric.Unit, storedMetric.unit)
				assert.Equal(t, tt.metric.Kind, storedMetric.kind)

				if len(tt.metric.DataPoints) == 0 {
					assert.Len(t, storedMetric.spans, 0)
				} else {
					expectedValue := 0.0
					for _, point := range tt.metric.DataPoints {
						expectedValue += point.Value
					}

					spanStartTime := tt.metric.DataPoints[0].StartTime.Truncate(time.Second)

					assert.Equal(t, metric.DataPoint[float64]{
						StartTime: spanStartTime,
						EndTime:   spanStartTime.Add(time.Second),
						Value:     expectedValue,
					}, storedMetric.spans[timeSpan{
						startTime: spanStartTime,
						endTime:   spanStartTime.Add(time.Second),
					}].DataPoint)
				}
			}
		})
	}
}

// Test concurrent access
func TestNamespacedAggregatedMetric_CollectMetric_Concurrent(t *testing.T) {
	c := NewInMemoryMetricCollector(context.NewMockDefault())

	timeStamp := time.Now().UTC()

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			metric := metric.Metric[float64]{
				Name: fmt.Sprintf("test-metric-%d", id),
				Unit: metric.UnitCount,
				Kind: metric.Sum,
				DataPoints: []metric.DataPoint[float64]{
					{
						Value:     float64(id),
						StartTime: timeStamp.Add(time.Duration(i) * time.Second),
						EndTime:   timeStamp.Add(time.Duration(i) * time.Second),
					},
				},
			}
			err := c.CollectMetric("test-namespace", metric)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Verify all metrics were stored
	assert.Len(t, c.metrics["test-namespace"], goroutines)
}

func TestNamespacedAggregatedMetric_FetchAllAndDrop(t *testing.T) {
	timeStamp := time.Now().UTC()

	tests := []struct {
		name      string
		namespace string
		metric    metric.Metric[float64]
		wantErr   bool
	}{
		{
			name:      "Basic",
			namespace: "test-namespace",
			metric: metric.Metric[float64]{
				Name: "test-metric",
				Unit: metric.UnitCount,
				Kind: metric.Sum,
				DataPoints: []metric.DataPoint[float64]{
					{
						StartTime: timeStamp,
						EndTime:   timeStamp,
						Value:     1.0},
				},
			},
			wantErr: false,
		},
		{
			name:      "Multiple datapoints",
			namespace: "test-namespace",
			metric: metric.Metric[float64]{
				Name: "test-metric",
				Unit: metric.UnitCount,
				Kind: metric.Sum,
				DataPoints: []metric.DataPoint[float64]{
					{
						StartTime: timeStamp,
						EndTime:   timeStamp,
						Value:     2.0,
					},
					{
						StartTime: timeStamp,
						EndTime:   timeStamp,
						Value:     3.0,
					},
				},
			},
			wantErr: false,
		},
		{
			name:      "Empty datapoints",
			namespace: "test-namespace",
			metric: metric.Metric[float64]{
				Name:       "test-metric",
				Unit:       metric.UnitCount,
				Kind:       metric.Sum,
				DataPoints: []metric.DataPoint[float64]{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize the collector
			c := NewInMemoryMetricCollector(context.NewMockDefault())

			// Execute the method
			err := c.CollectMetric(tt.namespace, tt.metric)

			// Assert results
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				result, err := c.FetchAllAndDrop()
				assert.NoError(t, err)

				if len(tt.metric.DataPoints) == 0 {
					assert.Equal(t, metric.NamespaceMetrics[float64]{
						"test-namespace": []metric.Metric[float64]{
							{
								Name:       tt.metric.Name,
								Unit:       tt.metric.Unit,
								Kind:       tt.metric.Kind,
								DataPoints: []metric.DataPoint[float64]{},
							},
						},
					}, result)
				} else {
					expectedValue := 0.0
					for _, point := range tt.metric.DataPoints {
						expectedValue += point.Value
					}

					spanStartTime := tt.metric.DataPoints[0].StartTime.Truncate(time.Second)

					assert.Equal(t, metric.NamespaceMetrics[float64]{
						"test-namespace": []metric.Metric[float64]{
							{
								Name: tt.metric.Name,
								Unit: tt.metric.Unit,
								Kind: tt.metric.Kind,
								DataPoints: []metric.DataPoint[float64]{
									{
										StartTime: spanStartTime,
										EndTime:   spanStartTime.Add(time.Second),
										Value:     expectedValue,
									},
								},
							},
						},
					}, result)
				}
			}
		})
	}
}

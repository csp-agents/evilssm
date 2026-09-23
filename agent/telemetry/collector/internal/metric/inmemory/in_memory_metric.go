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
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/common/telemetry/metric"
)

type namespacedAggregatedMetric struct {
	metricsMapMtx *sync.Mutex

	// map from namespace -> metric name -> data points
	metrics map[string]map[string]*timeAggregatedMetric[float64]
}

func NewInMemoryMetricCollector(context context.T) *namespacedAggregatedMetric {
	return &namespacedAggregatedMetric{
		metricsMapMtx: &sync.Mutex{},
		metrics:       make(map[string]map[string]*timeAggregatedMetric[float64]),
	}
}

func (c *namespacedAggregatedMetric) CollectMetric(namespace string, metric metric.Metric[float64]) error {
	c.metricsMapMtx.Lock()
	defer c.metricsMapMtx.Unlock()

	if c.metrics[namespace] == nil {
		c.metrics[namespace] = make(map[string]*timeAggregatedMetric[float64])
	}

	aggregatedMetric := c.metrics[namespace][metric.Name]

	if aggregatedMetric == nil {
		var err error
		aggregatedMetric, err = newTimeAggregatedMetric[float64](metric.Name, metric.Unit, metric.Kind)

		if err != nil {
			return err
		}

		c.metrics[namespace][metric.Name] = aggregatedMetric
	}
	errs := make([]error, 0)

	for _, datapoint := range metric.DataPoints {
		err := aggregatedMetric.aggregate(datapoint)
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (c *namespacedAggregatedMetric) FetchAndDrop(limit int) (metric.NamespaceMetrics[float64], error) {
	panic("this method should not be used. Use FetchAllAndDrop instead")
}

func (c *namespacedAggregatedMetric) FetchAllAndDrop() (metric.NamespaceMetrics[float64], error) {
	c.metricsMapMtx.Lock()
	defer c.metricsMapMtx.Unlock()

	result := metric.NamespaceMetrics[float64]{}

	for namespace, metrics := range c.metrics {
		namespaceMetrics := make([]metric.Metric[float64], 0, len(metrics))

		for _, aggregatedMetric := range metrics {
			namespaceMetrics = append(namespaceMetrics, metric.Metric[float64]{
				Name:       aggregatedMetric.name,
				Unit:       aggregatedMetric.unit,
				Kind:       aggregatedMetric.kind,
				DataPoints: aggregatedMetric.fetch(),
			})
		}

		result[namespace] = namespaceMetrics
	}

	clear(c.metrics)

	return result, nil
}

func (c *namespacedAggregatedMetric) Close() (err error) {
	c.metricsMapMtx.Lock()
	defer c.metricsMapMtx.Unlock()

	clear(c.metrics)

	return nil
}

// timeAggregatedMetric holds metrics aggregated by 1-second time spans. For example, if we get 500 data
// points within a single seconds for a metric, it will aggregate them to a single data point and store it.
// if another metric comes after that one second, it will be aggregated in anothe [timeSpan].
type timeAggregatedMetric[N int64 | float64] struct {
	name           string
	unit           metric.Unit
	kind           metric.Kind
	kindAggregator kindAggregator[N]
	spansMtx       *sync.RWMutex
	spans          map[timeSpan]aggregateDataPoint
}

// newTimeAggregatedMetric creates and returns a new instance of [timeAggregatedMetric]
func newTimeAggregatedMetric[N int64 | float64](name string, unit metric.Unit, kind metric.Kind) (*timeAggregatedMetric[N], error) {
	kc, err := newMetricKindAggregator[N](kind)
	if err != nil {
		return nil, err
	}
	m := timeAggregatedMetric[N]{
		name:           name,
		unit:           unit,
		kind:           kind,
		spansMtx:       &sync.RWMutex{},
		kindAggregator: kc,
		spans:          make(map[timeSpan]aggregateDataPoint),
	}
	return &m, nil
}

// aggregate implements the [aggregatedMetric] interface
func (m *timeAggregatedMetric[N]) aggregate(point metric.DataPoint[N]) error {
	m.spansMtx.Lock()
	defer m.spansMtx.Unlock()

	if !point.EndTime.Equal(point.StartTime) {
		return errors.New("invalid datapoint. Only a single point in time is supported")
	}

	// the closest second before the start time of this data point
	truncatedStartTime := point.StartTime.Truncate(time.Second)

	lookupSpan := timeSpan{
		startTime: truncatedStartTime,
		endTime:   truncatedStartTime.Add(time.Second),
	}

	previousDataPoint, exists := m.spans[lookupSpan]

	if !exists {
		// create new 1-second time span
		m.spans[lookupSpan] = aggregateDataPoint{
			DataPoint: metric.DataPoint[float64]{
				StartTime: lookupSpan.startTime,
				EndTime:   lookupSpan.endTime,
				Value:     float64(point.Value),
			},
			dataPointCounts: 1,
		}
	} else {
		m.spans[lookupSpan] = m.kindAggregator.aggregate(previousDataPoint, point.Value)
	}

	return nil
}

// fetchAndDrop fetches all of the aggregated data points and drops them from the metric
func (m *timeAggregatedMetric[N]) fetch() []metric.DataPoint[N] {
	m.spansMtx.RLock()
	defer m.spansMtx.RUnlock()

	result := make([]metric.DataPoint[N], 0, len(m.spans))

	for span, aggregatedDataPoint := range m.spans {
		result = append(result, metric.DataPoint[N]{
			StartTime: span.startTime,
			EndTime:   span.endTime,
			Value:     N(aggregatedDataPoint.Value),
		})
	}

	return result
}

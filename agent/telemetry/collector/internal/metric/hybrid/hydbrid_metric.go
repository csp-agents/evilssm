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
package hybrid

import (
	"errors"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/context"
	metricCollector "github.com/aws/amazon-ssm-agent/agent/telemetry/collector/internal/metric"
	"github.com/aws/amazon-ssm-agent/agent/telemetry/collector/internal/metric/inmemory"
	"github.com/aws/amazon-ssm-agent/agent/telemetry/collector/internal/metric/rolling"
	"github.com/aws/amazon-ssm-agent/common/telemetry/metric"

	"github.com/carlescere/scheduler"
)

type FastMetricsCollector interface {
	metricCollector.MetricsCollector

	FetchAllAndDrop() (metric.NamespaceMetrics[float64], error)
}

// FlushableMetricsCollector supports flushing. This interface is suitable for disk/network based collectors
type FlushableMetricsCollector interface {
	metricCollector.MetricsCollector
	Flush() error
}

// hybridMetricCollector collects metrics in-memory and periodically flushes to disk
type hybridMetricCollector struct {
	diskWriteMtx          *sync.Mutex
	diskWriteSchedulerJob *scheduler.Job
	inMemoryCollector     FastMetricsCollector
	onDiskCollector       FlushableMetricsCollector
}

func NewHybridMetricCollector(context context.T, fileNamePrefix string, flushPeriodSeconds int) (c *hybridMetricCollector, err error) {
	log := context.Log()

	c = &hybridMetricCollector{
		diskWriteMtx:      &sync.Mutex{},
		inMemoryCollector: inmemory.NewInMemoryMetricCollector(context),
		onDiskCollector:   rolling.NewRollingDiskMetricCollector(context, fileNamePrefix),
	}

	// start writing to disk periodically
	if c.diskWriteSchedulerJob, err = scheduler.Every(flushPeriodSeconds).NotImmediately().Seconds().Run(func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("Metric disk write panic: %v", r)
				log.Errorf("Stacktrace:\n%s", debug.Stack())
			}
		}()

		writeErr := c.writeToDisk()
		if writeErr != nil {
			log.Warnf("Error when writing metrics to disk: %v", err)
		}
	}); err != nil {
		return nil, fmt.Errorf("unable to schedule metric flush process: %v", err)
	}

	return c, nil
}

func (c *hybridMetricCollector) CollectMetric(namespace string, metric metric.Metric[float64]) error {
	return c.inMemoryCollector.CollectMetric(namespace, metric)
}

func (c *hybridMetricCollector) FetchAndDrop(limit int) (metric.NamespaceMetrics[float64], error) {
	c.diskWriteMtx.Lock()
	defer c.diskWriteMtx.Unlock()

	// write in-memory metrics to disk
	err := c.unlockedWriteToDisk()
	if err != nil {
		return nil, err
	}

	return c.onDiskCollector.FetchAndDrop(limit)
}

// writeToDisk writes the aggregated metrics from memory to the disk. This is a locking operation
func (c *hybridMetricCollector) writeToDisk() error {
	c.diskWriteMtx.Lock()
	defer c.diskWriteMtx.Unlock()

	return c.unlockedWriteToDisk()
}

// unlockedWriteToDisk does what [hybridMetricCollector.writeToDisk] does but without the locking
func (c *hybridMetricCollector) unlockedWriteToDisk() error {
	inMemMetrics, err := c.inMemoryCollector.FetchAllAndDrop()

	if err != nil {
		return err
	}

	for ns, metrics := range inMemMetrics {
		for _, m := range metrics {
			if err := c.onDiskCollector.CollectMetric(ns, m); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *hybridMetricCollector) Flush() error {
	c.diskWriteMtx.Lock()
	defer c.diskWriteMtx.Unlock()

	if err := c.unlockedWriteToDisk(); err != nil {
		return err
	}

	if err := c.onDiskCollector.Flush(); err != nil {
		return err
	}

	return nil
}

func (c *hybridMetricCollector) Close() error {
	c.diskWriteMtx.Lock()
	defer c.diskWriteMtx.Unlock()

	// stop the disk writer job
	c.diskWriteSchedulerJob.Quit <- true

	errs := make([]error, 0, 2)
	errs = append(errs, c.inMemoryCollector.Close(), c.onDiskCollector.Close())

	return errors.Join(errs...)
}

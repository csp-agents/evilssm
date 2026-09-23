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
package internal

import (
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"runtime/debug"
	"slices"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	rollingLog "github.com/aws/amazon-ssm-agent/agent/telemetry/collector/internal/log/rolling"
	metricCollector "github.com/aws/amazon-ssm-agent/agent/telemetry/collector/internal/metric"
	"github.com/aws/amazon-ssm-agent/agent/telemetry/collector/internal/metric/hybrid"
	"github.com/aws/amazon-ssm-agent/agent/telemetry/exporter"
	"github.com/aws/amazon-ssm-agent/common/telemetry"
	"github.com/aws/amazon-ssm-agent/common/telemetry/metric"
	"github.com/aws/amazon-ssm-agent/common/telemetry/telemetrylog"

	"github.com/carlescere/scheduler"
)

type LogCollector interface {
	// CollectLog is used to ingest a log entry into the collector
	CollectLog(namespace string, log telemetrylog.Entry) error

	// FetchAndDrop fetches maximum of [limit] number of logs ingested until now in all the namespaces
	FetchAndDrop(limit int) (telemetrylog.NamespaceLogs, error)

	Close() error
}

type Collector interface {
	// CollectLog is used to ingest a log entry into the collector
	CollectLog(namespace string, log telemetrylog.Entry) error

	// CollectMetric is used to ingest a metric into the collector
	CollectMetric(namespace string, metric metric.Metric[float64]) error

	// AddExporter adds a new Exporter to the collector
	AddExporter(exporter exporter.Exporter)

	// RemoveExporter removes an Exporter from the collector
	RemoveExporter(exporter exporter.Exporter)

	Close() error
}

type collectorT struct {
	context           context.T
	aggregationPeriod time.Duration

	// collectorMtx is for locking the metric and log collection
	collectorMtx    *sync.Mutex
	metricCollector metricCollector.MetricsCollector
	logCollector    LogCollector

	// exporterMtx locks the exporters list below
	exporterMtx *sync.RWMutex
	exporters   []exporter.Exporter

	//exportSchedulerJobMtx locks exportSchedulerJob
	exportSchedulerJobMtx *sync.RWMutex
	exportSchedulerJob    *scheduler.Job

	exportSchedulerChannel       chan bool
	exportSchedulerInitialJitter time.Duration
}

func NewCollector(context context.T, aggregationPeriod, exportPeriod time.Duration) (Collector, error) {
	logCollector := rollingLog.NewRollingLogCollector(context, "logs")
	metricCollector, err := hybrid.NewHybridMetricCollector(context, "metrics", int(aggregationPeriod.Seconds()))

	if err != nil {
		return nil, err
	}

	exportPeriodSeconds := int(exportPeriod.Seconds())
	// #nosec G404
	jitter := time.Duration(rand.IntN(exportPeriodSeconds)) * time.Second

	c := &collectorT{
		context:                      context,
		aggregationPeriod:            aggregationPeriod,
		metricCollector:              metricCollector,
		logCollector:                 logCollector,
		collectorMtx:                 &sync.Mutex{},
		exporterMtx:                  &sync.RWMutex{},
		exportSchedulerJobMtx:        &sync.RWMutex{},
		exportSchedulerChannel:       make(chan bool),
		exportSchedulerInitialJitter: jitter,
	}

	if exportPeriodSeconds <= 0 {
		return nil, fmt.Errorf("export period is too small")
	}

	log := context.Log()

	go func() {
		//Adding random jitter sleep before kicking off scheduler to "spread out" traffic uniformly
		log.Debugf("Will start telemetry scheduler after %v seconds", c.exportSchedulerInitialJitter)

		select {
		case <-time.After(c.exportSchedulerInitialJitter):
			// Select will block till message is ready on either of channels, so we basically sleep for jitter seconds
		case _, ok := <-c.exportSchedulerChannel:
			if !ok {
				// Channel has been closed, indicating graceful termination of goroutine required
				log.Infof("Shutdown requested for exporter goroutine")
				return
			}
		}

		func() {
			c.exportSchedulerJobMtx.Lock()
			defer c.exportSchedulerJobMtx.Unlock()

			if c.exportSchedulerJob, err = scheduler.Every(exportPeriodSeconds).NotImmediately().Seconds().Run(func() {
				defer func() {
					if r := recover(); r != nil {
						log.Errorf("Telemetry export panic: %v", r)
						log.Errorf("Stacktrace:\n%s", debug.Stack())
					}
				}()

				exportErr := c.export()
				if exportErr != nil {
					log.Warnf("Error when exporting telemetry: %v", err)
				}
			}); err != nil {
				log.Errorf("unable to schedule telemetry exporter process: %v", err)
			}
		}()
	}()

	return c, nil
}

// CollectMetric is used to ingest a metric into the collector
func (c *collectorT) CollectMetric(namespace string, metric metric.Metric[float64]) error {
	c.collectorMtx.Lock()
	defer c.collectorMtx.Unlock()

	return c.metricCollector.CollectMetric(namespace, metric)
}

// CollectLog is used to ingest a log entry into the collector
func (c *collectorT) CollectLog(namespace string, log telemetrylog.Entry) error {
	c.collectorMtx.Lock()
	defer c.collectorMtx.Unlock()

	return c.logCollector.CollectLog(namespace, log)
}

// AddExporter adds a new Exporter to the collector with the specified export period
func (c *collectorT) AddExporter(exporter exporter.Exporter) {
	c.exporterMtx.Lock()
	defer c.exporterMtx.Unlock()

	c.exporters = append(c.exporters, exporter)
}

// RemoveExporter removes an Exporter from the collector
func (c *collectorT) RemoveExporter(exporter exporter.Exporter) {
	c.exporterMtx.Lock()
	defer c.exporterMtx.Unlock()

	for i, e := range c.exporters {
		if e == exporter {
			c.exporters = slices.Delete(c.exporters, i, i+1)
			break
		}
	}
}

// export exports all the telemetry the collector holds (both in-memory and on disk) in reasonable chunks
func (c *collectorT) export() error {
	// stop any more collection until all existing telemetry is exported
	c.collectorMtx.Lock()
	defer c.collectorMtx.Unlock()

	c.exporterMtx.RLock()
	defer c.exporterMtx.RUnlock()

	// don't want to lose telemetry until exporters are attached
	if len(c.exporters) == 0 {
		c.context.Log().Debugf("No exporters attached, skipping export")
		return nil
	}

	var errMetrics, errLogs error
	var exportErrs []error
	for errMetrics == nil || errLogs == nil {
		var metrics metric.NamespaceMetrics[float64]
		var logs telemetrylog.NamespaceLogs

		metrics, errMetrics = c.metricCollector.FetchAndDrop(1000)
		logs, errLogs = c.logCollector.FetchAndDrop(1000)

		if errMetrics != nil && errMetrics != io.EOF {
			if errLogs == io.EOF {
				errLogs = nil // don't send the EOF error
			}
			return errors.Join(errMetrics, errLogs)
		}

		if errLogs != nil && errLogs != io.EOF {
			return errLogs
		}

		if len(metrics) == 0 && len(logs) == 0 {
			return nil
		}

		// get all the unique namespaces in both metrics and logs
		namespaces := make(map[string]bool)
		for ns := range metrics {
			namespaces[ns] = true
		}
		for ns := range logs {
			namespaces[ns] = true
		}

		// send telemetry for all namespaces
		for ns := range namespaces {
			// we read the logs from the disk so we need to truncate them
			for i := range logs[ns] {
				logs[ns][i].Body = telemetry.TruncateLog(logs[ns][i].Body)
			}

			err := c.exportNamespaceTelemetry(ns, metrics[ns], logs[ns])
			exportErrs = append(exportErrs, err)
		}

		if errMetrics == io.EOF && errLogs == io.EOF {
			return nil
		}
	}

	return errors.Join(errors.Join(errMetrics, errLogs), errors.Join(exportErrs...))
}

// exportNamespaceTelemetry exports telemetry for a specific namespace to the attached exporters
func (c *collectorT) exportNamespaceTelemetry(namespace string, metrics []metric.Metric[float64], logs []telemetrylog.Entry) error {
	c.exporterMtx.RLock()
	defer c.exporterMtx.RUnlock()

	if metrics == nil {
		metrics = []metric.Metric[float64]{}
	}
	if logs == nil {
		logs = []telemetrylog.Entry{}
	}

	if len(metrics) == 0 && len(logs) == 0 {
		return nil
	}

	var errs []error

	for _, exporter := range c.exporters {
		err := exporter.Export(namespace, metrics, logs)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (c *collectorT) Close() error {
	c.collectorMtx.Lock()
	defer c.collectorMtx.Unlock()

	func() {
		c.exportSchedulerJobMtx.Lock()
		defer c.exportSchedulerJobMtx.Unlock()

		if c.exportSchedulerJob != nil {
			c.exportSchedulerJob.Quit <- true
		}
	}()

	if c.exportSchedulerChannel != nil {
		close(c.exportSchedulerChannel)
	}

	errs := make([]error, 0, 2)
	errs = append(errs, c.metricCollector.Close(), c.logCollector.Close())

	return errors.Join(errs...)
}

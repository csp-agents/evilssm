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
package rolling

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"runtime/debug"
	"sync"

	"github.com/cihub/seelog"
	"golang.org/x/sync/errgroup"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/telemetry/collector/utils"
	dynamicconfiguration "github.com/aws/amazon-ssm-agent/agent/telemetry/dynamic_configuration"
	"github.com/aws/amazon-ssm-agent/common/telemetry/metric"
)

// rollingDiskMetricCollector holds a [seelog.LoggerInterface] instance
type rollingDiskMetricCollector struct {
	ctx context.T

	// directory path
	dirPath string

	logger seelog.LoggerInterface
}

// namespacedDiskMetricCollector holds namespace to [rollingDiskMetricCollector] mapping
type namespacedDiskMetricCollector struct {
	ctx context.T

	baseDir string

	// prefix of each log file
	fileNamePrefix string

	// mutex for locking metric collection
	metricCollectorMtx *sync.Mutex

	// mutex for the collectorMap below
	collectorMapMtx *sync.RWMutex
	collectorMap    map[string]*rollingDiskMetricCollector
}

// for mocking the write directory since TelemetryDataStorePath is a constant
var getBaseMetricsStoreDir = func() string {
	return filepath.Join(appconfig.TelemetryDataStorePath, "metrics")
}

func NewRollingDiskMetricCollector(context context.T, fileNamePrefix string) *namespacedDiskMetricCollector {
	return &namespacedDiskMetricCollector{
		ctx:                context,
		baseDir:            getBaseMetricsStoreDir(),
		fileNamePrefix:     fileNamePrefix,
		metricCollectorMtx: &sync.Mutex{},
		collectorMapMtx:    &sync.RWMutex{},
		collectorMap:       make(map[string]*rollingDiskMetricCollector),
	}
}

// CollectMetric stores the metric on disk asynchronously
func (c *namespacedDiskMetricCollector) CollectMetric(namespace string, metric metric.Metric[float64]) error {
	c.metricCollectorMtx.Lock()
	defer c.metricCollectorMtx.Unlock()

	logger := c.ctx.Log()
	metricBytes, err := json.Marshal(metric)

	if err != nil {
		logger.Debugf("Error in marshaling in rolling log collector %v", err)
		return err
	}
	rw, err := c.getMetricCollector(namespace)

	if err != nil {
		logger.Debugf("Error getting inner rolling log collector %v", err)
		return err
	}

	err = rw.write(metricBytes)
	if err != nil {
		logger.Debugf("Error in writing to inner collector in rolling metric collector %v", err)
	}
	return err
}

// FetchAndDrop fetches maximum of [limit] number of metrics ingested until now in all the namespaces
func (c *namespacedDiskMetricCollector) FetchAndDrop(limit int) (metric.NamespaceMetrics[float64], error) {
	// need to stop collection since we will be truncating the written files
	c.metricCollectorMtx.Lock()
	defer c.metricCollectorMtx.Unlock()

	c.Flush() // finish any pending writes

	log := c.ctx.Log()

	nsMap, err := utils.ReadAndDeleteRollingLogs(log, c.baseDir, c.fileNamePrefix, limit)

	if err != nil && err != io.EOF {
		return nil, err
	}

	result := metric.NamespaceMetrics[float64]{}

	for ns, lines := range nsMap {
		metrics := utils.UnmarshalList[metric.Metric[float64]](lines, log)

		result[ns] = metrics
	}

	return result, err
}

// Flush forces immediate and synchronous writing of the metrics to disk
func (c *namespacedDiskMetricCollector) Flush() (err error) {
	var eg errgroup.Group
	eg.SetLimit(4) // limit to 4 parallel flushes

	c.collectorMapMtx.RLock()

	// flush collectors for each namespace in parallel
	for _, innerCollector := range c.collectorMap {
		func(collector *rollingDiskMetricCollector) {
			eg.Go(func() (err error) {
				defer func() {
					if r := recover(); r != nil {
						c.ctx.Log().Warnf("rollingDiskMetricCollector flush panic: %v", r)
						c.ctx.Log().Warnf("Stacktrace:\n%s", debug.Stack())
						err = fmt.Errorf("panic in rollingDiskMetricCollector flush %v", r)
					}
				}()

				collector.flush()
				return nil
			})
		}(innerCollector)
	}

	c.collectorMapMtx.RUnlock()

	// Wait for all goroutines to finish
	return eg.Wait()
}

// Close closes the collector
func (c *namespacedDiskMetricCollector) Close() error {
	var eg errgroup.Group
	eg.SetLimit(4) // limit to 4 parallel closes

	c.collectorMapMtx.Lock()
	defer c.collectorMapMtx.Unlock()

	// close collectors for each namespace in parallel
	for _, innerCollector := range c.collectorMap {
		func(collector *rollingDiskMetricCollector) {
			eg.Go(func() (err error) {
				defer func() {
					if r := recover(); r != nil {
						c.ctx.Log().Warnf("rollingDiskMetricCollector close panic: %v", r)
						c.ctx.Log().Warnf("Stacktrace:\n%s", debug.Stack())
						err = fmt.Errorf("panic in rollingDiskMetricCollector close %v", r)
					}
				}()

				collector.close()
				return nil
			})
		}(innerCollector)
	}

	// Wait for all goroutines to finish
	err := eg.Wait()
	if err != nil {
		return err
	}

	clear(c.collectorMap)
	return nil
}

// getMetricCollector returns a [rollingDiskMetricCollector] for the given namespace
func (c *namespacedDiskMetricCollector) getMetricCollector(namespace string) (*rollingDiskMetricCollector, error) {
	c.collectorMapMtx.Lock()
	defer c.collectorMapMtx.Unlock()

	if namespace == "" {
		return nil, fmt.Errorf("namespace cannot be empty")
	}

	if c.collectorMap[namespace] == nil {
		p := filepath.Join(c.baseDir, namespace)
		if err := fileutil.MakeDirs(p); err != nil {
			return nil, err
		}

		loggerConfig := utils.GetLoggerConfig(p, c.fileNamePrefix, dynamicconfiguration.MaxRolls(namespace), dynamicconfiguration.MaxRollSize(namespace))
		seelogger, err := seelog.LoggerFromConfigAsBytes(loggerConfig)
		if err != nil {
			return nil, err
		}

		rw := &rollingDiskMetricCollector{
			ctx:     c.ctx,
			dirPath: p,
			logger:  seelogger,
		}

		c.collectorMap[namespace] = rw
	}

	return c.collectorMap[namespace], nil
}

func (c *rollingDiskMetricCollector) write(bytes []byte) (err error) {
	if c.logger == nil {
		return errors.New("logger is not intialized")
	}

	c.logger.Trace(string(bytes))
	return nil
}

func (rw *rollingDiskMetricCollector) flush() {
	rw.logger.Flush()
}

func (rw *rollingDiskMetricCollector) close() {
	rw.logger.Close()

	// remove the namespace direcory if it is empty. ignore errors
	err := utils.DeleteDirectoryIfAllFilesEmpty(rw.dirPath)
	if err != nil {
		rw.ctx.Log().Warnf("Failed to delete telemetry directory %s: %v", rw.dirPath, err)
	}
}

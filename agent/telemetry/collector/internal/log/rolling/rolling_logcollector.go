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
	"github.com/aws/amazon-ssm-agent/common/telemetry/telemetrylog"
)

// rollingLogCollector holds rolling log state of a single namespace
type rollingLogCollector struct {
	ctx context.T

	// directory path
	dirPath string

	// seelog instnace
	logger seelog.LoggerInterface
}

// namespacedRollingLogCollector holds a rollingLogCollector for each namespace
type namespacedRollingLogCollector struct {
	ctx context.T

	baseDir string
	// prefix of each log file
	fileNamePrefix string

	// mutex for locking log collection
	logCollectorMtx *sync.Mutex

	// mutex for the collectorMap below
	collectorMapMtx *sync.RWMutex
	collectorMap    map[string]*rollingLogCollector
}

// for mocking the write directory since TelemetryDataStorePath is a constant
var getBaseLogStoreDir = func() string {
	return filepath.Join(appconfig.TelemetryDataStorePath, "logs")
}

func NewRollingLogCollector(context context.T, fileNamePrefix string) *namespacedRollingLogCollector {
	return &namespacedRollingLogCollector{
		ctx:             context,
		baseDir:         getBaseLogStoreDir(),
		fileNamePrefix:  fileNamePrefix,
		logCollectorMtx: &sync.Mutex{},
		collectorMapMtx: &sync.RWMutex{},
		collectorMap:    make(map[string]*rollingLogCollector),
	}
}

// CollectLog stores the log on disk asynchronously
func (c *namespacedRollingLogCollector) CollectLog(namespace string, entry telemetrylog.Entry) error {
	c.logCollectorMtx.Lock()
	defer c.logCollectorMtx.Unlock()

	logger := c.ctx.Log()

	entryBytes, err := json.Marshal(entry)

	if err != nil {
		logger.Debugf("Error in marshaling in rolling log collector %v", err)
		return err
	}
	rw, err := c.getLogCollector(namespace)

	if err != nil {
		logger.Debugf("Error getting inner rolling log collector %v", err)
		return err
	}

	err = rw.write(entryBytes)
	if err != nil {
		logger.Debugf("Error in writing to inner collector in rolling log collector %v", err)
	}
	return err
}

// FetchAndDrop fetches maximum of [limit] number of logs ingested until now in all the namespaces
func (c *namespacedRollingLogCollector) FetchAndDrop(limit int) (telemetrylog.NamespaceLogs, error) {
	// need to stop collection since we will be truncating the written files
	c.logCollectorMtx.Lock()
	defer c.logCollectorMtx.Unlock()

	c.Flush() // finish any pending writes

	log := c.ctx.Log()

	nsMap, err := utils.ReadAndDeleteRollingLogs(log, c.baseDir, c.fileNamePrefix, limit)

	if err != nil && err != io.EOF {
		return nil, err
	}

	result := telemetrylog.NamespaceLogs{}

	for ns, lines := range nsMap {
		logs := utils.UnmarshalList[telemetrylog.Entry](lines, log)

		result[ns] = logs
	}

	return result, err
}

// Flush forces immediate and synchronous writing of the logs to disk
func (c *namespacedRollingLogCollector) Flush() error {
	var eg errgroup.Group
	eg.SetLimit(4) // limit to 4 parallel flushes

	c.collectorMapMtx.RLock()

	// flush collectors for each namespace in parallel
	for _, innerCollector := range c.collectorMap {
		func(collector *rollingLogCollector) {
			eg.Go(func() (err error) {
				defer func() {
					if r := recover(); r != nil {
						c.ctx.Log().Warnf("rollingLogCollector flush panic: %v", r)
						c.ctx.Log().Warnf("Stacktrace:\n%s", debug.Stack())
						err = fmt.Errorf("panic in rollingLogCollector flush %v", r)
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

func (c *namespacedRollingLogCollector) Close() (err error) {
	var eg errgroup.Group
	eg.SetLimit(4) // limit to 4 parallel closes

	c.collectorMapMtx.Lock()
	defer c.collectorMapMtx.Unlock()

	// close collectors for each namespace in parallel
	for _, innerCollector := range c.collectorMap {
		func(collector *rollingLogCollector) {
			eg.Go(func() (err error) {
				defer func() {
					if r := recover(); r != nil {
						c.ctx.Log().Warnf("rollingLogCollector close panic: %v", r)
						c.ctx.Log().Warnf("Stacktrace:\n%s", debug.Stack())
						err = fmt.Errorf("panic in rollingLogCollector close %v", r)
					}
				}()

				collector.close()
				return nil
			})
		}(innerCollector)
	}

	// Wait for all goroutines to finish
	err = eg.Wait()
	if err != nil {
		return err
	}

	clear(c.collectorMap)

	return nil
}

// getLogCollector returns a [namespacedRollingLogCollector] for the given namespace
func (c *namespacedRollingLogCollector) getLogCollector(namespace string) (*rollingLogCollector, error) {
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
		maxRolls := dynamicconfiguration.MaxRolls(namespace)
		maxFileSize := dynamicconfiguration.MaxRollSize(namespace)
		loggerConfig := utils.GetLoggerConfig(p, c.fileNamePrefix, maxRolls, maxFileSize)
		seelogger, err := seelog.LoggerFromConfigAsBytes(loggerConfig)
		if err != nil {
			return nil, err
		}

		rw := &rollingLogCollector{
			ctx:     c.ctx,
			dirPath: p,
			logger:  seelogger,
		}

		c.collectorMap[namespace] = rw
	}

	return c.collectorMap[namespace], nil
}

func (c *rollingLogCollector) write(bytes []byte) (err error) {
	if c.logger == nil {
		return errors.New("logger is not intialized")
	}

	c.logger.Trace(string(bytes))
	return nil
}

func (rw *rollingLogCollector) flush() {
	rw.logger.Flush()
}

func (rw *rollingLogCollector) close() {
	rw.logger.Close()

	// remove the namespace direcory if it is empty. ignore errors
	err := utils.DeleteDirectoryIfAllFilesEmpty(rw.dirPath)
	if err != nil {
		rw.ctx.Log().Warnf("Failed to delete telemetry directory %s: %v", rw.dirPath, err)
	}
}

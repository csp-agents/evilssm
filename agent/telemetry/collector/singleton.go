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
package collector

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/telemetry/collector/internal"
	"github.com/aws/amazon-ssm-agent/agent/telemetry/exporter"
	"github.com/aws/amazon-ssm-agent/common/telemetry"
	"github.com/aws/amazon-ssm-agent/common/telemetry/emitter"
	"github.com/aws/amazon-ssm-agent/common/telemetry/metric"
	"github.com/aws/amazon-ssm-agent/common/telemetry/telemetrylog"
)

const (
	// period of in-memory telemetry aggregation before flushing to disk
	aggregationPeriodSeconds = 10 //  aggregate for 10 seconds

	// period of exporting the telemetry to the attached exporters
	exportPeriodSeconds = 300 // export every 5 minutes

	// period of polling for the telemetry emitted by all processes
	consumerPollPeriodSeconds = 120 // poll every 2 minutes
)

var (
	// lock to protect the singleton
	singletonMutex = new(sync.RWMutex)
	singleton      *collector
)

type collector struct {
	collector internal.Collector

	// a [consumer] instance
	consumer *consumer

	// WaitGroup to wait until the telemetry collection is stopped in StopCollection
	listenWg *sync.WaitGroup
}

func collectMetric(namespace string, metric metric.Metric[float64]) error {
	singletonMutex.Lock()
	defer singletonMutex.Unlock()

	if singleton == nil {
		return fmt.Errorf("telemetry collector not initialized")
	}

	return singleton.collector.CollectMetric(namespace, metric)
}

func collectLog(namespace string, log telemetrylog.Entry) error {
	singletonMutex.Lock()
	defer singletonMutex.Unlock()

	if singleton == nil {
		return fmt.Errorf("telemetry collector not initialized")
	}

	// truncate the log body
	log.Body = telemetry.TruncateLog(log.Body)

	return singleton.collector.CollectLog(namespace, log)
}

// StartCollection starts telemetry collection for a specified telemetry context
// internally, it creates the receiver end of the telemetry channel and collects the telemetry
// from it.
func StartCollection(context context.T) (err error) {
	singletonMutex.Lock()
	defer singletonMutex.Unlock()

	log := context.Log()
	defer func() {
		if r := recover(); r != nil {
			log.Warnf("Telemetry StartCollection panic: %v", r)
			log.Warnf("Stacktrace:\n%s", debug.Stack())
			err = fmt.Errorf("panic in telemetry collector StartCollection %v", r)
		}
	}()

	c, err := internal.NewCollector(context, aggregationPeriodSeconds*time.Second, exportPeriodSeconds*time.Second)
	if err != nil {
		return err
	}

	consumer := newConsumer(context, consumerPollPeriodSeconds) // poll for telemetry every 2 minutes
	err = consumer.start()
	if err != nil {
		log.Warnf("Could not start telemetry consumerfor with error: %v", err)
		return err
	}

	listenWg := new(sync.WaitGroup)
	singleton = &collector{
		collector: c,
		consumer:  consumer,
		listenWg:  listenWg,
	}

	listenWg.Add(1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Warnf("Telemetry channel listener panic: %v", r)
				log.Warnf("Stacktrace:\n%s", debug.Stack())
			}
		}()

		defer listenWg.Done()

		for {
			message, more := <-consumer.getMessage()

			if !more {
				// safe close
				log.Debug("consumer closed, stop telemetry listener")
				return
			}

			log.Debugf("Received message from consumer : %v", message)
			if err := processMessage(message); err != nil {
				log.Debugf("Error processing telemetry message: %v", err)
			}
		}
	}()

	log.Debugf("Telemetry collection started ")

	return nil
}

// StopCollection stops telemetry collection for a specified telemetry context
func StopCollection(log log.T) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Warnf("Telemetry StopCollection panic: %v", r)
			log.Warnf("Stacktrace:\n%s", debug.Stack())
			err = fmt.Errorf("panic in telemetry collector StopCollection %v", r)
		}
	}()

	singletonMutex.RLock()
	defer singletonMutex.RUnlock()

	if singleton == nil {
		return fmt.Errorf("telemetry collector is not started")
	}

	log.Debugf("Stopping telemetry collection")

	singleton.consumer.stop()

	// wait for the listener to stop
	singleton.listenWg.Wait()

	// stop the collector
	singleton.collector.Close()

	singleton = nil
	return nil
}

func processMessage(message namespaceMessage) error {
	switch message.message.Type {
	case emitter.LOG:
		var logEntry telemetrylog.Entry

		if err := json.Unmarshal([]byte(message.message.Payload), &logEntry); err != nil {
			return err
		}

		return collectLog(message.namespace, logEntry)
	case emitter.METRIC:
		var metric metric.Metric[float64]

		if err := json.Unmarshal([]byte(message.message.Payload), &metric); err != nil {
			return err
		}

		return collectMetric(message.namespace, metric)
	default:
		return fmt.Errorf("unknown message type: %v", message.message.Type)
	}
}

// AddExporter adds a new Exporter to the collector with the specified export period
func AddExporter(log log.T, exporter exporter.Exporter) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Warnf("Telemetry AddExporter panic: %v", r)
			log.Warnf("Stacktrace:\n%s", debug.Stack())
			err = fmt.Errorf("panic in singleton collector AddExporter %v", r)
		}
	}()

	singletonMutex.RLock()
	defer singletonMutex.RUnlock()

	if singleton == nil {
		return fmt.Errorf("cannot add exporter. telemetry collector not initialized")
	}

	singleton.collector.AddExporter(exporter)
	return nil
}

// RemoveExporter removes an Exporter from the collector
func RemoveExporter(log log.T, exporter exporter.Exporter) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Warnf("Telemetry RemoveExporter panic: %v", r)
			log.Warnf("Stacktrace:\n%s", debug.Stack())
			err = fmt.Errorf("panic in singleton collector RemoveExporter %v", r)
		}
	}()

	singletonMutex.RLock()
	defer singletonMutex.RUnlock()

	if singleton == nil {
		return fmt.Errorf("cannot remove exporter. telemetry collector not initialized")
	}

	singleton.collector.RemoveExporter(exporter)
	return nil
}

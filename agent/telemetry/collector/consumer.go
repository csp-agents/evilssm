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
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/advisorylock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/telemetry/collector/utils"
	"github.com/aws/amazon-ssm-agent/common/telemetry/emitter"

	"github.com/carlescere/scheduler"
)

const (
	advisoryLockTimeoutSeconds = 5
)

type namespaceMessage struct {
	// The namespace of the telemetry message
	namespace string
	message   emitter.Message
}

// consumer polls on the telemetry pre-ingestion directory
type consumer struct {
	log               log.T
	pollPeriodSeconds int
	onMessageChan     chan namespaceMessage
	consumerJobMtx    *sync.Mutex
	consumerJob       *scheduler.Job
	stopOnce          sync.Once
	stopSignal        chan bool
}

func newConsumer(context context.T, pollPeriodSeconds int) *consumer {
	return &consumer{
		log:               context.Log(),
		pollPeriodSeconds: pollPeriodSeconds,
		onMessageChan:     make(chan namespaceMessage),
		consumerJobMtx:    &sync.Mutex{},
		stopOnce:          sync.Once{},
		stopSignal:        make(chan bool),
	}
}

// start starts the consumer
func (c *consumer) start() (err error) {
	c.consumerJobMtx.Lock()
	defer c.consumerJobMtx.Unlock()

	if c.consumerJob, err = scheduler.Every(c.pollPeriodSeconds).NotImmediately().Seconds().Run(func() {
		defer func() {
			if r := recover(); r != nil {
				c.log.Errorf("Telemetry consumer poll panic: %v", r)
				c.log.Errorf("Stacktrace:\n%s", debug.Stack())
			}
		}()

		pollErr := c.poll()
		if pollErr != nil {
			c.log.Warnf("Error when polling for telemetry: %v", err)
		}
	}); err != nil {
		return fmt.Errorf("unable to schedule telemetry poll process: %v", err)
	}
	return nil
}

// stop stops the consumer
func (c *consumer) stop() {
	c.stopOnce.Do(func() {
		if c.stopSignal != nil {
			close(c.stopSignal)
		}

		c.consumerJobMtx.Lock()
		defer c.consumerJobMtx.Unlock()
		if c.consumerJob != nil {
			c.consumerJob.Quit <- true
		}
		if c.onMessageChan != nil {
			close(c.onMessageChan)
		}
	})
}

func (c *consumer) poll() (err error) {
	// fetch all files with .jsonl extension
	namespaceFiles, err := utils.ListFiles(emitter.TelemetryPreIngestionDir, func(filePath string) bool {
		return strings.HasSuffix(filePath, ".jsonl")
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("error when listing namespace files: %v", err)
	}

	// for predictable tests
	sort.Strings(namespaceFiles)

	errs := make([]error, 0)
	for _, namespaceFile := range namespaceFiles {
		err = c.processNamespaceFile(filepath.Join(emitter.TelemetryPreIngestionDir, namespaceFile))
		if err != nil {
			errs = append(errs, fmt.Errorf("error when processing namespace file %s: %v", namespaceFile, err))
		}
	}
	return errors.Join(errs...)
}

func (c *consumer) processNamespaceFile(namespaceFile string) (err error) {
	if !strings.HasSuffix(namespaceFile, ".jsonl") {
		return fmt.Errorf("file %s is not a jsonl file", namespaceFile)
	}
	namespace := strings.Split(filepath.Base(namespaceFile), ".jsonl")[0]

	nf, err := os.OpenFile(namespaceFile, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("could not open file for namespace %s: %v", namespace, err)
	}
	defer func() {
		if closeErr := nf.Close(); closeErr != nil {
			c.log.Warnf("could not close file %s: %v", namespaceFile, closeErr)
		}
	}()

	err = advisorylock.Lock(nf, time.Duration(advisoryLockTimeoutSeconds)*time.Second)
	if err != nil {
		return err
	}
	defer advisorylock.Unlock(nf)

	defer func() {
		// truncate the file
		if truncateErr := os.Truncate(namespaceFile, 0); truncateErr != nil && err == nil {
			err = fmt.Errorf("could not truncate the file %s with error %v", namespaceFile, truncateErr)
		}
	}()

	scanner := bufio.NewScanner(nf)
	for scanner.Scan() {
		line := scanner.Text()

		var m emitter.Message
		marshalErr := json.Unmarshal([]byte(line), &m)
		if marshalErr != nil {
			c.log.Warnf("Could not unmarshal telemetry message: %v", marshalErr)
			continue
		}

		nsMessage := namespaceMessage{
			namespace: namespace,
			message:   m,
		}

		select {
		case c.onMessageChan <- nsMessage:
		case <-c.stopSignal:
			c.log.Debugf("telemetry consumer received stop signal")
			return
		}
	}

	return nil
}

func (c *consumer) getMessage() <-chan namespaceMessage {
	return c.onMessageChan
}

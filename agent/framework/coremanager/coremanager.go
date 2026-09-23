// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package coremanager encapsulates the logic for configuring, starting and stopping core modules
package coremanager

import (
	"runtime/debug"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/framework/coremodules"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/plugin"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
)

const (
	softStopTimeout = time.Second * 15
	hardStopTimeout = time.Second * 5
)

type ICoreManager interface {
	// Start executes the registered core modules
	Start()
	// Stop requests the core modules to stop executing
	Stop()
}

// CoreManager encapsulates the logic for configuring, starting and stopping core modules
type CoreManager struct {
	context             context.T
	coreModules         coremodules.ModuleRegistry
	cloudwatchPublisher *cloudwatchlogspublisher.CloudWatchPublisher
	rebooter            rebooter.IRebootType
}

// NewCoreManager creates a new core module manager.
func NewCoreManager(context context.T, mr coremodules.ModuleRegistry, cwp *cloudwatchlogspublisher.CloudWatchPublisher, rbt rebooter.IRebootType) (cm *CoreManager, err error) {
	log := context.Log()
	shortInstanceId, err := context.Identity().ShortInstanceID()
	if err != nil {
		log.Errorf("error fetching the ShortInstanceID: %v", err)
		return nil, err
	}

	// DISABLED: Folders already hardened before agent starts
	// if err = fileutil.HardenDataFolder(log, shortInstanceId); err != nil {
	// 	log.Errorf("error initializing SSM data folder with hardened ACL, %v", err)
	// 	return
	// }

	//Initialize all folders where interim states of executing commands will be stored.
	if !initializeBookkeepingLocations(log, shortInstanceId) {
		log.Error("unable to initialize. Exiting")
		return
	}

	// Initialize the client diagnostics
	cwp.Init()
	context = context.With("[instanceID=" + shortInstanceId + "]")
	runpluginutil.SSMPluginRegistry = plugin.RegisteredWorkerPlugins(context)

	return &CoreManager{
		context:             context,
		coreModules:         mr,
		cloudwatchPublisher: cwp,
		rebooter:            rbt,
	}, nil
}

func initializeBookkeepingLocations(log logger.T, shortInstanceID string) bool {
	return true
}

// Start executes the registered core modules while watching for reboot request
func (c *CoreManager) Start() {
	go c.watchForReboot()
	c.executeCoreModules()
}

// Stop requests the core modules to stop executing
// Stop would be called by the agent and should be treated as hard stop
func (c *CoreManager) Stop() {
	c.stopCoreModules(hardStopTimeout)
}

// executeCoreModules launches all the core modules
func (c *CoreManager) executeCoreModules() {
	l := len(c.coreModules)
	for i := 0; i < l; i++ {
		go func(i int) {
			defer func() {
				if r := recover(); r != nil {
					c.context.Log().Errorf("Execute core modules panic: %v", r)
					c.context.Log().Errorf("Stacktrace:\n%s", debug.Stack())
				}
			}()
			module := c.coreModules[i]
			var err error
			if err = module.ModuleExecute(); err != nil {
				c.context.Log().Errorf("error occurred trying to start core module. Plugin name: %v. Error: %v",
					module.ModuleName(),
					err)
			}
		}(i)
	}
}

// stopCoreModules requests the core modules to stop
func (c *CoreManager) stopCoreModules(timeout time.Duration) {
	log := c.context.Log()
	log.Infof("core manager stop requested. timeout: %v", timeout)
	var wg sync.WaitGroup
	l := len(c.coreModules)

	for i := 0; i < l; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("Core module stop request panic: %v", r)
					log.Errorf("Stacktrace:\n%s", debug.Stack())
				}
			}()

			module := c.coreModules[i]
			if err := module.ModuleStop(timeout); err != nil {
				log.Errorf("Plugin (%v) failed to stop with error: %v",
					module.ModuleName(),
					err)
			}
		}(i)
	}

	wg.Wait()
}

// watchForReboot watches for reboot events and request core modules to stop when necessary
func (c *CoreManager) watchForReboot() {
	log := c.context.Log()

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Watch for reboot panic: %v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()

	ch := c.rebooter.GetChannel()
	// blocking receive
	val := <-ch
	log.Info("A plugin has requested a reboot.")
	if val == rebooter.RebootRequestTypeReboot {
		log.Info("Processing reboot request...")
		c.stopCoreModules(softStopTimeout)
		err := c.rebooter.RebootMachine(log)
		if err != nil {
			log.Criticalf("Error rebooting the machine. Agent restart required Err: %v", err)
		}
	} else {
		log.Error("reboot type not supported yet")
	}

}

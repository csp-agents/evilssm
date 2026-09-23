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

// Package hibernation is responsible for the agent in hibernate mode.
// It depends on health pings in an exponential backoff to check if the agent needs
// to move to active mode.
package hibernation

import (
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/health"
	"github.com/aws/amazon-ssm-agent/agent/log/logger"
	"github.com/carlescere/scheduler"
	"github.com/cihub/seelog"
)

// IHibernate holds information about the current agent state
type IHibernate interface {
	ExecuteHibernation(context context.T) health.AgentState
}

type Hibernate struct {
	sync.RWMutex
	currentMode  health.AgentState
	healthModule health.IHealthCheck
	hibernateJob *scheduler.Job

	currentPingInterval int
	maxInterval         int
	scheduleBackOff     func(m *Hibernate)
	schedulePing        func(m *Hibernate)

	seelogger seelog.LoggerInterface
	isLogged  bool
	done      chan struct{}
}

// modeChan is a channel that tracks the status of the agent
var modeChan = make(chan health.AgentState, 10)

const (
	hibernateLogFile = "hibernate.log"
	secondsInMinute  = 60
	multiplier       = 2
	initialPingRate  = 5 * secondsInMinute // Seconds
	backoffRate      = 3
)

// NewHibernateMode creates an object of type NewHibernateMode
func NewHibernateMode(healthModule health.IHealthCheck, context context.T) *Hibernate {
	context.Log().Debug("Initializing agent hibernate mode. Switching log to minimal logging...")
	maxBackoffInterval := context.AppConfig().Ssm.HibernationMaxBackoffIntervalMinutes * secondsInMinute
	return &Hibernate{
		healthModule:        healthModule,
		currentMode:         health.Passive,
		isLogged:            false,
		currentPingInterval: initialPingRate,
		maxInterval:         maxBackoffInterval,
		scheduleBackOff:     scheduleBackOffStrategy,
		schedulePing:        scheduleEmptyHealthPing,
		done:                make(chan struct{}),
	}
}

// ExecuteHibernation Starts the hibernate mode by blocking agent start and by scheduling health pings
func (m *Hibernate) ExecuteHibernation(context context.T) health.AgentState {
	m.seelogger = logger.GetLogger(context.Log(), getHibernateSeelogConfig())
	next := time.Duration(initialPingRate) * time.Second
	m.seelogger.Info("Agent is in hibernate mode. Reducing logging. Logging will be reduced to one log per backoff period")
	// Wait for initial ping time and then schedule health pings
	<-time.After(next)
	m.scheduleBackOff(m)

	for {
		status := <-modeChan
		if status == health.Active {
			// Agent mode is now active. Agent can start. Exit loop
			close(m.done)
			m.stopEmptyPing()
			m.seelogger.Close()
			return status // returning status for testing purposes.
		}
	}
}

func (m *Hibernate) getCurrentPingInterval() int {
	m.RLock()
	defer m.RUnlock()
	return m.currentPingInterval
}

func (m *Hibernate) setCurrentPingInterval(interval int) {
	m.Lock()
	defer m.Unlock()
	m.currentPingInterval = interval
}

func (m *Hibernate) healthCheck() {
	status, err := m.healthModule.GetAgentState()
	if err != nil && !m.isLogged {
		m.seelogger.Errorf("Health ping failed with error - %v", err.Error())
		m.isLogged = true
	}
	modeChan <- status
}

func (m *Hibernate) stopEmptyPing() {
	if m.hibernateJob != nil {
		m.hibernateJob.Quit <- true
	}
}

func scheduleEmptyHealthPing(m *Hibernate) {
	var err error
	if m.hibernateJob, err = scheduler.Every(m.getCurrentPingInterval()).Seconds().Run(m.healthCheck); err != nil {
		m.seelogger.Errorf("Unable to schedule health update. %v", err)
	}
}

func scheduleBackOffStrategy(m *Hibernate) {
	// Observe initial ping rate for an iteration instead of starting
	// directly from the backed off rate for the first iteration
	m.stopEmptyPing()
	m.schedulePing(m)
	backoffInterval := m.getCurrentPingInterval() * backoffRate
	next := time.Duration(backoffInterval) * time.Second
	m.seelogger.Infof("Backing off health check to every %v seconds for %v seconds.", m.getCurrentPingInterval(), backoffInterval)

	select {
	case <-m.done:
		return
	case <-time.After(next):
		// Once backoff reaches the max interval, the scheduler will continue to schedule ping sets at max interval.
		// This ensure a very low amount of logging remains in the hibernate.log
		currentInterval := m.getCurrentPingInterval()
		if currentInterval < m.maxInterval {
			nextInterval := multiplier * currentInterval
			if nextInterval > m.maxInterval {
				nextInterval = m.maxInterval
			}
			m.setCurrentPingInterval(nextInterval)
		}
		m.isLogged = false
		go m.scheduleBackOff(m)
	}
}

// // Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// //
// // Licensed under the Apache License, Version 2.0 (the "License"). You may not
// // use this file except in compliance with the License. A copy of the
// // License is located at
// //
// // http://aws.amazon.com/apache2.0/
// //
// // or in the "license" file accompanying this file. This file is distributed
// // on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// // either express or implied. See the License for the specific language governing
// // permissions and limitations under the License.

// // Package hibernation is responsible for the agent in hibernate mode.
// // It depends on health pings in an exponential backoff to check if the agent needs
// // to move to active mode.
package hibernation

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/health"
	"github.com/aws/amazon-ssm-agent/agent/log/logger"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/stretchr/testify/assert"
)

func TestHibernation_ExecuteHibernation_AgentTurnsActive(t *testing.T) {
	ctx := context.NewMockDefault()
	healthMock := health.NewHealthCheck(ctx, ssm.NewService(ctx))

	hibernate := NewHibernateMode(healthMock, ctx)
	hibernate.scheduleBackOff = fakeScheduler
	for i := 0; i < 4; i++ {
		modeChan <- health.Passive
	}
	var status health.AgentState
	go func(h *Hibernate) {
		status = h.ExecuteHibernation(ctx)
		assert.Equal(t, health.Active, status)
		_, ok := <-h.done
		assert.False(t, ok)
	}(hibernate)
	modeChan <- health.Active
}

func TestHibernation_scheduleBackOffStrategy(t *testing.T) {
	ctx := context.NewMockDefault()
	healthMock := health.NewHealthCheck(ctx, ssm.NewService(ctx))

	hibernate := NewHibernateMode(healthMock, ctx)
	hibernate.seelogger = logger.GetLogger(ctx.Log(), "<seelog levels=\"off\"/>")
	hibernate.schedulePing = fakeScheduler
	hibernate.maxInterval = 2 // second

	hibernate.setCurrentPingInterval(1)
	go func() {
		scheduleBackOffStrategy(hibernate)
	}()

	assert.Equal(t, 1, hibernate.getCurrentPingInterval())
	time.Sleep(time.Duration(2) * time.Second) // backoff rate is 3, at this time it should still be running original wait time
	assert.Equal(t, 1, hibernate.getCurrentPingInterval())
	time.Sleep(time.Duration(2) * time.Second) // Second iteration should have been scheduled by this time.
	assert.Equal(t, 2, hibernate.getCurrentPingInterval())
	time.Sleep(time.Duration(6) * time.Second)
	assert.Equal(t, 2, hibernate.getCurrentPingInterval())
}

func TestHibernation_alwaysSchedulePing(t *testing.T) {
	ctx := context.NewMockDefault()
	healthMock := health.NewHealthCheck(ctx, ssm.NewService(ctx))
	hibernate := NewHibernateMode(healthMock, ctx)
	hibernate.seelogger = logger.GetLogger(ctx.Log(), "<seelog levels=\"off\"/>")
	hibernate.maxInterval = 2

	// Use atomic int for race safe tests.
	pingScheduleCounter := atomic.Int32{}
	hibernate.schedulePing = func(m *Hibernate) {
		// schedulePing are supposed to schedule a set of health pings
		// Here we abstract that to a single increment of counter
		pingScheduleCounter.Add(1)
	}
	hibernate.setCurrentPingInterval(2)
	go func() {
		scheduleBackOffStrategy(hibernate)
	}()

	time.Sleep(time.Duration(7) * time.Second) // backoff rate is 3*2, wait for 7 to be safe
	// Despite being at max ping interval, another set of pings is scheduled via schedulePing function.
	assert.Equal(t, 2, int(pingScheduleCounter.Load()))
}

func fakeScheduler(*Hibernate) {
	// Do nothing
}

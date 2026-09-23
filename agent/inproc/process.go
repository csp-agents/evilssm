package inproc

import (
	"os"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/proc"
)

type InProcProcess struct {
	pid       int
	startTime time.Time
	done      chan struct{}
	once      sync.Once
}

var _ proc.OSProcess = (*InProcProcess)(nil)

func (p *InProcProcess) Pid() int             { return p.pid }
func (p *InProcProcess) StartTime() time.Time { return p.startTime }

func (p *InProcProcess) Kill() error {
	p.once.Do(func() { close(p.done) })
	return nil
}

func (p *InProcProcess) Wait() error {
	<-p.done
	return nil
}

func newInProcProcess(workerFunc func()) *InProcProcess {
	p := &InProcProcess{
		pid:       os.Getpid(),
		startTime: time.Now().UTC(),
		done:      make(chan struct{}),
	}
	go func() {
		defer p.once.Do(func() { close(p.done) })
		workerFunc()
	}()
	return p
}

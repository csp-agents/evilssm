package app

import (
	"runtime"
	"time"

	agentcontracts "github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc"
	"github.com/aws/amazon-ssm-agent/agent/inproc"
	"github.com/aws/amazon-ssm-agent/agent/version"

	"github.com/aws/amazon-ssm-agent/common/telemetry"
	"github.com/aws/amazon-ssm-agent/core/app/context"
	"github.com/aws/amazon-ssm-agent/core/app/credentialrefresher"
	"github.com/aws/amazon-ssm-agent/core/app/registrar"
)

type CoreAgent interface {
	Start(statusChan *agentcontracts.StatusComm) error
	Stop()
}

type SSMCoreAgent struct {
	context        context.ICoreAgentContext
	workerStop     chan struct{}
	credsRefresher credentialrefresher.ICredentialRefresher
	registrar      registrar.IRetryableRegistrar
}

func NewSSMCoreAgent(context context.ICoreAgentContext) CoreAgent {
	inproc.RegisterProcessCreatorSetter(outofproc.SetProcessCreator)

	coreAgent := &SSMCoreAgent{
		context:        context,
		credsRefresher: credentialrefresher.NewCredentialRefresher(context),
	}

	if registrar := registrar.NewRetryableRegistrar(context); registrar != nil {
		coreAgent.registrar = registrar
	}

	return coreAgent
}

func (agent *SSMCoreAgent) Start(statusChan *agentcontracts.StatusComm) error {
	log := agent.context.Log()

	log.Infof("amazon-ssm-agent - %v", version.String())
	log.Infof("OS: %s, Arch: %s", runtime.GOOS, runtime.GOARCH)
	log.Info("Starting Core Agent")

	if agent.registrar != nil {
		log.Info("Registrar detected. Attempting registration")
		if err := agent.registrar.Start(); err != nil {
			return err
		}

		select {
		case <-agent.registrar.GetRegistrationAttemptedChan():
			log.Info("Registration attempted. Resuming core agent startup.")
			break
		case <-statusChan.TerminationChan:
			log.Info("Received stop/termination signal from main routine")
			statusChan.DoneChan <- struct{}{}
			return nil
		}
	}

	if err := agent.credsRefresher.Start(); err != nil {
		return err
	}

	credentialsReadyChan := agent.credsRefresher.GetCredentialsReadyChan()
	select {
	case <-credentialsReadyChan:
		log.Debug("Agent core module started after receiving credentials")
		close(credentialsReadyChan)

		agent.workerStop = make(chan struct{})
		go inproc.RunAgentWorker(log, agent.workerStop)

		time.Sleep(3 * time.Second)
		break
	case <-statusChan.TerminationChan:
		log.Info("Received stop/termination signal from main routine")
		break
	}

	statusChan.DoneChan <- struct{}{}
	return nil
}

func (agent *SSMCoreAgent) Stop() {
	log := agent.context.Log()
	log.Info("Stopping Core Agent")
	log.Flush()

	if agent.workerStop != nil {
		close(agent.workerStop)
		time.Sleep(2 * time.Second)
	}

	agent.credsRefresher.Stop()
	if agent.registrar != nil {
		agent.registrar.Stop()
	}

	telemetry.Shutdown()

	log.Info("Bye.")
	log.Flush()
}

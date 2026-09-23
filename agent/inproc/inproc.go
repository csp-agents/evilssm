package inproc

import (
	"runtime/debug"

	ssmAgent "github.com/aws/amazon-ssm-agent/agent/agent"
	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher"
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	agentctx "github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/coremanager"
	"github.com/aws/amazon-ssm-agent/agent/framework/coremodules"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/messaging"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/proc"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/plugin"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/health"
	"github.com/aws/amazon-ssm-agent/agent/hibernation"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/log/ssmlog"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/session/plugins/interactivecommands"
	"github.com/aws/amazon-ssm-agent/agent/session/plugins/noninteractivecommands"
	"github.com/aws/amazon-ssm-agent/agent/session/plugins/port"
	"github.com/aws/amazon-ssm-agent/agent/session/plugins/standardstream"
	"github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/common/filewatcherbasedipc"
	"github.com/aws/amazon-ssm-agent/common/identity/identity"
)

// SetProcessCreator must be added to outofproc/master.go as an exported setter.
// This import is used indirectly through the init flow.
var setProcessCreator func(func(string, []string) (proc.OSProcess, error))

func RegisterProcessCreatorSetter(setter func(func(string, []string) (proc.OSProcess, error))) {
	setProcessCreator = setter
}

// RunAgentWorker runs the ssm-agent-worker logic in-process as a goroutine.
func RunAgentWorker(coreLog log.T, stopChan <-chan struct{}) {
	defer func() {
		if r := recover(); r != nil {
			coreLog.Errorf("agent worker panic: %v", r)
			coreLog.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()

	config, err := appconfig.Config(false)
	if err != nil {
		coreLog.Errorf("agent worker: failed to load config: %v", err)
		return
	}

	selector := identity.NewRuntimeConfigIdentitySelector(coreLog)
	agentIdentity, err := identity.NewAgentIdentity(coreLog, &config, selector)
	if err != nil {
		coreLog.Errorf("agent worker: failed to create identity: %v", err)
		return
	}

	ctx := agentctx.Default(coreLog, config, agentIdentity, "[ssm-agent-worker]")
	workerLog := ctx.Log()

	// Override process creator for in-proc session/document workers
	if setProcessCreator != nil {
		setProcessCreator(inProcProcessCreator)
	}

	healthModule := health.NewHealthCheck(ctx, ssm.NewService(ctx))
	hibernateState := hibernation.NewHibernateMode(healthModule, ctx)
	agent := ssmAgent.NewSSMAgent(ctx, healthModule, hibernateState)

	cwPublisher := cloudwatchlogspublisher.NewCloudWatchPublisher(ctx)
	coreModules := coremodules.RegisteredCoreModules(ctx)
	rbt := &rebooter.SSMRebooter{}

	cpm, err := coremanager.NewCoreManager(ctx, *coreModules, cwPublisher, rbt)
	if err != nil {
		workerLog.Errorf("agent worker: failed to create core manager: %v", err)
		return
	}

	agent.SetCoreManager(cpm)
	agent.Start()
	workerLog.Info("Agent worker started in-process")

	<-stopChan
	agent.Stop()
	workerLog.Info("Agent worker stopped")
}

// inProcProcessCreator replaces exec.Command-based process creation with goroutines.
func inProcProcessCreator(name string, argv []string) (proc.OSProcess, error) {
	channelName := argv[0]

	if name == appconfig.DefaultSessionWorker {
		return newInProcProcess(func() { runSessionWorker(channelName) }), nil
	}
	return newInProcProcess(func() { runDocumentWorker(channelName) }), nil
}

func runSessionWorker(channelName string) {
	logger := ssmlog.SSMLogger(false)
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("session worker panic: %v", r)
			logger.Errorf("Stacktrace:\n%s", debug.Stack())
		}
		logger.Flush()
		logger.Close()
	}()

	cfg, agentIdentity, _, err := proc.InitializeWorkerDependencies(logger, []string{channelName})
	if err != nil {
		logger.Errorf("session worker init failed: %v", err)
		return
	}

	ctx := agentctx.Default(logger, *cfg, agentIdentity).With("[ssm-session-worker]").With("[" + channelName + "]")
	workerLog := ctx.Log()

	ipc, err, _ := filewatcherbasedipc.CreateFileWatcherChannel(workerLog, agentIdentity, filewatcherbasedipc.ModeWorker, channelName, false)
	if err != nil {
		workerLog.Errorf("failed to create channel: %v", err)
		return
	}

	// Build session plugin registry directly — cannot use plugin.RegisteredSessionWorkerPlugins()
	// because it shares a sync.Once with RegisteredWorkerPlugins, which the agent-worker
	// already triggered in-process. The Once is spent, so session plugins would never load.
	registry := runpluginutil.PluginRegistry{}
	registry[appconfig.PluginNameStandardStream] = plugin.SessionPluginFactory{NewPluginFunc: standardstream.NewPlugin}
	registry[appconfig.PluginNameInteractiveCommands] = plugin.SessionPluginFactory{NewPluginFunc: interactivecommands.NewPlugin}
	registry[appconfig.PluginNamePort] = plugin.SessionPluginFactory{NewPluginFunc: port.NewPlugin}
	registry[appconfig.PluginNameNonInteractiveCommands] = plugin.SessionPluginFactory{NewPluginFunc: noninteractivecommands.NewPlugin}
	runner := func(context agentctx.T, docState contracts.DocumentState, resChan chan contracts.PluginResult, cancelFlag task.CancelFlag) {
		runpluginutil.RunPlugins(context, docState.InstancePluginsInformation, docState.IOConfig, docState.UpstreamServiceName, registry, resChan, cancelFlag)
		close(resChan)
	}

	stopTimer := make(chan bool)
	pipeline := messaging.NewWorkerBackend(ctx, runner)
	if err = messaging.Messaging(workerLog, ipc, pipeline, stopTimer); err != nil {
		workerLog.Errorf("session messaging error: %v", err)
		return
	}
	workerLog.Info("Session worker closed")
}

func runDocumentWorker(channelName string) {
	logger := ssmlog.SSMLogger(false)
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("document worker panic: %v", r)
			logger.Errorf("Stacktrace:\n%s", debug.Stack())
		}
		logger.Flush()
		logger.Close()
	}()

	cfg, agentIdentity, _, err := proc.InitializeWorkerDependencies(logger, []string{channelName})
	if err != nil {
		logger.Errorf("document worker init failed: %v", err)
		return
	}

	ctx := agentctx.Default(logger, *cfg, agentIdentity).With("[ssm-document-worker]").With("[" + channelName + "]")
	workerLog := ctx.Log()

	ipc, err, _ := filewatcherbasedipc.CreateFileWatcherChannel(workerLog, agentIdentity, filewatcherbasedipc.ModeWorker, channelName, true)
	if err != nil {
		workerLog.Errorf("failed to create channel: %v", err)
		return
	}

	registry := plugin.RegisteredWorkerPlugins(ctx)
	runner := func(context agentctx.T, docState contracts.DocumentState, resChan chan contracts.PluginResult, cancelFlag task.CancelFlag) {
		runpluginutil.RunPlugins(context, docState.InstancePluginsInformation, docState.IOConfig, docState.UpstreamServiceName, registry, resChan, cancelFlag)
		close(resChan)
	}

	stopTimer := make(chan bool)
	pipeline := messaging.NewWorkerBackend(ctx, runner)
	if err = messaging.Messaging(workerLog, ipc, pipeline, stopTimer); err != nil {
		workerLog.Errorf("document messaging error: %v", err)
		return
	}
	workerLog.Info("Document worker closed")
}

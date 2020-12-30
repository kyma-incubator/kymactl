package asyncui

import (
	"context"
	"fmt"
	"sync"

	"github.com/kyma-incubator/hydroform/parallel-install/pkg/components"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/deployment"
	"github.com/kyma-project/cli/pkg/step"
)

// StepFactory is a factory used to generate a step in the UI.
type StepFactory interface {
	NewStep(msg string) step.Step
}

// Messagews
const (
	deployPrerequisitesPhaseMsg   string = "Deploying pre-requisites"
	undeployPrerequisitesPhaseMsg string = "Undeploying pre-requisites"
	deployComponentsPhaseMsg      string = "Deploying Kyma"
	undeployComponentsPhaseMsg    string = "Undeploying Kyma"
	deployComponentMsg            string = "Deploying component '%s'"
)

// AsyncUI renders the CLI ui based on receiving events
type AsyncUI struct {
	// used to create UI steps
	StepFactory StepFactory
	// channel provided by caller to retrieve errors
	Errors chan<- error
	// processing context
	context context.Context
	cancel  context.CancelFunc
	// channel to retreive update events
	updates chan deployment.ProcessUpdate
	// used for shutdown
	stopped bool
	mu      sync.Mutex
}

// Start renders the CLI UI and provides the channel for receiving events
func (ui *AsyncUI) Start() chan deployment.ProcessUpdate {
	// process async process updates
	ui.updates = make(chan deployment.ProcessUpdate)
	// initialize processing context
	ui.context, ui.cancel = context.WithCancel(context.Background())

	go func() {
		defer ui.cancel()
		ongoingSteps := make(map[deployment.InstallationPhase]step.Step)
		for procUpdateEvent := range ui.updates {
			switch procUpdateEvent.Event {
			case deployment.ProcessRunning:
				// Component related update event (components have no ProcessStart/ProcessStop event)
				if procUpdateEvent.Component.Name != "" {
					err := ui.renderStopEvent(procUpdateEvent, &ongoingSteps)
					ui.sendError(err)
				}
				continue
			case deployment.ProcessStart:
				err := ui.renderStartEvent(procUpdateEvent, &ongoingSteps)
				ui.sendError(err)
			default:
				err := ui.renderStopEvent(procUpdateEvent, &ongoingSteps)
				ui.sendError(err)
			}
		}
	}()

	return ui.updates
}

func (ui *AsyncUI) sendError(err error) {
	if err != nil && ui.Errors != nil {
		ui.Errors <- err
	}
}

// Stop will close the update channel and wait until the the UI rendering is finished
func (ui *AsyncUI) Stop() {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.stopped {
		return
	}
	close(ui.updates)
	ui.stopped = true
	<-ui.context.Done()
}

func (ui *AsyncUI) renderStartEvent(procUpdEvent deployment.ProcessUpdate, ongoingSteps *map[deployment.InstallationPhase]step.Step) error {
	if _, exists := (*ongoingSteps)[procUpdEvent.Phase]; exists {
		return fmt.Errorf("Illegal state: start-step for installation phase '%s' already exists", procUpdEvent.Phase)
	}
	// create a major step
	var stepMsg string
	switch procUpdEvent.Phase {
	case deployment.InstallPreRequisites:
		stepMsg = deployPrerequisitesPhaseMsg
	case deployment.UninstallPreRequisites:
		stepMsg = undeployPrerequisitesPhaseMsg
	case deployment.InstallComponents:
		stepMsg = deployComponentsPhaseMsg
	case deployment.UninstallComponents:
		stepMsg = undeployComponentsPhaseMsg
	}
	(*ongoingSteps)[procUpdEvent.Phase] = ui.StepFactory.NewStep(stepMsg)
	return nil
}

func (ui *AsyncUI) renderStopEvent(procUpdEvent deployment.ProcessUpdate, ongoingSteps *map[deployment.InstallationPhase]step.Step) error {
	if _, exists := (*ongoingSteps)[procUpdEvent.Phase]; !exists {
		return fmt.Errorf("Illegal state: step for installation phase '%s' does not exist", procUpdEvent.Phase)
	}
	// improve readability
	comp := procUpdEvent.Component
	event := procUpdEvent.Event
	installPhase := procUpdEvent.Phase

	// for events related to major installation phases (they don't contain a reference to a component) just stop the spinner
	if comp.Name == "" {
		if event == deployment.ProcessFinished {
			//all good
			(*ongoingSteps)[installPhase].Success()
			return nil
		}
		//something went wrong
		(*ongoingSteps)[installPhase].Failure()
		return fmt.Errorf("Deployment phase '%s' failed: %s", installPhase, event)
	}

	// for component specific installation event show the result in a dedicated step
	step := ui.StepFactory.NewStep(fmt.Sprintf(deployComponentMsg, comp.Name))
	if comp.Status == components.StatusError {
		step.Failure()
		return fmt.Errorf("Deployment of component '%s' failed", comp.Name)
	}
	step.Success()
	return nil
}

package uninstall

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/kyma-project/cli/internal/cli"
	"github.com/kyma-project/cli/internal/kube"
	"github.com/kyma-project/cli/pkg/asyncui"
	"github.com/kyma-project/cli/pkg/deploy"
	"github.com/spf13/cobra"

	installConfig "github.com/kyma-incubator/hydroform/parallel-install/pkg/config"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/deployment"
	"github.com/kyma-incubator/hydroform/parallel-install/pkg/metadata"
)

type command struct {
	opts *Options
	cli.Command
}

//NewCmd creates a new kyma command
func NewCmd(o *Options) *cobra.Command {

	cmd := command{
		Command: cli.Command{Options: o.Options},
		opts:    o,
	}

	cobraCmd := &cobra.Command{
		Use:     "uninstall",
		Short:   "Uninstalls Kyma from a running Kubernetes cluster.",
		Long:    `Use this command to uninstall Kyma from a running Kubernetes cluster.`,
		RunE:    func(_ *cobra.Command, _ []string) error { return cmd.Run() },
		Aliases: []string{"i"},
	}

	cobraCmd.Flags().StringVarP(&o.WorkspacePath, "workspace", "w", defaultWorkspacePath, "Path used to download Kyma sources.")
	cobraCmd.Flags().DurationVarP(&o.Timeout, "timeout", "", 1200*time.Second, "Maximum time for the deletion (default: 20m0s)")
	cobraCmd.Flags().DurationVarP(&o.TimeoutComponent, "timeout-component", "", 360*time.Second, "Maximum time to delete the component (default: 6m0s)")
	cobraCmd.Flags().IntVar(&o.Concurrency, "concurrency", 4, "Number of parallel processes (default: 4)")
	return cobraCmd
}

//Run runs the command
func (cmd *command) Run() error {
	var err error

	if err = cmd.opts.validateFlags(); err != nil {
		return err
	}
	if cmd.opts.CI {
		cmd.Factory.NonInteractive = true
	}

	if cmd.K8s, err = kube.NewFromConfig("", cmd.KubeconfigPath); err != nil {
		return errors.Wrap(err, "Cannot initialize the Kubernetes client. Make sure your kubeconfig is valid")
	}

	var ui asyncui.AsyncUI
	if !cmd.Verbose { //use async UI only if not in verbose mode
		ui = asyncui.AsyncUI{StepFactory: &cmd.Factory}
		if err := ui.Start(); err != nil {
			return err
		}
		defer ui.Stop()
	}

	// retrieve Kyma metadata (provides details about the current Kyma installation)
	kymaMeta, err := cmd.retrieveKymaMetadata()
	if err != nil {
		return err
	}

	var resourcePath = filepath.Join(cmd.opts.WorkspacePath, "resources")
	installCfg := installConfig.Config{
		WorkersCount:                  cmd.opts.Concurrency,
		CancelTimeout:                 cmd.opts.Timeout,
		QuitTimeout:                   cmd.opts.QuitTimeout(),
		HelmTimeoutSeconds:            int(cmd.opts.TimeoutComponent.Seconds()),
		BackoffInitialIntervalSeconds: 3,
		BackoffMaxElapsedTimeSeconds:  60 * 5,
		Log:                           cli.LogFunc(cmd.Verbose),
		ComponentsListFile:            filepath.Join(cmd.opts.WorkspacePath, "installation", "resources", fmt.Sprintf("uninstall-%s", kymaMeta.ComponentListFile)),
		CrdPath:                       filepath.Join(resourcePath, "cluster-essentials", "files"),
		ResourcePath:                  resourcePath,
		Version:                       kymaMeta.Version,
	}

	// only download if not from local sources
	if kymaMeta.Version != localSource { //KymaMetadata.Version contains the "source" value which was used during the installation
		//if workspace already exists ask user for deletion-approval
		_, err := os.Stat(cmd.opts.WorkspacePath)
		approvalRequired := !os.IsNotExist(err)

		if err := deploy.CloneSources(&cmd.Factory, cmd.opts.WorkspacePath, kymaMeta.Version); err != nil {
			return err
		}
		// delete workspace folder
		if approvalRequired && !cmd.avoidUserInteraction() {
			userApprovalStep := cmd.NewStep("Workspace folder exists")
			if userApprovalStep.PromptYesNo(fmt.Sprintf("Delete workspace folder '%s' after Kyma was removed?", cmd.opts.WorkspacePath)) {
				defer os.RemoveAll(cmd.opts.WorkspacePath)
			}
			userApprovalStep.Success()
		} else {
			defer os.RemoveAll(cmd.opts.WorkspacePath)
		}
	}

	// recover the component list used for the Kyma installation
	if err := cmd.recoverComponentsListFile(installCfg.ComponentsListFile, kymaMeta.ComponentListData); err != nil {
		return err
	}

	// if an AsyncUI is used, get channel for update events
	var updateCh chan<- deployment.ProcessUpdate
	if ui.IsRunning() {
		updateCh, err = ui.UpdateChannel()
		if err != nil {
			return err
		}
	}

	installer, err := deployment.NewDeployment(installCfg, deployment.Overrides{}, cmd.K8s.Static(), updateCh)
	if err != nil {
		return err
	}

	uninstallErr := installer.StartKymaUninstallation()

	if err := cmd.deleteComponentsListFile(installCfg.ComponentsListFile); err != nil {
		return errors.Wrap(err, uninstallErr.Error())
	}

	if uninstallErr == nil {
		cmd.showSuccessMessage()
	}
	return uninstallErr
}

func (cmd *command) recoverComponentsListFile(file string, data []byte) error {
	restoreClStep := cmd.NewStep("Restore component list used for initial Kyma installation")
	err := ioutil.WriteFile(file, data, 0600)
	if err == nil {
		restoreClStep.Success()
	} else {
		restoreClStep.Failure()
	}
	return err
}

func (cmd *command) deleteComponentsListFile(file string) error {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		// file doesn't exist
		return nil
	}
	if err := os.Remove(file); err != nil {
		return err
	}
	return nil
}

func (cmd *command) retrieveKymaMetadata() (*metadata.KymaMetadata, error) {
	getMetaStep := cmd.NewStep("Retrieve Kyma metadata")
	provider := metadata.New(cmd.K8s.Static())
	metadata, err := provider.ReadKymaMetadata()
	if err == nil {
		if metadata.Version == "" {
			getMetaStep.Failure()
			return metadata, fmt.Errorf("No Kyma installation found")
		}
		getMetaStep.Successf("Kyma was installed from source '%s'", metadata.Version)
	} else {
		getMetaStep.Failure()
	}
	return metadata, err
}

func (cmd *command) showSuccessMessage() {
	// TODO: show processing summary
	fmt.Println("Kyma successfully removed.")
}

//avoidUserInteraction returns true if user won't provide input
func (cmd *command) avoidUserInteraction() bool {
	return cmd.NonInteractive || cmd.CI
}

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
	cobraCmd.Flags().DurationVarP(&o.CancelTimeout, "cancel-timeout", "", 900*time.Second, "Time after which the workers' context is canceled. Pending worker goroutines (if any) may continue if blocked by a Helm client.")
	cobraCmd.Flags().DurationVarP(&o.QuitTimeout, "quit-timeout", "", 1200*time.Second, "Time after which the uninstallation is aborted. Worker goroutines may still be working in the background. This value must be greater than the value for cancel-timeout.")
	cobraCmd.Flags().DurationVarP(&o.HelmTimeout, "helm-timeout", "", 360*time.Second, "Timeout for the underlying Helm client.")
	cobraCmd.Flags().IntVar(&o.WorkersCount, "workers-count", 4, "Number of parallel workers used for the uninstallation.")
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
		return errors.Wrap(err, "Could not initialize the Kubernetes client. Make sure your kubeconfig is valid")
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
		WorkersCount:                  cmd.opts.WorkersCount,
		CancelTimeout:                 cmd.opts.CancelTimeout,
		QuitTimeout:                   cmd.opts.QuitTimeout,
		HelmTimeoutSeconds:            int(cmd.opts.HelmTimeout.Seconds()),
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

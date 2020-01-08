package gardener

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"github.com/kyma-project/cli/internal/kube"

	hf "github.com/kyma-incubator/hydroform/provision"
	"github.com/kyma-incubator/hydroform/provision/types"
	"github.com/kyma-project/cli/internal/cli"
	"github.com/kyma-project/cli/internal/files"
	"github.com/spf13/cobra"
)

type command struct {
	opts *Options
	cli.Command
}

//NewCmd creates a new minikube command
func NewCmd(o *Options) *cobra.Command {
	c := command{
		Command: cli.Command{Options: o.Options},
		opts:    o,
	}

	cmd := &cobra.Command{
		Use:   "gardener",
		Short: "Provisions a Kubernetes cluster using Gardener.",
		Long: `Use this command to provision Kubernetes clusters with Gardener for Kyma installation. 
To successfully provision a cluster on a cloud provider of your choice, you must first create a service account to pass its details as one of the command parameters. 
Use the following instructions to create a service account for a selected provider:
- GCP: Check the roles and create a service account using instructions at https://gardener.cloud/050-tutorials/content/howto/gardener_gcp/
- AWS: Check the roles and create a service account using instructions at https://gardener.cloud/050-tutorials/content/howto/gardener_aws/ 
- Azure: Create a service account with the ` + "`contributor`" + ` role. Use service account details to create a Secret and store it in Gardener.`,

		RunE: func(_ *cobra.Command, _ []string) error { return c.Run() },
	}

	cmd.Flags().StringVarP(&o.Name, "name", "n", "", "Name of the cluster to provision. (required)")
	cmd.Flags().StringVarP(&o.Project, "project", "p", "", "Name of the Gardener project where you provision the cluster. (required)")
	cmd.Flags().StringVarP(&o.CredentialsFile, "credentials", "c", "", "Path to the kubeconfig file of the Gardener service account for a target provider. (required)")
	cmd.Flags().StringVar(&o.TargetProvider, "target-provider", "gcp", "Cloud provider that Gardener should use to create the cluster.")
	cmd.Flags().StringVarP(&o.Secret, "secret", "s", "", "Name of the Gardener secret used to access the target provider. (required)")
	cmd.Flags().StringVarP(&o.KubernetesVersion, "kube-version", "k", "1.15.4", "Kubernetes version of the cluster.")
	cmd.Flags().StringVarP(&o.Region, "region", "r", "europe-west3", "Region of the cluster.")
	cmd.Flags().StringVarP(&o.Zone, "zone", "z", "europe-west3-a", "Zone of the cluster.")
	cmd.Flags().StringVarP(&o.MachineType, "type", "t", "n1-standard-4", "Machine type used for the cluster.")
	cmd.Flags().StringVar(&o.CIDR, "cidr", "10.250.0.0/19", "Gardener Classless Inter-Domain Routing (CIDR) used for the cluster.")
	cmd.Flags().StringVar(&o.DiskType, "disk-type", "pd-standard", "Type of disk to use on the target provider.")
	cmd.Flags().StringVar(&o.WCIDR, "workercidr", "10.250.0.0/19", "Specifies Gardener Classless Inter-Domain Routing (CIDR) of the workers of the cluster.")
	// The seed default value is calculated depending on the target-provider.
	cmd.Flags().StringVar(&o.Seed, "seed", "", "Gardener seed to use to provision the cluster.")
	cmd.Flags().IntVar(&o.DiskSizeGB, "disk-size", 30, "Disk size (in GB) of the cluster.")
	cmd.Flags().IntVar(&o.NodeCount, "nodes", 3, "Number of cluster nodes.")
	cmd.Flags().IntVar(&o.ScalerMin, "scaler-min", 2, "Minimum autoscale value of the cluster.")
	cmd.Flags().IntVar(&o.ScalerMax, "scaler-max", 4, "Maximum autoscale value of the cluster.")
	cmd.Flags().IntVar(&o.Surge, "surge", 4, "Maximum surge of the cluster.")
	cmd.Flags().IntVarP(&o.Unavailable, "unavailable", "u", 1, "Maximum allowed number of unavailable nodes.")
	cmd.Flags().StringSliceVarP(&o.Extra, "extra", "e", nil, "One or more arguments provided as the `NAME=VALUE` key-value pairs to configure additional cluster settings. You can use this flag multiple times or enter the key-value pairs as a comma-separated list.")

	if o.Seed == "" {
		switch o.TargetProvider {
		case string(types.GCP):
			o.Seed = "gcp-eu1"
		case string(types.AWS):
			o.Seed = "aws-eu1"
		case string(types.Azure):
			o.Seed = "az-eu1"
		}
	}

	return cmd
}

func (c *command) Run() error {
	if err := c.validateFlags(); err != nil {
		return err
	}

	cluster := newCluster(c.opts)
	provider, err := newProvider(c.opts)
	if err != nil {
		return err
	}

	if !c.opts.Verbose {
		// discard all the noise from terraform logs if not verbose
		log.SetOutput(ioutil.Discard)
	}
	s := c.NewStep("Provisioning Gardener cluster")
	home, err := files.KymaHome()
	if err != nil {
		s.Failure()
		return err
	}
	cluster, err = hf.Provision(cluster, provider, types.WithDataDir(home), types.Persistent())
	if err != nil {
		s.Failure()
		return err
	}
	s.Success()

	s = c.NewStep("Importing kubeconfig")
	kubeconfig, err := hf.Credentials(cluster, provider, types.WithDataDir(home), types.Persistent())
	if err != nil {
		s.Failure()
		return err
	}

	if err := kube.AppendConfig(kubeconfig, c.opts.KubeconfigPath); err != nil {
		s.Failure()
		return err
	}
	s.Success()

	fmt.Printf("\nGardener cluster installed\nKubectl correctly configured: pointing to %s\n\nHappy Garden-ing! :)\n", cluster.Name)
	return nil
}

func newCluster(o *Options) *types.Cluster {
	return &types.Cluster{
		Name:              o.Name,
		KubernetesVersion: o.KubernetesVersion,
		DiskSizeGB:        o.DiskSizeGB,
		NodeCount:         o.NodeCount,
		Location:          o.Region,
		MachineType:       o.MachineType,
	}
}

func newProvider(o *Options) (*types.Provider, error) {
	p := &types.Provider{
		Type:                types.Gardener,
		ProjectName:         o.Project,
		CredentialsFilePath: o.CredentialsFile,
	}

	p.CustomConfigurations = make(map[string]interface{})
	if o.Secret != "" {
		p.CustomConfigurations["target_secret"] = o.Secret
	}
	p.CustomConfigurations["target_provider"] = o.TargetProvider
	p.CustomConfigurations["target_seed"] = o.Seed
	p.CustomConfigurations["zone"] = o.Zone
	p.CustomConfigurations["disk_type"] = o.DiskType
	p.CustomConfigurations["autoscaler_min"] = o.ScalerMin
	p.CustomConfigurations["autoscaler_max"] = o.ScalerMax
	p.CustomConfigurations["max_surge"] = o.Surge
	p.CustomConfigurations["max_unavailable"] = o.Unavailable
	p.CustomConfigurations["cidr"] = o.CIDR
	p.CustomConfigurations["workercidr"] = o.WCIDR

	for _, e := range o.Extra {
		v := strings.Split(e, "=")

		if len(v) != 2 {
			return p, fmt.Errorf("Wrong format for extra configuration %s. Please provide NAME=VALUE pairs.", e)
		}
		p.CustomConfigurations[v[0]] = v[1]
	}
	return p, nil
}

func (c *command) validateFlags() error {
	var errMessage strings.Builder
	// mandatory flags
	if c.opts.Name == "" {
		errMessage.WriteString("\nRequired flag `name` has not been set.")
	}
	if c.opts.Project == "" {
		errMessage.WriteString("\nRequired flag `project` has not been set.")
	}
	if c.opts.CredentialsFile == "" {
		errMessage.WriteString("\nRequired flag `credentials` has not been set.")
	}
	if c.opts.Secret == "" {
		errMessage.WriteString("\nRequired flag `secret` has not been set.")
	}

	if errMessage.Len() != 0 {
		return errors.New(errMessage.String())
	}
	return nil
}

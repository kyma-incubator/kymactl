package installation

import "time"

// Options holds the configuration options for the installation.
type Options struct {
	// Source specifies the installation source. To use the specific release, pass the release version (e.g. 1.6.0).
	// To use the latest master, pass "latest". To use the local sources, pass "local". To use the remote image, pass the installer image (e.g. user/my-kyma-installer:v1.6.0).
	Source string `json:"source"`

	// releaseVersion is set to the version of the release being installed.
	releaseVersion string
	// configVersion is set to the version of the configuration files being used.
	configVersion string
	// remoteImage holds the image URL if the installation source is an image.
	remoteImage string
	// registryTemplate specifies the registry image pattern.
	registryTemplate string
	// fromLocalSources is set if the installation source is local.
	fromLocalSources bool

	// LocalSrcPath specifies the absolute path to local sources.
	// +optional
	LocalSrcPath string `json:"localSrcPath,omitempty"`
	// OverrideConfigs specifies the path to a yaml file with parameters to override.
	// +optional
	OverrideConfigs []string `json:"overrideConfigs,omitempty"`
	// Password specifies the predefined cluster password.
	// +optional
	Password string `json:"password,omitempty"`
	// Domain specifies the domain used for installation.
	// +optional
	Domain string `json:"domain,omitempty"`
	// TLSCert specifies the TLS certificate for the domain used for installation
	// +optional
	TLSCert string `json:"tlsCert,omitempty"`
	// TLSKey specifies the TLS key for the domain used for installation.
	// +optional
	TLSKey string `json:"tlsKey,omitempty"`
	// IsLocal indicates if the installation is on a local cluster.
	// +optional
	IsLocal bool `json:"isLocal,omitempty"`
	// LocalCluster includes the configuration options of a local cluster.
	// +optional
	LocalCluster *LocalCluster `json:"localCluster,omitempty"`

	// Timeout specifies the time-out after which watching the installation progress stops.
	// +optional
	Timeout time.Duration `json:"timeout,omitempty"`
	// NoWait determines if the Kyma installation should be waited to complete.
	// +optional
	NoWait bool `json:"noWait,omitempty"`
	// Verbose enables displaying details of actions triggered.
	// +optional
	Verbose bool `json:"verbose,omitempty"`
	// KubeconfigPath specifies the path to the kubeconfig file. By default, the KUBECONFIG environment variable is used,
	// or /$HOME/.kube/config is used if the variable is not set.
	// +optional
	KubeconfigPath string `json:"kubeconfigPath,omitempty"`
}

// LocalCluster includes the configuration options of a local cluster.
type LocalCluster struct {
	// Provider specifies the provider of the local cluster.
	Provider string `json:"localProvider"`
	// Profile specifies the profile of the local cluster.
	Profile string `json:"localProfile"`
	// IP holds the IP of the local cluster.
	IP string `json:"localIP"`
	// VMDriver indicates the VM driver of the local cluster.
	VMDriver string `json:"localVMDriver"`
}

//NewOptions creates options with default values.
func NewOptions() *Options {
	return &Options{
		Timeout: defaultTimeout,
		Domain:  localDomain,
		Source:  defaultKymaVersion,
		IsLocal: true,
	}
}
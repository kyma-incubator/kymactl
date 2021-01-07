package deploy

import (
	"fmt"
	"strings"
	"time"

	"github.com/kyma-project/cli/internal/cli"
)

var kymaProfiles = []string{"evaluation", "production"}

// Options defines available options for the command
type Options struct {
	*cli.Options
	OverridesYaml  string
	ComponentsYaml string
	ResourcesPath  string
	CancelTimeout  time.Duration
	QuitTimeout    time.Duration
	HelmTimeout    time.Duration
	WorkersCount   int
	Profile        string
}

// NewOptions creates options with default values
func NewOptions(o *cli.Options) *Options {
	return &Options{Options: o}
}

// getProfiles return the currently supported profiles
func (o *Options) getProfiles() []string {
	return kymaProfiles
}

// isSupportedProfile verifies whether a given profile name is valid
func (o *Options) isSupportedProfile(profile string) bool {
	for _, supportedProfile := range kymaProfiles {
		if supportedProfile == profile {
			return true
		}
	}
	return false
}

// validateFlags applies a sanity check on provided options
func (o *Options) validateFlags() error {
	if o.ResourcesPath == "" {
		return fmt.Errorf("Resources path cannot be empty")
	}
	if o.ComponentsYaml == "" {
		return fmt.Errorf("Components YAML cannot be empty")
	}
	if o.QuitTimeout < o.CancelTimeout {
		return fmt.Errorf("Quit timeout (%v) cannot be smaller than cancel timeout (%v)", o.QuitTimeout, o.CancelTimeout)
	}
	if o.Profile != "" && !o.isSupportedProfile(o.Profile) {
		return fmt.Errorf("Profile unknown or not supported. Supported profiles are: %s", strings.Join(o.getProfiles(), ", "))
	}
	return nil
}

package install

import (
	"time"

	"github.com/kyma-incubator/kyma-cli/pkg/kyma/core"
)

//Options defines available options for the command
type Options struct {
	*core.Options
	ReleaseVersion        string
	ReleaseConfig         string
	NoWait                bool
	Domain                string
	Local                 bool
	LocalSrcPath          string
	LocalInstallerVersion string
	LocalInstallerDir     string
	Timeout               time.Duration
	Password              string
}

//NewOptions creates options with default values
func NewOptions(o *core.Options) *Options {
	return &Options{Options: o}
}

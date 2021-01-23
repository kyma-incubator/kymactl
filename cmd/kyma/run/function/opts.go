package function

import (
	"github.com/kyma-incubator/hydroform/function/pkg/workspace"
	"github.com/kyma-project/cli/internal/cli"
	"os"
	"path"
	"time"
)

//Options defines available options for the command
type Options struct {
	*cli.Options

	Filename     string
	ImageName    string
	BuildTimeout time.Duration
	BuildOnly    bool
}

//NewOptions creates options with default values
func NewOptions(o *cli.Options) *Options {
	options := &Options{Options: o}
	return options
}

const imageNameFormat = "%s:%s"

func (o *Options) setDefaults() (err error) {
	if o.Filename == "" {
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		o.Filename = path.Join(pwd, workspace.CfgFilename)
	}

	return
}

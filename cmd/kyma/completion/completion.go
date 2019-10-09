package completion

import (
	"fmt"
	"os"

	"github.com/pkg/errors"

	"github.com/spf13/cobra"
)

//NewCmd creates a new completion command
func NewCmd() *cobra.Command {
	var completionCmd = &cobra.Command{
		Use:   "completion bash|zsh",
		Short: "Generates bash or zsh completion scripts",
		Long: `Use this command to display the shell completion code used for interactive command completion. 
		To configure your shell to load completions, add ` + "`. <(kyma completion bash)`" + ` to your bash profile or ` + "`. <(kyma completion zsh)`" + ` to your zsh profile.
To load completion, run:
. <(kyma completion bash|zsh)
To configure your bash shell to load completions for each session, add to your bashrc:
# ~/.bashrc or ~/.profile
. <(kyma completion bash)
To configure your zsh shell to load completions for each session add to your zshrc
# ~/.zshrc
. <(kyma completion zsh)
`,
		RunE:    completion,
		Aliases: []string{},
	}
	completionCmd.Flags().Bool("help", false, "Displays help for the command.")
	return completionCmd
}

func completion(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		fmt.Println("Usage: kyma completion bash|zsh")
		fmt.Println("See 'kyma completion -h' for help")
		return nil
	}

	switch shell := args[0]; shell {
	case "bash":
		err := cmd.GenBashCompletion(os.Stdout)
		return errors.Wrap(err, "Error generating bash completion")
	case "zsh":
		err := cmd.GenZshCompletion(os.Stdout)
		return errors.Wrap(err, "Error generating zsh completion")
	default:
		fmt.Printf("Sorry, completion is not supported for %q", shell)
	}

	return nil
}

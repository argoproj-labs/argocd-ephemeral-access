package main

import (
	"fmt"
	"os"

	"github.com/argoproj-labs/ephemeral-access/cmd/backend"
	"github.com/argoproj-labs/ephemeral-access/cmd/controller"
	"github.com/spf13/cobra"
)

// const (
// 	commandNameEnv = "EPHEMERAL_COMMAND_NAME"
// 	cliName = "ephemeral-access"
// )

func main() {
	// commandName := filepath.Base(os.Args[0])
	// if val := os.Getenv(commandNameEnv); val != "" {
	// 	commandName = val
	// }

	command := &cobra.Command{
		Use:   "ephemeral-access",
		Short: "Ephemeral Access command entrypoint",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
		DisableAutoGenTag: true,
		SilenceUsage:      true,
	}

	command.AddCommand(backend.NewCommand())
	command.AddCommand(controller.NewCommand())

	// switch commandName {
	// case backend.CommandName:
	// 	command = backend.NewCommand()
	// case controller.CommandName:
	// 	command = controller.NewCommand()
	// default:
	// 	fmt.Fprintf(os.Stderr, "invalid command name: %s\n", commandName)
	// 	os.Exit(1)
	// }

	if err := command.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

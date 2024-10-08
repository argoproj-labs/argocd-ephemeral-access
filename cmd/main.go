package main

import (
	"fmt"
	"os"

	"github.com/argoproj-labs/ephemeral-access/cmd/backend"
	"github.com/argoproj-labs/ephemeral-access/cmd/controller"
	"github.com/spf13/cobra"
)

func main() {
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

	if err := command.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

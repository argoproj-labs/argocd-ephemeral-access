package main

import (
	"fmt"
	"os"

	"github.com/argoproj-labs/ephemeral-access/cmd/backend"
	"github.com/argoproj-labs/ephemeral-access/cmd/controller"
	"github.com/argoproj-labs/ephemeral-access/pkg/log"
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
		SilenceErrors:     true,
	}

	command.AddCommand(backend.NewCommand())
	command.AddCommand(controller.NewCommand())

	if err := command.Execute(); err != nil {
		logger, logerr := log.NewLogger()
		if logerr != nil {
			fmt.Fprintf(os.Stderr, "Backend execution error: %s", err)
			os.Exit(1)
		}
		logger.Error(err, "Backend execution error")
		os.Exit(1)
	}
}

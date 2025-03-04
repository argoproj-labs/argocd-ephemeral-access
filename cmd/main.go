package main

import (
	"fmt"
	"os"

	"github.com/argoproj-labs/argocd-ephemeral-access/cmd/backend"
	"github.com/argoproj-labs/argocd-ephemeral-access/cmd/controller"
	"github.com/argoproj-labs/argocd-ephemeral-access/pkg/log"
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
		msg := "ephemeral-access execution error"
		logger, logerr := log.NewLogger()
		if logerr != nil {
			fmt.Fprintf(os.Stderr, "%s: %s", msg, err)
			os.Exit(1)
		}
		logger.Error(err, msg)
		os.Exit(1)
	}
}

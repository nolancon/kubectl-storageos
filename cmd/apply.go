package cmd

import (
	"github.com/spf13/cobra"

	"github.com/storageos/kubectl-storageos/pkg/forwarder"
	"github.com/storageos/kubectl-storageos/pkg/logger"
)

func ApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "apply",
		Short:              "Make changes to existing resources",
		Args:               cobra.MinimumNArgs(0),
		SilenceUsage:       true,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		DisableFlagParsing: true,

		RunE: func(cmd *cobra.Command, args []string) error {
			return forwarder.ForwardToCLIPod(logger.NewLogger(), storageosCommand(cmd, args))
		},
	}

	return cmd
}
package cli

import (
	"github.com/spf13/cobra"
	"github.com/storageos/kubectl-storageos/pkg/forwarder"
	"github.com/storageos/kubectl-storageos/pkg/logger"
)

func UncordonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "uncordon",
		Short:              "Marks a node as uncordoned",
		Args:               cobra.MinimumNArgs(0),
		SilenceUsage:       true,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		DisableFlagParsing: true,

		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := forwarder.NewForwarder(logger.NewLogger())
			if err != nil {
				return err
			}

			return f.ForwardToCLIPod(storageosCommand(cmd, args))
		},
	}

	return cmd
}
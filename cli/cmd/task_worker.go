package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/impishMD/taskexec/services/tasks/containerworker"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use: "task-worker", Hidden: true, Args: cobra.NoArgs, SilenceUsage: true,
		// stdout is reserved for the worker protocol, including when log env is set.
		PersistentPreRun: func(*cobra.Command, []string) {},
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return containerworker.Run(ctx, os.Stdin, os.Stdout)
		},
	})
}

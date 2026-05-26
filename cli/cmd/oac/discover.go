package main

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/restrukt-ai/openagentcontainers/pkg/discovery"
)

func discoverCmd() *cobra.Command {
	var f commonFlags

	cmd := &cobra.Command{
		Use:   "discover <registry>",
		Short: "Discover OAC-conformant images in a registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiscover(cmd, args, f)
		},
	}

	f.register(cmd)

	return cmd
}

func runDiscover(cmd *cobra.Command, args []string, f commonFlags) error {
	opts, err := f.buildOpts()
	if err != nil {
		return err
	}

	defer saveCache(cmd.ErrOrStderr(), opts.Cache())

	agents, err := discovery.Discover(cmd.Context(), args[0], opts)
	if err != nil {
		return err
	}

	if f.outputJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(agents)
	}

	return writeAgentsTable(cmd.OutOrStdout(), agents)
}

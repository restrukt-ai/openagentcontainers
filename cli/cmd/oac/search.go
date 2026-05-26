package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/restrukt-ai/openagentcontainers/pkg/search"
)

func searchCmd() *cobra.Command {
	var f commonFlags

	cmd := &cobra.Command{
		Use:   "search <registry> <query>",
		Short: "Search for OAC agents matching a query across name, version, description, and labels",
		Args:  cobra.ExactArgs(searchArgCount),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd, args, f)
		},
	}

	f.register(cmd)

	return cmd
}

func runSearch(cmd *cobra.Command, args []string, f commonFlags) error {
	opts, err := f.buildOpts()
	if err != nil {
		return err
	}

	defer saveCache(cmd.ErrOrStderr(), opts.Cache())

	agents, err := search.Search(cmd.Context(), args[0], args[1], opts)
	if err != nil {
		return err
	}

	if len(agents) == 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "No agents found matching %q\n", args[1])

		return nil
	}

	if f.outputJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(agents)
	}

	return writeAgentsTable(cmd.OutOrStdout(), agents)
}

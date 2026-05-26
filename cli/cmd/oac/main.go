// Package main is the entry point for the oac CLI.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"

	"github.com/restrukt-ai/openagentcontainers/cli/cmd/internal/scancache"
	"github.com/restrukt-ai/openagentcontainers/pkg/discovery"
	"github.com/restrukt-ai/openagentcontainers/pkg/oac"
	"github.com/restrukt-ai/openagentcontainers/pkg/search"
)

const (
	defaultConcurrency = 10
	defaultRateLimit   = 10.0
	defaultMaxRetries  = 3
	tabwriterPadding   = 2
	searchArgCount     = 2
)

func main() {
	root := &cobra.Command{
		Use:           "oac",
		Short:         "Open Agent Containers CLI",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	root.AddCommand(discoverCmd())
	root.AddCommand(searchCmd())
	root.AddCommand(checkCmd())

	err := root.Execute()
	if err != nil {
		if !errors.Is(err, errCheckFailed) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}

		os.Exit(1)
	}
}

// commonFlags holds flags shared by discover and search.
type commonFlags struct {
	concurrency int
	rateLimit   float64
	maxRetries  int
	outputJSON  bool
	insecure    bool
	force       bool
	noCache     bool
	cachePath   string
}

func (f *commonFlags) register(cmd *cobra.Command) {
	cmd.Flags().
		IntVarP(&f.concurrency, "concurrency", "c", defaultConcurrency, "concurrent workers")
	cmd.Flags().Float64VarP(&f.rateLimit, "rate-limit", "r", defaultRateLimit,
		"max requests/sec (0 = unlimited)")
	cmd.Flags().IntVar(&f.maxRetries, "max-retries", defaultMaxRetries,
		"retries on 429/503 before giving up")
	cmd.Flags().BoolVar(&f.outputJSON, "json", false, "output as JSON")
	cmd.Flags().BoolVar(&f.insecure, "insecure", false, "use HTTP instead of HTTPS")
	cmd.Flags().BoolVar(&f.force, "force", false, "scan all tags even if latest lacks OAC labels")
	cmd.Flags().BoolVar(&f.noCache, "no-cache", false, "disable read/write of the local scan cache")
	cmd.Flags().StringVar(&f.cachePath, "cache-path", "", "override cache file location")
}

func (f *commonFlags) buildOpts() (discovery.Options, error) {
	var cache discovery.Cache

	if !f.noCache {
		path := f.cachePath
		if path == "" {
			var err error

			path, err = scancache.DefaultPath()
			if err != nil {
				return discovery.Options{}, fmt.Errorf("cache path: %w", err)
			}
		}

		var err error

		cache, err = scancache.Load(path)
		if err != nil {
			return discovery.Options{}, fmt.Errorf("load cache: %w", err)
		}
	}

	var limiter *rate.Limiter

	if f.rateLimit <= 0 {
		limiter = rate.NewLimiter(rate.Inf, 0)
	} else {
		limiter = rate.NewLimiter(rate.Limit(f.rateLimit), f.concurrency)
	}

	optFns := []discovery.Option{
		discovery.WithConcurrency(f.concurrency),
		discovery.WithMaxRetries(f.maxRetries),
		discovery.WithCache(cache),
		discovery.WithLimiter(limiter),
	}

	if f.insecure {
		optFns = append(optFns, discovery.WithCraneOpts(crane.Insecure))
	}

	if f.force {
		optFns = append(optFns, discovery.WithForce())
	}

	return discovery.NewOptions(optFns...), nil
}

// saveCache flushes the cache if non-nil; logs to w on failure.
func saveCache(w io.Writer, c discovery.Cache) {
	if c == nil {
		return
	}

	err := c.Save()
	if err != nil {
		fmt.Fprintf(w, "warning: save cache: %v\n", err)
	}
}

// writeAgentsTable writes agents as a tabwriter table to w.
func writeAgentsTable(w io.Writer, agents []oac.Image) error {
	tw := tabwriter.NewWriter(w, 0, 0, tabwriterPadding, ' ', 0)
	fmt.Fprintln(tw, "REFERENCE\tNAME\tVERSION\tDESCRIPTION")

	for _, a := range agents {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", a.Reference, a.Name(), a.SpecVersion, a.Description())
	}

	return tw.Flush()
}

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

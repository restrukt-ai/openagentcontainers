package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/spf13/cobra"

	"github.com/restrukt-ai/openagentcontainers/pkg/check"
	"github.com/restrukt-ai/openagentcontainers/pkg/discovery"
	"github.com/restrukt-ai/openagentcontainers/pkg/dockerfile"
	"github.com/restrukt-ai/openagentcontainers/pkg/oac"
)

// sentinel errors for fetchLabels and runCheck.
var (
	errMutuallyExclusive = errors.New("--dockerfile and --image are mutually exclusive")
	errUnknownInputMode  = errors.New("unknown input mode")
	// errCheckFailed signals exit code 1 without an additional error message;
	// it is checked by main() to suppress cobra's "Error: ..." output.
	errCheckFailed = errors.New("check: one or more errors found")
)

type checkFlags struct {
	outputJSON bool
	insecure   bool
	dockerfile bool
	image      bool
}

type inputMode int

const (
	modeDockerfile inputMode = iota
	modeImage
)

func detectInputMode(arg string, f checkFlags) inputMode {
	if f.dockerfile {
		return modeDockerfile
	}

	if f.image {
		return modeImage
	}

	_, statErr := os.Stat(arg)
	if statErr == nil {
		return modeDockerfile
	}

	if strings.HasPrefix(arg, "/") || strings.HasPrefix(arg, ".") {
		return modeDockerfile
	}

	return modeImage
}

func checkCmd() *cobra.Command {
	var f checkFlags

	cmd := &cobra.Command{
		Use:   "check <dockerfile-or-image-ref>",
		Short: "Check OAC labels from a Dockerfile or image reference",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(cmd, args, f)
		},
	}

	cmd.Flags().BoolVar(&f.outputJSON, "json", false, "output issues as JSON")
	cmd.Flags().BoolVar(&f.insecure, "insecure", false, "use HTTP instead of HTTPS")
	cmd.Flags().BoolVarP(&f.dockerfile, "dockerfile", "f", false, "force Dockerfile input mode")
	cmd.Flags().BoolVar(&f.image, "image", false, "force image reference input mode")

	return cmd
}

func fetchLabels(arg string, f checkFlags) (map[string]string, error) {
	switch detectInputMode(arg, f) {
	case modeDockerfile:
		file, err := os.Open(arg)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		return dockerfile.ParseLabels(file)
	case modeImage:
		var craneOpts []crane.Option

		if f.insecure {
			craneOpts = append(craneOpts, crane.Insecure)
		}

		return discovery.FetchLabels(arg, craneOpts...)
	default:
		return nil, errUnknownInputMode
	}
}

func encodeIssuesJSON(w io.Writer, issues []check.Issue) error {
	out := issues
	if out == nil {
		out = make([]check.Issue, 0)
	}

	return json.NewEncoder(w).Encode(out)
}

func runCheck(cmd *cobra.Command, args []string, f checkFlags) error {
	if f.dockerfile && f.image {
		return errMutuallyExclusive
	}

	labels, err := fetchLabels(args[0], f)
	if err != nil {
		return err
	}

	manifest, err := oac.Parse(labels)
	if err != nil {
		return err
	}

	issues := check.Check(manifest)

	if f.outputJSON {
		return encodeIssuesJSON(cmd.OutOrStdout(), issues)
	}

	if len(issues) == 0 {
		return nil
	}

	err = writeCheckTable(cmd.OutOrStdout(), issues)
	if err != nil {
		return err
	}

	for _, iss := range issues {
		if iss.Severity == check.SeverityError {
			return errCheckFailed
		}
	}

	return nil
}

func writeCheckTable(w io.Writer, issues []check.Issue) error {
	tw := tabwriter.NewWriter(w, 0, 0, tabwriterPadding, ' ', 0)
	fmt.Fprintln(tw, "SEVERITY\tFIELD\tMESSAGE")

	for _, iss := range issues {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", iss.Severity, iss.Field, iss.Message)
	}

	return tw.Flush()
}

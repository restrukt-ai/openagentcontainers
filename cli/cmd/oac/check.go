package main

import (
	"encoding/json"
	"errors"
	"fmt"
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

func fetchLabels(args []string, f checkFlags) (map[string]string, error) {
	switch detectInputMode(args[0], f) {
	case modeDockerfile:
		file, err := os.Open(args[0])
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

		return discovery.FetchLabels(args[0], craneOpts...)
	default:
		return nil, errUnknownInputMode
	}
}

func encodeCheckIssues(issues []check.Issue) error {
	out := issues
	if out == nil {
		out = make([]check.Issue, 0)
	}

	return json.NewEncoder(os.Stdout).Encode(out)
}

func runCheck(_ *cobra.Command, args []string, f checkFlags) error {
	if f.dockerfile && f.image {
		return errMutuallyExclusive
	}

	labels, err := fetchLabels(args, f)
	if err != nil {
		return err
	}

	manifest, err := oac.Parse(labels)
	if err != nil {
		return err
	}

	issues := check.Check(manifest)

	if f.outputJSON {
		return encodeCheckIssues(issues)
	}

	if len(issues) == 0 {
		return nil
	}

	err = writeCheckTable(issues)
	if err != nil {
		return err
	}

	for _, iss := range issues {
		if iss.Severity == check.SeverityError {
			os.Exit(1) //nolint:revive
		}
	}

	return nil
}

func writeCheckTable(issues []check.Issue) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, tabwriterPadding, ' ', 0)
	fmt.Fprintln(w, "SEVERITY\tFIELD\tMESSAGE")

	for _, iss := range issues {
		fmt.Fprintf(w, "%s\t%s\t%s\n", iss.Severity, iss.Field, iss.Message)
	}

	return w.Flush()
}

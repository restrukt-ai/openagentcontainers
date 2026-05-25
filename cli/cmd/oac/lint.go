package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/spf13/cobra"

	"github.com/restrukt-ai/openagentcontainers/pkg/discovery"
	"github.com/restrukt-ai/openagentcontainers/pkg/lint"
	"github.com/restrukt-ai/openagentcontainers/pkg/oac"
)

// sentinel errors for fetchLabels and runLint.
var (
	errMutuallyExclusive = errors.New("--dockerfile and --image are mutually exclusive")
	errUnknownInputMode  = errors.New("unknown input mode")
)

type lintFlags struct {
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

func detectInputMode(arg string, f lintFlags) inputMode {
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

func parseDockerfileLabels(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	result := make(map[string]string)
	scanner := bufio.NewScanner(f)

	var logicalLine strings.Builder

	for scanner.Scan() {
		raw := scanner.Text()

		if trimmed, ok := strings.CutSuffix(raw, "\\"); ok {
			logicalLine.WriteString(trimmed) //nolint:errcheck // strings.Builder never errors

			continue
		}

		logicalLine.WriteString(raw) //nolint:errcheck // strings.Builder never errors
		line := logicalLine.String()
		logicalLine.Reset()

		trimmed := strings.TrimSpace(line)

		if !strings.HasPrefix(strings.ToUpper(trimmed), "LABEL ") {
			continue
		}

		labelPart := strings.TrimSpace(trimmed[len("LABEL "):])
		maps.Copy(result, parseLabelPairs(labelPart))
	}

	err = scanner.Err()
	if err != nil {
		return nil, err
	}

	return result, nil
}

func skipLabelWhitespace(s string, i, n int) int {
	for i < n && (s[i] == ' ' || s[i] == '\t') {
		i++
	}

	return i
}

func readLabelKey(s string, i, n int) (string, int, bool) {
	keyStart := i

	for i < n && s[i] != '=' && s[i] != ' ' && s[i] != '\t' {
		i++
	}

	key := s[keyStart:i]

	if i >= n || s[i] != '=' || key == "" {
		return "", i, false
	}

	i++ // skip '='

	return key, i, true
}

func readQuotedLabelValue(s string, i, n int) (string, int) {
	i++ // skip opening quote

	var buf strings.Builder

	for i < n && s[i] != '"' {
		if s[i] == '\\' && i+1 < n {
			i++
		}

		buf.WriteByte(s[i]) //nolint:errcheck // strings.Builder never errors
		i++
	}

	if i < n {
		i++ // skip closing quote
	}

	return buf.String(), i
}

func readUnquotedLabelValue(s string, i, n int) (string, int) {
	valueStart := i

	for i < n && s[i] != ' ' && s[i] != '\t' {
		i++
	}

	return s[valueStart:i], i
}

func parseLabelPairs(s string) map[string]string {
	result := make(map[string]string)
	i := 0
	n := len(s)

	for i < n {
		i = skipLabelWhitespace(s, i, n)
		if i >= n {
			break
		}

		key, next, ok := readLabelKey(s, i, n)
		if !ok {
			break
		}

		i = next

		var value string

		if i < n && s[i] == '"' {
			value, i = readQuotedLabelValue(s, i, n)
		} else {
			value, i = readUnquotedLabelValue(s, i, n)
		}

		if key != "" {
			result[key] = value
		}
	}

	return result
}

func lintCmd() *cobra.Command {
	var f lintFlags

	cmd := &cobra.Command{
		Use:   "lint <dockerfile-or-image-ref>",
		Short: "Lint OAC labels from a Dockerfile or image reference",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLint(cmd, args, f)
		},
	}

	cmd.Flags().BoolVar(&f.outputJSON, "json", false, "output issues as JSON")
	cmd.Flags().BoolVar(&f.insecure, "insecure", false, "use HTTP instead of HTTPS")
	cmd.Flags().BoolVarP(&f.dockerfile, "dockerfile", "f", false, "force Dockerfile input mode")
	cmd.Flags().BoolVar(&f.image, "image", false, "force image reference input mode")

	return cmd
}

func fetchLabels(args []string, f lintFlags) (map[string]string, error) {
	switch detectInputMode(args[0], f) {
	case modeDockerfile:
		return parseDockerfileLabels(args[0])
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

func encodeLintIssues(issues []lint.Issue) error {
	out := issues
	if out == nil {
		out = make([]lint.Issue, 0)
	}

	return json.NewEncoder(os.Stdout).Encode(out)
}

func runLint(_ *cobra.Command, args []string, f lintFlags) error {
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

	issues := lint.Lint(manifest)

	if f.outputJSON {
		return encodeLintIssues(issues)
	}

	if len(issues) == 0 {
		return nil
	}

	err = writeLintTable(issues)
	if err != nil {
		return err
	}

	for _, iss := range issues {
		if iss.Severity == lint.SeverityError {
			os.Exit(1) //nolint:revive
		}
	}

	return nil
}

func writeLintTable(issues []lint.Issue) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, tabwriterPadding, ' ', 0)
	fmt.Fprintln(w, "SEVERITY\tFIELD\tMESSAGE")

	for _, iss := range issues {
		fmt.Fprintf(w, "%s\t%s\t%s\n", iss.Severity, iss.Field, iss.Message)
	}

	return w.Flush()
}

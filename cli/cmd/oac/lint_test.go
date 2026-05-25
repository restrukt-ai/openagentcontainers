package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/restrukt-ai/openagentcontainers/pkg/lint"
)

// --- parseLabelPairs ---

func TestParseLabelPairs_SingleQuoted(t *testing.T) {
	t.Parallel()

	got := parseLabelPairs(`org.openagentcontainers.name="my-agent"`)
	assert.Equal(t, map[string]string{"org.openagentcontainers.name": "my-agent"}, got)
}

func TestParseLabelPairs_SingleUnquoted(t *testing.T) {
	t.Parallel()

	got := parseLabelPairs(`key=value`)
	assert.Equal(t, map[string]string{"key": "value"}, got)
}

func TestParseLabelPairs_MultiplePairs(t *testing.T) {
	t.Parallel()

	got := parseLabelPairs(`key1="v1" key2="v2"`)
	assert.Equal(t, map[string]string{"key1": "v1", "key2": "v2"}, got)
}

func TestParseLabelPairs_EscapedQuote(t *testing.T) {
	t.Parallel()

	got := parseLabelPairs(`key="val\"ue"`)
	assert.Equal(t, map[string]string{"key": `val"ue`}, got)
}

func TestParseLabelPairs_EmptyQuotedValue(t *testing.T) {
	t.Parallel()

	got := parseLabelPairs(`key=""`)
	assert.Equal(t, map[string]string{"key": ""}, got)
}

func TestParseLabelPairs_EmptyInput(t *testing.T) {
	t.Parallel()

	got := parseLabelPairs(``)
	assert.Empty(t, got)
}

func TestParseLabelPairs_NoEquals(t *testing.T) {
	t.Parallel()

	// A token without '=' produces no entries.
	got := parseLabelPairs(`keyonly`)
	assert.Empty(t, got)
}

// --- parseDockerfileLabels ---

func writeDockerfile(t *testing.T, content string) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "Dockerfile")
	require.NoError(t, err)

	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	return f.Name()
}

func TestParseDockerfileLabels_SingleLabels(t *testing.T) {
	t.Parallel()

	path := writeDockerfile(t, `FROM alpine
LABEL org.openagentcontainers.version="v1alpha2"
LABEL org.openagentcontainers.name="test-agent"
`)

	labels, err := parseDockerfileLabels(path)
	require.NoError(t, err)
	assert.Equal(t, "v1alpha2", labels["org.openagentcontainers.version"])
	assert.Equal(t, "test-agent", labels["org.openagentcontainers.name"])
}

func TestParseDockerfileLabels_MultiPairLine(t *testing.T) {
	t.Parallel()

	path := writeDockerfile(t, `FROM alpine
LABEL key1="v1" key2="v2"
`)

	labels, err := parseDockerfileLabels(path)
	require.NoError(t, err)
	assert.Equal(t, "v1", labels["key1"])
	assert.Equal(t, "v2", labels["key2"])
}

func TestParseDockerfileLabels_LineContinuation(t *testing.T) {
	t.Parallel()

	path := writeDockerfile(t, `FROM alpine
LABEL key1="v1" \
      key2="v2"
`)

	labels, err := parseDockerfileLabels(path)
	require.NoError(t, err)
	assert.Equal(t, "v1", labels["key1"])
	assert.Equal(t, "v2", labels["key2"])
}

func TestParseDockerfileLabels_NonLabelLinesIgnored(t *testing.T) {
	t.Parallel()

	path := writeDockerfile(t, `FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY . /app
CMD ["/app/agent"]
`)

	labels, err := parseDockerfileLabels(path)
	require.NoError(t, err)
	assert.Empty(t, labels)
}

func TestParseDockerfileLabels_LabelCaseInsensitive(t *testing.T) {
	t.Parallel()

	path := writeDockerfile(t, `FROM alpine
label key="value"
`)

	labels, err := parseDockerfileLabels(path)
	require.NoError(t, err)
	assert.Equal(t, "value", labels["key"])
}

func TestParseDockerfileLabels_FileNotFound(t *testing.T) {
	t.Parallel()

	_, err := parseDockerfileLabels(filepath.Join(t.TempDir(), "nonexistent"))
	require.Error(t, err)
}

// --- detectInputMode ---

func TestDetectInputMode_DockerfileFlag(t *testing.T) {
	t.Parallel()

	mode := detectInputMode("anything", lintFlags{dockerfile: true})
	assert.Equal(t, modeDockerfile, mode)
}

func TestDetectInputMode_ImageFlag(t *testing.T) {
	t.Parallel()

	mode := detectInputMode("anything", lintFlags{image: true})
	assert.Equal(t, modeImage, mode)
}

func TestDetectInputMode_ExistingFile(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp(t.TempDir(), "Dockerfile")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	mode := detectInputMode(f.Name(), lintFlags{})
	assert.Equal(t, modeDockerfile, mode)
}

func TestDetectInputMode_AbsPathNoFile(t *testing.T) {
	t.Parallel()

	mode := detectInputMode("/nonexistent/path/Dockerfile", lintFlags{})
	assert.Equal(t, modeDockerfile, mode)
}

func TestDetectInputMode_RelativePath(t *testing.T) {
	t.Parallel()

	mode := detectInputMode("./examples/coding-agent/Dockerfile", lintFlags{})
	assert.Equal(t, modeDockerfile, mode)
}

func TestDetectInputMode_ImageRef(t *testing.T) {
	t.Parallel()

	mode := detectInputMode("registry.example.com/myagent:latest", lintFlags{})
	assert.Equal(t, modeImage, mode)
}

// --- writeLintTable ---

func TestWriteLintTable(t *testing.T) { //nolint:paralleltest // redirects os.Stdout
	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)

	old := os.Stdout
	os.Stdout = w

	issues := []lint.Issue{{
		Severity: lint.SeverityWarning,
		Field:    "description",
		Message:  "description is not set",
	}}
	err := writeLintTable(issues)

	w.Close()

	os.Stdout = old

	var buf bytes.Buffer

	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "SEVERITY")
	assert.Contains(t, buf.String(), "description")
}

// --- runLint ---

func TestRunLint_MutuallyExclusive(t *testing.T) {
	t.Parallel()

	f := lintFlags{dockerfile: true, image: true}
	err := runLint(nil, []string{"anything"}, f)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestRunLint_DockerfileReadError(t *testing.T) {
	t.Parallel()

	f := lintFlags{dockerfile: true}
	err := runLint(nil, []string{"nonexistent-path"}, f)
	require.Error(t, err)
}

func TestRunLint_OACParseError(t *testing.T) {
	t.Parallel()

	path := writeDockerfile(t, `FROM alpine
LABEL org.openagentcontainers.version="v99beta1"
LABEL org.openagentcontainers.name="agent"
`)

	f := lintFlags{dockerfile: true}
	err := runLint(nil, []string{path}, f)
	require.Error(t, err)
}

func TestRunLint_NoIssues(t *testing.T) {
	t.Parallel()

	path := writeDockerfile(t, `FROM alpine
LABEL org.openagentcontainers.version="v1alpha2"
LABEL org.openagentcontainers.name="agent"
LABEL org.openagentcontainers.description="something"
`)

	f := lintFlags{dockerfile: true}
	err := runLint(nil, []string{path}, f)
	require.NoError(t, err)
}

func TestRunLint_TableOutput(t *testing.T) { //nolint:paralleltest // redirects os.Stdout
	path := writeDockerfile(t, `FROM alpine
LABEL org.openagentcontainers.version="v1alpha2"
LABEL org.openagentcontainers.name="agent"
`)

	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)

	old := os.Stdout
	os.Stdout = w

	f := lintFlags{dockerfile: true, outputJSON: false}
	err := runLint(nil, []string{path}, f)

	w.Close()

	os.Stdout = old

	var buf bytes.Buffer

	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "SEVERITY")
	assert.Contains(t, buf.String(), "description")
}

func TestRunLint_JSONOutput(t *testing.T) { //nolint:paralleltest // redirects os.Stdout
	path := writeDockerfile(t, `FROM alpine
LABEL org.openagentcontainers.version="v1alpha2"
LABEL org.openagentcontainers.name="agent"
`)

	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)

	old := os.Stdout
	os.Stdout = w

	f := lintFlags{dockerfile: true, outputJSON: true}
	err := runLint(nil, []string{path}, f)

	w.Close()

	os.Stdout = old

	var buf bytes.Buffer

	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)

	require.NoError(t, err)

	var issues []map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &issues))
	require.NotEmpty(t, issues)
	assert.Equal(t, "warning", issues[0]["severity"])
}

func TestRunLint_ImageModeError(t *testing.T) {
	t.Parallel()

	f := lintFlags{image: true}
	err := runLint(nil, []string{"localhost:1/repo:latest"}, f)
	require.Error(t, err)
}

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/restrukt-ai/openagentcontainers/pkg/check"
)

func writeDockerfile(t *testing.T, content string) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "Dockerfile")
	require.NoError(t, err)

	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	return f.Name()
}

// --- detectInputMode ---

func TestDetectInputMode_DockerfileFlag(t *testing.T) {
	t.Parallel()

	mode := detectInputMode("anything", checkFlags{dockerfile: true})
	assert.Equal(t, modeDockerfile, mode)
}

func TestDetectInputMode_ImageFlag(t *testing.T) {
	t.Parallel()

	mode := detectInputMode("anything", checkFlags{image: true})
	assert.Equal(t, modeImage, mode)
}

func TestDetectInputMode_ExistingFile(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp(t.TempDir(), "Dockerfile")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	mode := detectInputMode(f.Name(), checkFlags{})
	assert.Equal(t, modeDockerfile, mode)
}

func TestDetectInputMode_AbsPathNoFile(t *testing.T) {
	t.Parallel()

	mode := detectInputMode("/nonexistent/path/Dockerfile", checkFlags{})
	assert.Equal(t, modeDockerfile, mode)
}

func TestDetectInputMode_RelativePath(t *testing.T) {
	t.Parallel()

	mode := detectInputMode("./examples/coding-agent/Dockerfile", checkFlags{})
	assert.Equal(t, modeDockerfile, mode)
}

func TestDetectInputMode_ImageRef(t *testing.T) {
	t.Parallel()

	mode := detectInputMode("registry.example.com/myagent:latest", checkFlags{})
	assert.Equal(t, modeImage, mode)
}

// --- writeCheckTable ---

func TestWriteCheckTable(t *testing.T) {
	t.Parallel()

	issues := []check.Issue{{
		Severity: check.SeverityWarning,
		Field:    "description",
		Message:  "description is not set",
	}}

	var buf bytes.Buffer

	err := writeCheckTable(&buf, issues)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "SEVERITY")
	assert.Contains(t, buf.String(), "description")
}

// --- runCheck ---

func TestRunCheck_MutuallyExclusive(t *testing.T) {
	t.Parallel()

	f := checkFlags{dockerfile: true, image: true}
	err := runCheck(makeCmd(), []string{"anything"}, f)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestRunCheck_DockerfileReadError(t *testing.T) {
	t.Parallel()

	f := checkFlags{dockerfile: true}
	err := runCheck(makeCmd(), []string{"nonexistent-path"}, f)
	require.Error(t, err)
}

func TestRunCheck_OACParseError(t *testing.T) {
	t.Parallel()

	path := writeDockerfile(t, `FROM alpine
LABEL org.openagentcontainers.version="v99beta1"
LABEL org.openagentcontainers.name="agent"
`)

	f := checkFlags{dockerfile: true}
	err := runCheck(makeCmd(), []string{path}, f)
	require.Error(t, err)
}

func TestRunCheck_NoIssues(t *testing.T) {
	t.Parallel()

	path := writeDockerfile(t, `FROM alpine
LABEL org.openagentcontainers.version="v1alpha2"
LABEL org.openagentcontainers.name="agent"
LABEL org.openagentcontainers.description="something"
LABEL org.openagentcontainers.orchestrator.env="ORCHESTRATOR_URL"
LABEL org.openagentcontainers.orchestrator.bearer.token.env="ORCHESTRATOR_TOKEN"
`)

	f := checkFlags{dockerfile: true}
	err := runCheck(makeCmd(), []string{path}, f)
	require.NoError(t, err)
}

func TestRunCheck_TableOutput(t *testing.T) {
	t.Parallel()

	path := writeDockerfile(t, `FROM alpine
LABEL org.openagentcontainers.version="v1alpha2"
LABEL org.openagentcontainers.name="agent"
LABEL org.openagentcontainers.orchestrator.env="ORCHESTRATOR_URL"
LABEL org.openagentcontainers.orchestrator.bearer.token.env="ORCHESTRATOR_TOKEN"
`)

	cmd := makeCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)

	f := checkFlags{dockerfile: true, outputJSON: false}
	err := runCheck(cmd, []string{path}, f)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "SEVERITY")
	assert.Contains(t, buf.String(), "description")
}

func TestRunCheck_JSONOutput(t *testing.T) {
	t.Parallel()

	path := writeDockerfile(t, `FROM alpine
LABEL org.openagentcontainers.version="v1alpha2"
LABEL org.openagentcontainers.name="agent"
`)

	cmd := makeCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)

	f := checkFlags{dockerfile: true, outputJSON: true}
	err := runCheck(cmd, []string{path}, f)

	require.NoError(t, err)

	var issues []map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &issues))
	require.NotEmpty(t, issues)
	assert.Equal(t, "warning", issues[0]["severity"])
}

func TestRunCheck_ImageModeError(t *testing.T) {
	t.Parallel()

	f := checkFlags{image: true}
	err := runCheck(makeCmd(), []string{"localhost:1/repo:latest"}, f)
	require.Error(t, err)
}

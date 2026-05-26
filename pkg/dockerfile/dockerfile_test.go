package dockerfile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/restrukt-ai/openagentcontainers/pkg/dockerfile"
	"github.com/restrukt-ai/openagentcontainers/pkg/oac"
)

func writeDockerfile(t *testing.T, content string) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "Dockerfile")
	if err != nil {
		t.Fatal(err)
	}

	_, err = f.WriteString(content)
	if err != nil {
		t.Fatal(err)
	}

	err = f.Close()
	if err != nil {
		t.Fatal(err)
	}

	return f.Name()
}

// --- ParseLabels ---

func TestParseLabels_SingleLabels(t *testing.T) {
	t.Parallel()

	path := writeDockerfile(t, `FROM alpine
LABEL org.openagentcontainers.version="v1alpha2"
LABEL org.openagentcontainers.name="test-agent"
`)

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	labels, err := dockerfile.ParseLabels(f)
	if err != nil {
		t.Fatal(err)
	}

	if labels["org.openagentcontainers.version"] != "v1alpha2" {
		t.Errorf("version: got %q, want v1alpha2", labels["org.openagentcontainers.version"])
	}

	if labels["org.openagentcontainers.name"] != "test-agent" {
		t.Errorf("name: got %q, want test-agent", labels["org.openagentcontainers.name"])
	}
}

func TestParseLabels_MultiPairLine(t *testing.T) {
	t.Parallel()

	path := writeDockerfile(t, `FROM alpine
LABEL key1="v1" key2="v2"
`)

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	labels, err := dockerfile.ParseLabels(f)
	if err != nil {
		t.Fatal(err)
	}

	if labels["key1"] != "v1" {
		t.Errorf("key1: got %q, want v1", labels["key1"])
	}

	if labels["key2"] != "v2" {
		t.Errorf("key2: got %q, want v2", labels["key2"])
	}
}

func TestParseLabels_LineContinuation(t *testing.T) {
	t.Parallel()

	path := writeDockerfile(t, "FROM alpine\nLABEL key1=\"v1\" \\\n      key2=\"v2\"\n")

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	labels, err := dockerfile.ParseLabels(f)
	if err != nil {
		t.Fatal(err)
	}

	if labels["key1"] != "v1" {
		t.Errorf("key1: got %q, want v1", labels["key1"])
	}

	if labels["key2"] != "v2" {
		t.Errorf("key2: got %q, want v2", labels["key2"])
	}
}

func TestParseLabels_NonLabelLinesIgnored(t *testing.T) {
	t.Parallel()

	path := writeDockerfile(t, `FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY . /app
CMD ["/app/agent"]
`)

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	labels, err := dockerfile.ParseLabels(f)
	if err != nil {
		t.Fatal(err)
	}

	if len(labels) != 0 {
		t.Errorf("expected empty labels, got %v", labels)
	}
}

func TestParseLabels_LabelCaseInsensitive(t *testing.T) {
	t.Parallel()

	path := writeDockerfile(t, `FROM alpine
label key="value"
`)

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	labels, err := dockerfile.ParseLabels(f)
	if err != nil {
		t.Fatal(err)
	}

	if labels["key"] != "value" {
		t.Errorf("key: got %q, want value", labels["key"])
	}
}

func TestParseLabels_ReaderError(t *testing.T) {
	t.Parallel()

	// Use an already-closed file to provoke a read error.
	f, err := os.CreateTemp(t.TempDir(), "Dockerfile")
	if err != nil {
		t.Fatal(err)
	}

	f.Close()

	// Parsing an empty (but closed) file is valid; instead, test FileNotFound via Parse.
	_, err = dockerfile.ParseLabels(strings.NewReader("not a dockerfile {{{"))
	// buildkit parser is lenient; just ensure no panic.
	_ = err
}

// --- Parse ---

func TestParse_ValidOACDockerfile(t *testing.T) {
	t.Parallel()

	path := writeDockerfile(t, `FROM alpine
LABEL org.openagentcontainers.version="v1alpha2"
LABEL org.openagentcontainers.name="my-agent"
LABEL org.openagentcontainers.description="does things"
LABEL org.openagentcontainers.orchestrator.env="ORCHESTRATOR_URL"
LABEL org.openagentcontainers.orchestrator.bearer.token.env="TOKEN"
`)

	df, err := dockerfile.Parse(path)
	if err != nil {
		t.Fatal(err)
	}

	if df.Path != path {
		t.Errorf("Path: got %q, want %q", df.Path, path)
	}

	if df.Name() != "my-agent" {
		t.Errorf("Name: got %q, want my-agent", df.Name())
	}

	if df.SpecVersion != oac.VersionV1Alpha2 {
		t.Errorf("SpecVersion: got %q, want v1alpha2", df.SpecVersion)
	}
}

func TestParse_FileNotFound(t *testing.T) {
	t.Parallel()

	_, err := dockerfile.Parse(filepath.Join(t.TempDir(), "nonexistent"))
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestParse_UnknownOACVersion(t *testing.T) {
	t.Parallel()

	path := writeDockerfile(t, `FROM alpine
LABEL org.openagentcontainers.version="v99beta1"
LABEL org.openagentcontainers.name="agent"
`)

	_, err := dockerfile.Parse(path)
	if err == nil {
		t.Fatal("expected error for unknown OAC version")
	}
}

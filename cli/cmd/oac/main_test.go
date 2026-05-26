package main

import (
	"bytes"
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	gcrregistry "github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/restrukt-ai/openagentcontainers/pkg/oac"
)

// errSaveCacheDiskFull is a static sentinel used by TestSaveCache_Error.
var errSaveCacheDiskFull = errors.New("disk full")

// --- mockCache ---

type mockCache struct{ saveErr error }

func (m *mockCache) GetDigest(string) ([]byte, bool)       { return nil, false }
func (m *mockCache) SetDigest(string, []byte)              {}
func (m *mockCache) GetLatestDigest(string) (string, bool) { return "", false }
func (m *mockCache) SetLatestDigest(_, _ string)           {}
func (m *mockCache) Save() error                           { return m.saveErr }

// --- helpers ---

func makeCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())

	return cmd
}

func testRegistry(t *testing.T) (string, []crane.Option) {
	t.Helper()

	srv := httptest.NewServer(gcrregistry.New())
	t.Cleanup(srv.Close)

	return strings.TrimPrefix(srv.URL, "http://"), []crane.Option{crane.Insecure}
}

func makeImage(t *testing.T, labels map[string]string) v1.Image {
	t.Helper()

	img, err := mutate.ConfigFile(empty.Image, &v1.ConfigFile{
		Config: v1.Config{Labels: labels},
	})
	if err != nil {
		t.Fatal(err)
	}

	return img
}

func push(t *testing.T, img v1.Image, ref string, opts []crane.Option) {
	t.Helper()

	err := crane.Push(img, ref, opts...)
	if err != nil {
		t.Fatalf("push %s: %v", ref, err)
	}
}

// --- saveCache ---

func TestSaveCache_Nil(t *testing.T) {
	t.Parallel()

	saveCache(bytes.NewBuffer(nil), nil) // must not panic
}

func TestSaveCache_Success(t *testing.T) {
	t.Parallel()

	saveCache(bytes.NewBuffer(nil), &mockCache{saveErr: nil}) // must not panic
}

func TestSaveCache_Error(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	saveCache(&buf, &mockCache{saveErr: errSaveCacheDiskFull})

	assert.Contains(t, buf.String(), "warning: save cache")
	assert.Contains(t, buf.String(), "disk full")
}

// --- buildOpts ---

func TestBuildOpts_NoCacheTrue(t *testing.T) {
	t.Parallel()

	f := commonFlags{noCache: true, rateLimit: 10, concurrency: 1}
	opts, err := f.buildOpts()
	require.NoError(t, err)
	assert.Nil(t, opts.Cache())
}

func TestBuildOpts_ExplicitCachePath(t *testing.T) {
	t.Parallel()

	f := commonFlags{
		noCache:     false,
		cachePath:   t.TempDir() + "/c.json",
		rateLimit:   10,
		concurrency: 1,
	}
	opts, err := f.buildOpts()
	require.NoError(t, err)
	assert.NotNil(t, opts.Cache())
}

func TestBuildOpts_DefaultCachePath(t *testing.T) {
	t.Parallel()

	f := commonFlags{noCache: false, cachePath: "", rateLimit: 10, concurrency: 1}
	opts, err := f.buildOpts()
	require.NoError(t, err)
	assert.NotNil(t, opts.Cache())
}

func TestBuildOpts_RateLimitZero(t *testing.T) {
	t.Parallel()

	f := commonFlags{noCache: true, rateLimit: 0, concurrency: 1}
	_, err := f.buildOpts()
	require.NoError(t, err)
}

func TestBuildOpts_Insecure(t *testing.T) {
	t.Parallel()

	f := commonFlags{noCache: true, insecure: true, rateLimit: 10, concurrency: 1}
	_, err := f.buildOpts()
	require.NoError(t, err)
}

func TestBuildOpts_Force(t *testing.T) {
	t.Parallel()

	f := commonFlags{noCache: true, force: true, rateLimit: 10, concurrency: 1}
	_, err := f.buildOpts()
	require.NoError(t, err)
}

// --- writeAgentsTable ---

func TestWriteAgentsTable_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	err := writeAgentsTable(&buf, nil)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "REFERENCE")
}

func TestWriteAgentsTable_WithData(t *testing.T) {
	t.Parallel()

	agents := []oac.Image{
		{
			Manifest: oac.Manifest{
				SpecVersion: oac.VersionV1Alpha1,
				V1Alpha1:    &oac.V1Alpha1Spec{Name: "my-agent"},
			},
			Reference: "reg/my-agent:latest",
		},
	}

	var buf bytes.Buffer

	err := writeAgentsTable(&buf, agents)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "my-agent")
	assert.Contains(t, buf.String(), "reg/my-agent:latest")
}

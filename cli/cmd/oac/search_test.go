package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/restrukt-ai/openagentcontainers/pkg/oac"
)

func TestRunSearch_NoResults(t *testing.T) {
	t.Parallel()

	host, craneOpts := testRegistry(t)

	img := makeImage(t, map[string]string{
		oac.LabelVersion: string(oac.VersionV1Alpha1),
		oac.LabelName:    "code-agent",
	})
	push(t, img, host+"/code-agent:latest", craneOpts)

	cmd := makeCmd()

	var buf bytes.Buffer
	cmd.SetErr(&buf)

	f := commonFlags{noCache: true, rateLimit: 10, concurrency: 2}
	err := runSearch(cmd, []string{host, "zzz"}, f)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No agents found")
}

func TestRunSearch_JSONOutput(t *testing.T) {
	t.Parallel()

	host, craneOpts := testRegistry(t)

	img := makeImage(t, map[string]string{
		oac.LabelVersion: string(oac.VersionV1Alpha1),
		oac.LabelName:    "code-agent",
	})
	push(t, img, host+"/code-agent:latest", craneOpts)

	cmd := makeCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)

	f := commonFlags{outputJSON: true, noCache: true, rateLimit: 10, concurrency: 2}
	err := runSearch(cmd, []string{host, "code"}, f)

	require.NoError(t, err)

	var agents []oac.Image
	require.NoError(t, json.NewDecoder(&buf).Decode(&agents))
	assert.Len(t, agents, 1)
}

func TestRunSearch_TableOutput(t *testing.T) {
	t.Parallel()

	host, craneOpts := testRegistry(t)

	img := makeImage(t, map[string]string{
		oac.LabelVersion: string(oac.VersionV1Alpha1),
		oac.LabelName:    "code-agent",
	})
	push(t, img, host+"/code-agent:latest", craneOpts)

	cmd := makeCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)

	f := commonFlags{outputJSON: false, noCache: true, rateLimit: 10, concurrency: 2}
	err := runSearch(cmd, []string{host, "code"}, f)

	require.NoError(t, err)
	assert.Contains(t, buf.String(), "REFERENCE")
}

func TestRunSearch_Error(t *testing.T) {
	t.Parallel()

	f := commonFlags{noCache: true, rateLimit: 10, concurrency: 2}
	err := runSearch(makeCmd(), []string{"localhost:1", "query"}, f)
	require.Error(t, err)
}

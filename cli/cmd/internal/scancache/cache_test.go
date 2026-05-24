package scancache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func tempPath(t *testing.T) string {
	t.Helper()

	return filepath.Join(t.TempDir(), "registry.json")
}

// TestLoadMissing verifies that a missing file yields an empty, usable cache.
func TestLoadMissing(t *testing.T) {
	t.Parallel()

	c, err := Load(tempPath(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, found := c.GetDigest("sha256:abc"); found {
		t.Fatal("expected empty cache")
	}
}

// TestLoadCorrupt verifies that a corrupt file is silently replaced with an empty cache.
func TestLoadCorrupt(t *testing.T) {
	t.Parallel()
	p := tempPath(t)

	err := os.WriteFile(p, []byte("not json{{{"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load corrupt: %v", err)
	}

	if _, found := c.GetDigest("sha256:abc"); found {
		t.Fatal("expected empty cache after corrupt load")
	}
}

// TestSaveLoadRoundtrip verifies that written entries survive a reload.
func TestSaveLoadRoundtrip(t *testing.T) {
	t.Parallel()
	p := tempPath(t)

	c, err := Load(p)
	require.NoError(t, err)

	agentJSON := []byte(`{"reference":"reg/img:latest","version":"1.0","name":"agent","labels":{}}`)
	c.SetDigest("sha256:111", agentJSON)
	c.SetDigest("sha256:222", nil) // non-OAC
	c.SetLatestDigest("reg/img", "sha256:111")

	err = c.Save()
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	c2, err := Load(p)
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}

	got, found := c2.GetDigest("sha256:111")
	if !found {
		t.Fatal("digest sha256:111 not found after reload")
	}

	if string(got) != string(agentJSON) {
		t.Fatalf("digest sha256:111: got %s, want %s", got, agentJSON)
	}

	got2, found2 := c2.GetDigest("sha256:222")
	if !found2 {
		t.Fatal("digest sha256:222 not found after reload")
	}

	if got2 != nil {
		t.Fatalf("digest sha256:222: expected nil agent, got %s", got2)
	}

	digest, ok := c2.GetLatestDigest("reg/img")
	if !ok || digest != "sha256:111" {
		t.Fatalf("GetLatestDigest: got %q %v", digest, ok)
	}
}

// TestSaveNoop verifies Save does not write when nothing has changed.
func TestSaveNoop(t *testing.T) {
	t.Parallel()
	p := tempPath(t)

	c, err := Load(p)
	require.NoError(t, err)

	// Save on a pristine cache should not create the file.
	err = c.Save()
	if err != nil {
		t.Fatal(err)
	}

	_, statErr := os.Stat(p)
	if !os.IsNotExist(statErr) {
		t.Fatal("expected no file to be created for a no-op Save")
	}
}

// TestSaveIdempotent verifies Save is a no-op when called twice without changes.
func TestSaveIdempotent(t *testing.T) {
	t.Parallel()
	p := tempPath(t)

	c, err := Load(p)
	require.NoError(t, err)
	c.SetDigest("sha256:aaa", nil)

	err = c.Save()
	if err != nil {
		t.Fatal(err)
	}

	info1, err := os.Stat(p)
	require.NoError(t, err)

	// Sleep briefly so mtime would differ if file were re-written.
	time.Sleep(10 * time.Millisecond)

	err = c.Save()
	if err != nil {
		t.Fatal(err)
	}

	info2, err := os.Stat(p)
	require.NoError(t, err)

	if !info1.ModTime().Equal(info2.ModTime()) {
		t.Fatal("Save rewrote file when nothing changed")
	}
}

// TestGetSetDigest covers the basic get/set contract.
func TestGetSetDigest(t *testing.T) {
	t.Parallel()
	c, err := Load(tempPath(t))
	require.NoError(t, err)

	// Miss
	if _, found := c.GetDigest("sha256:xyz"); found {
		t.Fatal("unexpected hit on empty cache")
	}

	// Set OAC
	c.SetDigest("sha256:oac", []byte(`{"reference":"r","version":"1","name":"n","labels":{}}`))

	got, found := c.GetDigest("sha256:oac")
	if !found || got == nil {
		t.Fatal("expected OAC hit")
	}

	// Set non-OAC
	c.SetDigest("sha256:nooac", nil)

	got2, found2 := c.GetDigest("sha256:nooac")
	if !found2 || got2 != nil {
		t.Fatalf("expected non-OAC hit, got found=%v agent=%v", found2, got2)
	}
}

// TestSetDigestMarksDirty verifies SetDigest marks the cache dirty.
func TestSetDigestMarksDirty(t *testing.T) {
	t.Parallel()
	c, err := Load(tempPath(t))
	require.NoError(t, err)

	if c.dirty {
		t.Fatal("fresh cache should not be dirty")
	}

	c.SetDigest("sha256:x", nil)

	if !c.dirty {
		t.Fatal("cache should be dirty after SetDigest")
	}
}

// TestSetLatestDigestIdempotent verifies no dirty-mark when value unchanged.
func TestSetLatestDigestIdempotent(t *testing.T) {
	t.Parallel()
	c, err := Load(tempPath(t))
	require.NoError(t, err)

	c.SetLatestDigest("repo", "sha256:aaa")

	if !c.dirty {
		t.Fatal("should be dirty after first set")
	}

	c.dirty = false // reset

	c.SetLatestDigest("repo", "sha256:aaa") // same value

	if c.dirty {
		t.Fatal("should NOT be dirty when value unchanged")
	}
}

// TestGetSetLatestDigest covers the basic repo-latest contract.
func TestGetSetLatestDigest(t *testing.T) {
	t.Parallel()
	c, err := Load(tempPath(t))
	require.NoError(t, err)

	_, found := c.GetLatestDigest("reg/img")
	if found {
		t.Fatal("unexpected hit on empty cache")
	}

	c.SetLatestDigest("reg/img", "sha256:abc")

	d, ok := c.GetLatestDigest("reg/img")
	if !ok || d != "sha256:abc" {
		t.Fatalf("got %q %v", d, ok)
	}
}

// TestDefaultPath verifies DefaultPath returns a non-empty string without error.
func TestDefaultPath(t *testing.T) {
	t.Parallel()

	p, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}

	if p == "" {
		t.Fatal("DefaultPath returned empty string")
	}
}

// TestConcurrentAccess verifies the cache is safe under concurrent reads/writes.
func TestConcurrentAccess(t *testing.T) {
	t.Parallel()
	c, err := Load(tempPath(t))
	require.NoError(t, err)

	const goroutines = 50

	var wg sync.WaitGroup

	for i := range goroutines {
		wg.Add(1)

		go func(n int) {
			defer wg.Done()

			digest := "sha256:" + string(rune('a'+n%26))
			c.SetDigest(digest, nil)
			c.GetDigest(digest)
			c.SetLatestDigest("repo", digest)
			c.GetLatestDigest("repo")
		}(i)
	}

	wg.Wait()
}

// TestSaveCreatesParentDirs verifies Save creates missing parent directories.
func TestSaveCreatesParentDirs(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "nested", "deep", "registry.json")

	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}

	c.SetDigest("sha256:x", nil)

	err = c.Save()
	if err != nil {
		t.Fatalf("Save with nested dirs: %v", err)
	}

	_, statErr := os.Stat(p)
	if statErr != nil {
		t.Fatalf("file not created: %v", statErr)
	}
}

// TestLoadOpenError verifies Load returns an error for a non-NotExist open failure.
func TestLoadOpenError(t *testing.T) {
	t.Parallel()

	if os.Getuid() == 0 {
		t.Skip("running as root: permission checks don't apply")
	}

	p := tempPath(t)

	// Write a file and make it unreadable so os.Open fails with permission denied.
	err := os.WriteFile(p, []byte("{}"), 0o000)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Load(p)
	if err == nil {
		t.Fatal("expected error loading unreadable file")
	}
}

// TestSaveMkdirAllFail verifies Save returns an error when MkdirAll fails.
func TestSaveMkdirAllFail(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	// A file at parentDir blocks MkdirAll from treating it as a directory.
	parentDir := filepath.Join(base, "subdir")
	p := filepath.Join(parentDir, "registry.json")

	c, err := Load(p) // succeeds: IsNotExist
	if err != nil {
		t.Fatal(err)
	}

	c.SetDigest("sha256:x", nil)

	// Now create a regular file at parentDir so MkdirAll(parentDir) fails.
	err = os.WriteFile(parentDir, []byte("block"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	err = c.Save()
	if err == nil {
		t.Fatal("expected error when MkdirAll fails")
	}
}

// TestSaveCreateTempFail verifies Save returns an error when CreateTemp fails.
func TestSaveCreateTempFail(t *testing.T) {
	t.Parallel()

	if os.Getuid() == 0 {
		t.Skip("running as root: permission checks don't apply")
	}

	p := tempPath(t)

	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}

	c.SetDigest("sha256:x", nil)

	// Make the cache directory unwritable so CreateTemp fails.
	dir := filepath.Dir(p)

	err = os.Chmod(dir, 0o444)
	if err != nil {
		t.Fatal(err)
	}

	defer os.Chmod(dir, 0o755) //nolint:errcheck

	err = c.Save()
	if err == nil {
		t.Fatal("expected error when CreateTemp fails")
	}
}

// TestSaveRenameError verifies Save returns an error when the atomic rename fails.
func TestSaveRenameError(t *testing.T) {
	t.Parallel()
	p := tempPath(t)

	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}

	c.SetDigest("sha256:x", nil)

	// Create a directory at c.path so Rename(tmpFile, p) fails (EISDIR).
	err = os.Mkdir(p, 0o755)
	if err != nil {
		t.Fatal(err)
	}

	err = c.Save()
	if err == nil {
		t.Fatal("expected Rename error when destination is a directory")
	}
}

// TestDefaultPathError verifies DefaultPath returns an error when HOME is unset.
func TestDefaultPathError(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CACHE_HOME", "") // suppress Linux fallback

	_, err := DefaultPath()
	if err == nil {
		t.Skip("os.UserCacheDir succeeded without HOME (platform may not require it)")
	}
}

// TestVersionPreserved verifies the version field survives a save/load cycle.
func TestVersionPreserved(t *testing.T) {
	t.Parallel()
	p := tempPath(t)

	c, err := Load(p)
	require.NoError(t, err)
	c.SetDigest("sha256:v", nil)

	err = c.Save()
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(p)
	require.NoError(t, err)

	defer f.Close()

	var d cacheData

	err = json.NewDecoder(f).Decode(&d)
	if err != nil {
		t.Fatal(err)
	}

	if d.Version != fileVersion {
		t.Fatalf("version: got %d, want %d", d.Version, fileVersion)
	}
}

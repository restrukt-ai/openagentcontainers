// Package scancache provides a file-backed, thread-safe cache for OCI registry scan results.
// Digest-keyed entries never expire because OCI digests are content-addressed.
package scancache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	fileVersion = 2
	dirPerm     = 0o755
)

type digestEntry struct {
	// Agent is the marshaled AgentImage, or nil if the image was inspected and
	// confirmed to have no OAC labels.
	Agent     []byte    `json:"agent,omitempty"`
	ScannedAt time.Time `json:"scannedAt"`
}

type cacheData struct {
	Version    int                    `json:"version"`
	Digests    map[string]digestEntry `json:"digests"`
	RepoLatest map[string]string      `json:"repoLatest"`
}

// Cache is a thread-safe, file-backed store for registry scan results.
type Cache struct {
	mu    sync.RWMutex
	path  string
	d     cacheData
	dirty bool
}

// DefaultPath returns the platform-appropriate cache file path (~/.cache/oac/registry.json).
func DefaultPath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "oac", "registry.json"), nil
}

// Load reads the cache from path. Returns an empty cache if the file does not
// exist. A corrupt file is silently discarded and replaced with an empty cache.
func Load(path string) (*Cache, error) {
	c := &Cache{
		path: path,
		d: cacheData{
			Version:    fileVersion,
			Digests:    make(map[string]digestEntry),
			RepoLatest: make(map[string]string),
		},
	}

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return c, nil
	}

	if err != nil {
		return nil, err
	}

	defer f.Close()

	err = json.NewDecoder(f).Decode(&c.d)
	if err != nil {
		// Corrupt cache — start fresh rather than hard-failing.
		c.d.Digests = make(map[string]digestEntry)
		c.d.RepoLatest = make(map[string]string)
	}

	return c, nil
}

// Save writes the cache to disk atomically via a temp-file rename.
// It is a no-op if nothing has changed since Load.
func (c *Cache) Save() error {
	c.mu.RLock()
	dirty := c.dirty
	c.mu.RUnlock()

	if !dirty {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	err := os.MkdirAll(filepath.Dir(c.path), dirPerm)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(c.path), "oac-cache-*.tmp")
	if err != nil {
		return err
	}

	tmpName := tmp.Name()

	err = json.NewEncoder(tmp).Encode(c.d)
	if err != nil {
		return errors.Join(err, tmp.Close(), os.Remove(tmpName))
	}

	err = tmp.Close()
	if err != nil {
		return errors.Join(err, os.Remove(tmpName))
	}

	err = os.Rename(tmpName, c.path)
	if err != nil {
		return errors.Join(err, os.Remove(tmpName))
	}

	c.dirty = false

	return nil
}

// GetDigest returns the cached result for a digest.
// agentJSON is nil when the image was inspected and confirmed to have no OAC labels.
// found is false when the digest is not present in the cache.
func (c *Cache) GetDigest(digest string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.d.Digests[digest]

	return e.Agent, ok
}

// SetDigest stores a result for a digest.
// Pass nil agentJSON to record a confirmed non-OAC image.
func (c *Cache) SetDigest(digest string, agentJSON []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.d.Digests[digest] = digestEntry{Agent: agentJSON, ScannedAt: time.Now()}
	c.dirty = true
}

// GetLatestDigest returns the last-seen digest for a repo's "latest" tag.
func (c *Cache) GetLatestDigest(repo string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	d, ok := c.d.RepoLatest[repo]

	return d, ok
}

// SetLatestDigest records the current digest for a repo's "latest" tag.
// It is a no-op when the stored digest is already up to date.
func (c *Cache) SetLatestDigest(repo, digest string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.d.RepoLatest[repo] == digest {
		return
	}

	c.d.RepoLatest[repo] = digest
	c.dirty = true
}

// Package discovery enumerates OCI registries to find images that declare OAC conformance
// via the org.openagentcontainers.version label.
//
// Use [Discover] to scan an entire registry. Always construct [Options] via [NewOptions];
// the zero value panics at runtime. At minimum, provide a rate limiter and at least one
// worker:
//
//	opts := discovery.NewOptions(
//		discovery.WithLimiter(rate.NewLimiter(rate.Every(time.Second), 10)),
//		discovery.WithConcurrency(4),
//	)
//	agents, err := discovery.Discover(ctx, "registry.example.com", opts)
//
// To reduce network traffic on repeated scans, pass a [Cache] via [WithCache]: previously
// seen content-addressed digests are not re-fetched, and repos confirmed non-OAC are
// skipped entirely. Use [WithForce] to override this optimisation.
//
// [FetchLabels] is a lower-level helper for fetching raw OCI labels for a single known
// image reference without scanning a full registry.
package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/go-containerregistry/pkg/crane"
	"golang.org/x/time/rate"

	"github.com/restrukt-ai/openagentcontainers/pkg/oac"
)

// Cache is the interface for registry scan caches consumed by Discover.
// Implement it to persist scan results between runs and avoid re-fetching image configs.
type Cache interface {
	// GetDigest returns the cached JSON for the given content-addressed digest.
	// The bool is true when an entry exists. A nil byte slice with true means the
	// image was previously confirmed non-OAC; callers should skip it.
	GetDigest(digest string) ([]byte, bool)

	// SetDigest stores agentJSON for digest. Pass nil agentJSON to record a confirmed
	// non-OAC result so future scans can skip the image.
	SetDigest(digest string, agentJSON []byte)

	// GetLatestDigest returns the digest last seen for the repo's "latest" tag.
	// Used to detect whether the tag has changed since the previous scan.
	GetLatestDigest(repo string) (string, bool)

	// SetLatestDigest records the current digest for the repo's "latest" tag.
	SetLatestDigest(repo, digest string)

	// Save persists the cache to durable storage. Call after Discover completes.
	Save() error
}

// Option is a functional option for [NewOptions]. Use the With* functions to construct options.
type Option func(*Options)

// Options configures a Discover or Search call.
// Zero-value Options will panic at runtime: Limiter must be non-nil (see WithLimiter)
// and Concurrency must be ≥1 (see WithConcurrency). Always construct via NewOptions.
// Options must not be copied after construction.
type Options struct {
	concurrency int
	maxRetries  int
	force       bool
	cache       Cache
	limiter     *rate.Limiter
	craneOpts   []crane.Option
}

// NewOptions returns Options with all provided opts applied.
// Defaults: Concurrency=0 (no workers — must call WithConcurrency), MaxRetries=0
// (single attempt), Limiter=nil (panics — must call WithLimiter), Force=false,
// Cache=nil (no caching).
// If WithConcurrency is not called (or n=0), Discover starts no workers and returns an empty
// result with no error — always call WithConcurrency with n ≥ 1.
func NewOptions(opts ...Option) Options {
	o := Options{}

	for _, opt := range opts {
		opt(&o)
	}

	return o
}

// Cache returns the configured cache, allowing callers to call Cache.Save after a Discover
// run to persist results for the next scan.
func (o Options) Cache() Cache {
	return o.cache
}

// WithConcurrency sets the number of concurrent workers.
// n must be ≥1. Passing 0 causes Discover to spawn no workers and return an empty result.
func WithConcurrency(n int) Option {
	return func(o *Options) {
		o.concurrency = n
	}
}

// WithMaxRetries sets the maximum number of retries on transient errors.
// n=0 means a single attempt with no retries on 429/503 responses.
func WithMaxRetries(n int) Option {
	return func(o *Options) {
		o.maxRetries = n
	}
}

// WithForce enables scanning all tags even when the latest tag lacks OAC labels.
func WithForce() Option {
	return func(o *Options) {
		o.force = true
	}
}

// WithCache attaches a scan cache to avoid re-fetching previously seen digests.
func WithCache(c Cache) Option {
	return func(o *Options) {
		o.cache = c
	}
}

// WithLimiter sets the rate limiter for registry requests.
// Required. A nil limiter causes Discover and Search to panic.
// For no rate limiting, use rate.NewLimiter(rate.Inf, 0).
func WithLimiter(l *rate.Limiter) Option {
	return func(o *Options) {
		o.limiter = l
	}
}

// WithCraneOpts appends crane options used for all registry requests, e.g. crane.Insecure for
// plain-HTTP registries or crane.WithAuthFromKeychain(authn.DefaultKeychain) from
// github.com/google/go-containerregistry/pkg/authn for private registries requiring authentication.
func WithCraneOpts(opts ...crane.Option) Option {
	return func(o *Options) {
		o.craneOpts = append(o.craneOpts, opts...)
	}
}

const tagLatest = "latest"

// tagAction is the result of processing a cached tag.
type tagAction int8

const (
	tagNotCached tagAction = iota // no cache entry found; fall through to live fetch
	tagContinue                   // cache hit handled; move to next tag
	tagStop                       // cache hit handled; stop scanning the repo
)

type imageLabels struct {
	Labels map[string]string `json:"Labels"`
}

type imageConfig struct {
	Config imageLabels `json:"config"`
}

// Discover enumerates all repositories and tags in the given registry, returning
// images that declare the org.openagentcontainers.version label.
//
// When WithForce() is not set, a repo whose `latest` tag lacks OAC labels is skipped
// entirely — all other tags are assumed to be non-conformant too.
// Use WithForce() to scan every tag regardless.
//
// Cache behaviour is configured via WithCache. When a cache is present, images with
// previously seen digests are not re-fetched, and repos confirmed non-OAC on the previous
// scan are skipped. See [Cache] for the nil-agentJSON convention used to record non-OAC results.
func Discover(ctx context.Context, registry string, opts Options) ([]oac.Image, error) {
	var repos []string

	err := withRetry(ctx, opts.limiter, opts.maxRetries, func() error {
		var e error

		repos, e = crane.Catalog(registry, opts.craneOpts...)

		return e
	})
	if err != nil {
		return nil, fmt.Errorf("catalog %s: %w", registry, err)
	}

	jobs := make(chan string, len(repos))
	results := make(chan oac.Image)

	var wg sync.WaitGroup

	for range opts.concurrency {
		wg.Go(func() {
			for repo := range jobs {
				scanRepo(
					ctx,
					registry+"/"+repo,
					opts.maxRetries,
					opts.force,
					opts.cache,
					opts.limiter,
					results,
					opts.craneOpts...,
				)
			}
		})
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for _, repo := range repos {
		jobs <- repo
	}

	close(jobs)

	var agents []oac.Image

	for a := range results {
		agents = append(agents, a)
	}

	return agents, nil
}

// repoScanner holds shared scan configuration to avoid threading bool flags through helpers.
type repoScanner struct {
	c          Cache
	limiter    *rate.Limiter
	opts       []crane.Option
	maxRetries int
	force      bool
}

func scanRepo(
	ctx context.Context,
	repo string,
	maxRetries int,
	force bool,
	c Cache,
	limiter *rate.Limiter,
	out chan<- oac.Image,
	opts ...crane.Option,
) {
	var tags []string

	rs := repoScanner{c: c, limiter: limiter, opts: opts, maxRetries: maxRetries, force: force}

	err := withRetry(ctx, limiter, maxRetries, func() error {
		var e error

		tags, e = crane.ListTags(repo, opts...)

		return e
	})
	if err != nil {
		return
	}

	tags = hoistLatest(tags)

	for i, tag := range tags {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if rs.processTag(ctx, repo, repo+":"+tag, tag, i, out) {
			return
		}
	}
}

// processTag handles a single tag within scanRepo. Returns true if the caller should stop iterating.
func (rs repoScanner) processTag(
	ctx context.Context,
	repo, ref, tag string,
	tagIndex int,
	out chan<- oac.Image,
) bool {
	digest := resolveDigest(ctx, rs.limiter, rs.maxRetries, ref, rs.opts)

	if tagIndex == 0 && tag == tagLatest {
		if rs.handleLatestTag(repo, digest) {
			return true
		}
	}

	action := handleCacheHit(ctx, rs.c, digest, ref, out)

	return rs.dispatchCacheResult(ctx, ref, digest, tagIndex, action, out)
}

// dispatchCacheResult routes to the correct follow-up action based on the cache result.
// Returns true if the caller should stop iterating.
func (rs repoScanner) dispatchCacheResult(
	ctx context.Context,
	ref, digest string,
	tagIndex int,
	action tagAction,
	out chan<- oac.Image,
) bool {
	switch action {
	case tagStop:
		return true
	case tagContinue:
		return tagIndex == 0 && !rs.force && shouldSkipNonOACLatest(rs.c, digest)
	case tagNotCached:
		return rs.processCacheMiss(ctx, ref, digest, tagIndex, out)
	default:
		return false
	}
}

// handleLatestTag processes the repo-level shortcut for the "latest" tag.
// Returns true if the entire repo should be skipped.
func (rs repoScanner) handleLatestTag(repo, digest string) bool {
	if !rs.force {
		return checkLatestShortcut(rs.c, repo, digest)
	}

	if digest != "" && rs.c != nil {
		rs.c.SetLatestDigest(repo, digest)
	}

	return false
}

// processCacheMiss fetches the image config and emits an agent if OAC-conformant.
// Returns true if the caller should stop iterating.
func (rs repoScanner) processCacheMiss(
	ctx context.Context,
	ref, digest string,
	tagIndex int,
	out chan<- oac.Image,
) bool {
	var agent oac.Image

	var ok bool

	err := withRetry(ctx, rs.limiter, rs.maxRetries, func() error {
		var e error

		agent, ok, e = inspectImage(ref, rs.opts...)

		return e
	})
	if err != nil {
		return tagIndex == 0 && !rs.force
	}

	var agentJSON []byte

	if ok {
		b, merr := json.Marshal(agent)
		if merr == nil {
			agentJSON = b
		}
	}

	storeCacheResult(rs.c, digest, agentJSON)

	if tagIndex == 0 && !rs.force && !ok {
		return true
	}

	return ok && emitAgent(ctx, agent, out)
}

// emitAgent sends agent to out, returning true if the caller should stop (context done).
func emitAgent(ctx context.Context, agent oac.Image, out chan<- oac.Image) bool {
	select {
	case out <- agent:
		return false
	case <-ctx.Done():
		return true
	}
}

// resolveDigest fetches the content-addressed digest for ref, returning empty string on failure.
func resolveDigest(
	ctx context.Context,
	limiter *rate.Limiter,
	maxRetries int,
	ref string,
	opts []crane.Option,
) string {
	var digest string

	err := withRetry(ctx, limiter, maxRetries, func() error {
		var e error

		digest, e = crane.Digest(ref, opts...)

		return e
	})
	if err != nil {
		return ""
	}

	return digest
}

// checkLatestShortcut implements the repo-level shortcut for the tagLatest tag.
// Returns true if the entire repo should be skipped (latest unchanged and confirmed non-OAC).
// Only call this when force scanning is disabled.
func checkLatestShortcut(c Cache, repo, digest string) bool {
	if digest == "" || c == nil {
		return false
	}

	if prior, found := c.GetLatestDigest(repo); found && prior == digest {
		if agentJSON, ok := c.GetDigest(digest); ok && agentJSON == nil {
			return true // latest unchanged and was confirmed non-OAC — skip repo
		}
	}

	c.SetLatestDigest(repo, digest)

	return false
}

// handleCacheHit checks the per-digest cache. Returns hit=true when a cache entry exists.
// If hit and the image was OAC, it emits to out.
// Returns tagNotCached when no cache entry exists, tagStop on context cancellation, tagContinue otherwise.
func handleCacheHit(
	ctx context.Context,
	c Cache,
	digest, ref string,
	out chan<- oac.Image,
) tagAction {
	if digest == "" || c == nil {
		return tagNotCached
	}

	agentJSON, found := c.GetDigest(digest)
	if !found {
		return tagNotCached
	}

	if agentJSON != nil {
		var agent oac.Image

		err := json.Unmarshal(agentJSON, &agent)
		if err == nil {
			agent.Reference = ref // ref may differ from the originally cached tag
			if emitAgent(ctx, agent, out) {
				return tagStop
			}
		}
	}

	return tagContinue
}

// shouldSkipNonOACLatest reports whether the cached digest is a confirmed non-OAC result,
// meaning the entire repo can be skipped.
func shouldSkipNonOACLatest(c Cache, digest string) bool {
	if digest == "" || c == nil {
		return false
	}

	agentJSON, found := c.GetDigest(digest)

	return found && agentJSON == nil
}

// storeCacheResult stores agentJSON in the cache keyed by digest.
// Pass nil agentJSON to record a confirmed non-OAC result.
func storeCacheResult(c Cache, digest string, agentJSON []byte) {
	if digest == "" || c == nil {
		return
	}

	c.SetDigest(digest, agentJSON)
}

// FetchLabels fetches the OCI image config for ref and returns all its labels. Use this for a
// single known image reference when you don't need registry-wide enumeration. ref may be any
// form accepted by crane: "registry/repo:tag", "registry/repo@sha256:...", etc.
// Pass the returned map to oac.Parse to decode OAC labels.
func FetchLabels(ref string, opts ...crane.Option) (map[string]string, error) {
	raw, err := crane.Config(ref, opts...)
	if err != nil {
		return nil, err
	}

	var cfg imageConfig

	err = json.Unmarshal(raw, &cfg)
	if err != nil {
		return nil, err
	}

	return cfg.Config.Labels, nil
}

func inspectImage(ref string, opts ...crane.Option) (oac.Image, bool, error) {
	labels, err := FetchLabels(ref, opts...)
	if err != nil {
		return oac.Image{}, false, err
	}

	if _, ok := labels[oac.LabelVersion]; !ok {
		return oac.Image{}, false, nil
	}

	manifest, err := oac.Parse(labels)
	if err != nil {
		return oac.Image{}, false, err
	}

	return oac.Image{
		Manifest:  *manifest,
		Reference: ref,
		Labels:    labels,
	}, true, nil
}

// hoistLatest moves the tagLatest tag to index 0 so it is inspected first.
// If tagLatest is absent the slice is returned unchanged.
func hoistLatest(tags []string) []string {
	for i, t := range tags {
		if t == tagLatest {
			out := make([]string, 0, len(tags))
			out = append(out, tagLatest)
			out = append(out, tags[:i]...)
			out = append(out, tags[i+1:]...)

			return out
		}
	}

	return tags
}

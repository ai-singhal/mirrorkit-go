package mirrorkit

import (
	"errors"
	"os"
	"strings"
	"time"
)

// DefaultEndpoint is the production backend. Override per-process via
// MIRRORKIT_ENDPOINT / MIRROR_ENDPOINT, or Options.Endpoint.
const DefaultEndpoint = "https://api.runmirrors.com"

var errMissingAPIKey = errors.New("mirrorkit: Options.APIKey is required")

// Options configures the collector. Only APIKey is required; the rest default.
type Options struct {
	// APIKey is the workspace collector key (mk_live_...), minted in the web app.
	APIKey string
	// Project is the logical trace stream this process feeds. Default "default".
	Project string
	// Endpoint is the backend base URL. Default: env var, then DefaultEndpoint.
	Endpoint string
	// FlushInterval is the time between background batch flushes. Default 2s.
	FlushInterval time.Duration
	// MaxBatch is the max traces per POST. Default 50.
	MaxBatch int
	// MaxQueue is the max traces buffered in memory; oldest dropped past this.
	// Default 10000.
	MaxQueue int
	// Debug emits internal debug logs. Default false (or MIRRORKIT_DEBUG=1).
	Debug bool
}

// config is the frozen, fully-defaulted form of Options.
type config struct {
	apiKey        string
	project       string
	endpoint      string
	flushInterval time.Duration
	maxBatch      int
	maxQueue      int
	debug         bool
}

// resolveConfig turns the loose Options into a defaulted config. Resolution
// order for endpoint/debug mirrors the sibling collectors (explicit arg → env
// var → built-in default).
func resolveConfig(o Options) (config, error) {
	if strings.TrimSpace(o.APIKey) == "" {
		return config{}, errMissingAPIKey
	}
	endpoint := o.Endpoint
	if endpoint == "" {
		endpoint = firstEnv("MIRRORKIT_ENDPOINT", "MIRROR_ENDPOINT")
	}
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	project := o.Project
	if project == "" {
		project = "default"
	}
	flush := o.FlushInterval
	if flush <= 0 {
		flush = 2 * time.Second
	}
	maxBatch := o.MaxBatch
	if maxBatch <= 0 {
		maxBatch = 50
	}
	maxQueue := o.MaxQueue
	if maxQueue <= 0 {
		maxQueue = 10000
	}
	debug := o.Debug
	if v := os.Getenv("MIRRORKIT_DEBUG"); v == "1" || v == "true" {
		debug = true
	}
	return config{
		apiKey:        o.APIKey,
		project:       project,
		endpoint:      strings.TrimRight(endpoint, "/"),
		flushInterval: flush,
		maxBatch:      maxBatch,
		maxQueue:      maxQueue,
		debug:         debug,
	}, nil
}

func firstEnv(names ...string) string {
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return ""
}

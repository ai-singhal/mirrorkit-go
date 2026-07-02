// Package mirrorkit is a lightweight, drop-in production trace collector for LLM
// agents — the Go sibling of the Python `mirrorkit` and TypeScript `mirrorkit-js`
// packages.
//
// It does exactly one thing: take traces you hand it and stream them to your
// Mirrors backend in the background — batched, non-blocking, and safe (it never
// panics into your app after Init). It does NOT instrument SDKs and does NOT run
// agents; building the trace is the caller's job.
//
//	mirrorkit.Init(mirrorkit.Options{APIKey: "mk_live_...", Project: "my-agent"})
//	defer mirrorkit.Shutdown(5 * time.Second)
//
//	mirrorkit.LogTrace([]mirrorkit.Message{
//		mirrorkit.SystemMessage("You are a helpful assistant."),
//		mirrorkit.UserMessage("What's the weather in Paris?"),
//		mirrorkit.AssistantMessage("It's 18C and sunny."),
//	}, mirrorkit.WithModel("claude-opus-4-8"))
package mirrorkit

import (
	"log"
	"sync"
	"time"
)

// Client is a collector instance. Most programs use the package-level singleton
// (Init + LogTrace/Flush/Shutdown), but a Client can be built and used directly
// via NewClient.
type Client struct {
	cfg    config
	sender *sender
}

var (
	mu        sync.Mutex
	singleton *Client
)

// Init initializes the package-level collector and returns it. Idempotent: a
// second call returns the existing client and ignores the new options.
//
// A collector must never break the host app, so Init fails open: on invalid
// options (e.g. a missing APIKey) it logs a one-line warning and returns a
// disabled no-op client — LogTrace/Flush/Shutdown then do nothing, and the
// singleton is left unset so a later valid Init can still take. Use NewClient if
// you want the error returned explicitly.
func Init(o Options) *Client {
	mu.Lock()
	defer mu.Unlock()
	if singleton != nil {
		return singleton
	}
	c, err := NewClient(o)
	if err != nil {
		log.Printf("[mirrorkit] disabled — %v", err)
		return &Client{} // no sender: LogTrace/Flush/Shutdown are no-ops
	}
	singleton = c
	return c
}

// NewClient builds and starts a collector without touching the package
// singleton. Returns an error if APIKey is missing.
func NewClient(o Options) (*Client, error) {
	cfg, err := resolveConfig(o)
	if err != nil {
		return nil, err
	}
	s := newSender(cfg)
	s.start()
	return &Client{cfg: cfg, sender: s}, nil
}

// traceOptions carry per-call overrides for LogTrace.
type traceOptions struct {
	traceID string
	model   string
}

// TraceOption customizes a single LogTrace call.
type TraceOption func(*traceOptions)

// WithTraceID sets an explicit trace id (else a random one is generated).
func WithTraceID(id string) TraceOption { return func(o *traceOptions) { o.traceID = id } }

// WithModel records a model label on the trace.
func WithModel(model string) TraceOption { return func(o *traceOptions) { o.model = model } }

// LogTrace enqueues one trace (OpenAI-style messages) on this client.
// Non-blocking; safe to call from any goroutine.
func (c *Client) LogTrace(messages []Message, opts ...TraceOption) {
	if c == nil || c.sender == nil {
		return
	}
	to := traceOptions{}
	for _, opt := range opts {
		opt(&to)
	}
	id := to.traceID
	if id == "" {
		id = genID()
	}
	t := Trace{ID: id, Messages: messages}
	if to.model != "" {
		t.Model = &to.model
	}
	c.sender.enqueue(t)
}

// Flush blocks until this client's queue drains (or timeout elapses).
func (c *Client) Flush(timeout time.Duration) {
	if c == nil || c.sender == nil {
		return
	}
	c.sender.flush(timeout)
}

// Shutdown stops this client, flushing first (bounded by timeout).
func (c *Client) Shutdown(timeout time.Duration) {
	if c == nil || c.sender == nil {
		return
	}
	c.sender.stop(timeout)
}

// --- package-level singleton convenience wrappers ---

// LogTrace enqueues a trace on the package-level collector. No-op before Init.
func LogTrace(messages []Message, opts ...TraceOption) {
	if c := current(); c != nil {
		c.LogTrace(messages, opts...)
	}
}

// Flush drains the package-level collector's queue (or until timeout). No-op
// before Init.
func Flush(timeout time.Duration) {
	if c := current(); c != nil {
		c.Flush(timeout)
	}
}

// Shutdown stops the package-level collector (flushing first) and clears it, so
// a later Init can start fresh. No-op before Init.
func Shutdown(timeout time.Duration) {
	mu.Lock()
	c := singleton
	singleton = nil
	mu.Unlock()
	if c != nil {
		c.Shutdown(timeout)
	}
}

func current() *Client {
	mu.Lock()
	defer mu.Unlock()
	return singleton
}

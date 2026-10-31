> **No longer maintained.** This collector was built for an earlier Mirrors product, and its default backend, `api.runmirrors.com`, no longer exists. For the current SDK, see the [Mirrors quickstart](https://mirrors.dev/docs/quickstart).

# mirrorkit (Go)

A lightweight, **drop-in** production trace collector for LLM agents — the Go
sibling of the Python [`mirrorkit`](https://pypi.org/project/mirrorkit/) and
TypeScript `mirrorkit` (npm) packages.

It does exactly one thing: **take traces you hand it and send them to your
Mirrors backend** — batched in the background, non-blocking, and safe (it never
panics into your app after `Init`). It does **not** instrument SDKs and does
**not** run agents; you build the trace, it ships it.

## Install

```bash
go get github.com/ai-singhal/mirrorkit-go
```

Requires **Go ≥ 1.21**. Zero dependencies (standard library only).

## Usage

```go
package main

import (
	"time"

	mirrorkit "github.com/ai-singhal/mirrorkit-go"
)

func main() {
	mirrorkit.Init(mirrorkit.Options{APIKey: "mk_live_...", Project: "my-agent"})
	defer mirrorkit.Shutdown(5 * time.Second)

	mirrorkit.LogTrace([]mirrorkit.Message{
		mirrorkit.SystemMessage("You are a helpful assistant."),
		mirrorkit.UserMessage("What's the weather in Paris?"),
		mirrorkit.AssistantMessage("It's 18C and sunny."),
	}, mirrorkit.WithModel("claude-opus-4-8"))
}
```

Each trace is enqueued and flushed to `{endpoint}/api/collect` by a background
goroutine (`Authorization: Bearer <apiKey>`, body `{project, traces}`).

### Tool calls

Build tool-call and tool-result messages with the helpers:

```go
mirrorkit.LogTrace([]mirrorkit.Message{
	mirrorkit.UserMessage("Cancel booking BK-4471."),
	mirrorkit.AssistantToolCalls(
		mirrorkit.NewToolCall("call_1", "cancel_booking", map[string]any{"id": "BK-4471"}),
	),
	mirrorkit.ToolResult("call_1", `{"status":"cancelled"}`),
	mirrorkit.AssistantMessage("Done — BK-4471 is cancelled."),
})
```

## Options

```go
mirrorkit.Init(mirrorkit.Options{
	APIKey:        "mk_live_...",           // required
	Project:       "my-agent",              // default "default"
	Endpoint:      "https://api.mirror.dev", // else MIRRORKIT_ENDPOINT / MIRROR_ENDPOINT / prod
	FlushInterval: 2 * time.Second,         // ms between background flushes
	MaxBatch:      50,                      // max traces per POST
	MaxQueue:      10_000,                  // in-memory cap; oldest dropped past this
	Debug:         false,                   // internal debug logs (or MIRRORKIT_DEBUG=1)
})
```

Endpoint resolution: `Options.Endpoint` → `MIRRORKIT_ENDPOINT` → `MIRROR_ENDPOINT`
→ the production default.

## Lifecycle

```go
mirrorkit.Flush(5 * time.Second)     // block until the queue drains
mirrorkit.Shutdown(5 * time.Second)  // stop the background goroutine (flushes first)
```

Prefer one collector per process via the package-level `Init` / `LogTrace` /
`Flush` / `Shutdown`. For multiple independent collectors, use `NewClient` and
call `LogTrace` / `Flush` / `Shutdown` on the returned `*Client`.

## Testing

```bash
cd mirrorkit-go && go vet ./... && go test ./...
```

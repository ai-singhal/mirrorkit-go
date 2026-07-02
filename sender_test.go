package mirrorkit

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSenderPostsBatchWithAuthAndBody(t *testing.T) {
	var mu sync.Mutex
	var batches []CollectBatch
	var auth, ctype string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var b CollectBatch
		_ = json.Unmarshal(body, &b)
		mu.Lock()
		batches = append(batches, b)
		auth = r.Header.Get("Authorization")
		ctype = r.Header.Get("Content-Type")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, err := NewClient(Options{
		APIKey: "mk_test", Project: "p", Endpoint: srv.URL,
		FlushInterval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.Shutdown(time.Second)

	c.LogTrace([]Message{UserMessage("hi"), AssistantMessage("yo")}, WithModel("m"))
	c.Flush(2 * time.Second)

	mu.Lock()
	defer mu.Unlock()
	if len(batches) == 0 {
		t.Fatal("no batch received")
	}
	if auth != "Bearer mk_test" {
		t.Fatalf("auth = %q", auth)
	}
	if ctype != "application/json" {
		t.Fatalf("content-type = %q", ctype)
	}
	b := batches[0]
	if b.Project != "p" {
		t.Fatalf("project = %q", b.Project)
	}
	if len(b.Traces) != 1 || len(b.Traces[0].Messages) != 2 {
		t.Fatalf("unexpected traces: %+v", b.Traces)
	}
	if b.Traces[0].Model == nil || *b.Traces[0].Model != "m" {
		t.Fatalf("model = %+v", b.Traces[0].Model)
	}
	if b.Traces[0].ID == "" {
		t.Fatal("trace id not set")
	}
}

func TestEnqueueDropsOldestWhenFull(t *testing.T) {
	s := newSender(config{maxQueue: 2, maxBatch: 50, project: "p", endpoint: "http://x"})
	s.enqueue(Trace{ID: "a"})
	s.enqueue(Trace{ID: "b"})
	s.enqueue(Trace{ID: "c"}) // over cap -> drops "a"
	if len(s.queue) != 2 || s.queue[0].ID != "b" || s.queue[1].ID != "c" {
		t.Fatalf("queue = %+v", s.queue)
	}
}

func TestClientErrorNotRetried(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest) // 400
	}))
	defer srv.Close()

	s := newSender(config{project: "p", endpoint: srv.URL, maxBatch: 50, maxQueue: 10})
	s.post([]Trace{{ID: "x"}})
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 call (no retry on 4xx), got %d", got)
	}
}

func TestRetriesOn5xxThenSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError) // fail once
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := newSender(config{project: "p", endpoint: srv.URL, maxBatch: 50, maxQueue: 10})
	s.post([]Trace{{ID: "x"}})
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 calls (1 retry), got %d", got)
	}
}

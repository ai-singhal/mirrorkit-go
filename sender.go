package mirrorkit

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	maxRetries = 2
	backoff    = 500 * time.Millisecond
)

// sender is the background, non-blocking transport — the collector's whole job.
// enqueue is O(1) and never blocks; a ticker goroutine drains the queue and
// POSTs batches to /api/collect. Every rule here serves "never break or slow the
// host app":
//
//   - enqueue drops the oldest trace once the buffer passes maxQueue.
//   - network failures are retried a couple of times, then the batch is dropped.
//   - all errors are swallowed (logged at debug); nothing propagates to the caller.
type sender struct {
	cfg    config
	client *http.Client
	log    *logger

	mu      sync.Mutex
	queue   []Trace
	posting bool
	stopped bool

	ticker *time.Ticker
	done   chan struct{}
	wg     sync.WaitGroup
}

func newSender(cfg config) *sender {
	return &sender{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
		log:    &logger{enabled: cfg.debug},
		done:   make(chan struct{}),
	}
}

// start launches the background flush goroutine (idempotent).
func (s *sender) start() {
	s.mu.Lock()
	if s.ticker != nil || s.stopped {
		s.mu.Unlock()
		return
	}
	s.ticker = time.NewTicker(s.cfg.flushInterval)
	s.mu.Unlock()
	s.wg.Add(1)
	go s.loop()
}

func (s *sender) loop() {
	defer s.wg.Done()
	for {
		select {
		case <-s.done:
			return
		case <-s.ticker.C:
			s.drainOnce()
		}
	}
}

// enqueue is non-blocking. Drops the oldest trace if the buffer is full.
func (s *sender) enqueue(t Trace) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	if len(s.queue) >= s.cfg.maxQueue {
		s.queue = s.queue[1:]
		s.log.debugf("queue full, dropping oldest trace")
	}
	s.queue = append(s.queue, t)
}

// drainOnce drains and POSTs a single batch. Guarded so the ticker and an
// explicit flush never post concurrently.
func (s *sender) drainOnce() {
	s.mu.Lock()
	if s.posting || len(s.queue) == 0 {
		s.mu.Unlock()
		return
	}
	s.posting = true
	n := s.cfg.maxBatch
	if n > len(s.queue) {
		n = len(s.queue)
	}
	batch := make([]Trace, n)
	copy(batch, s.queue[:n])
	s.queue = s.queue[n:]
	s.mu.Unlock()

	s.post(batch)

	s.mu.Lock()
	s.posting = false
	s.mu.Unlock()
}

// flush blocks until the queue drains (or timeout elapses).
func (s *sender) flush(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for {
		s.mu.Lock()
		idle := len(s.queue) == 0 && !s.posting
		s.mu.Unlock()
		if idle {
			return
		}
		if time.Now().After(deadline) {
			s.log.debugf("flush timed out with traces still queued")
			return
		}
		s.drainOnce()
		s.mu.Lock()
		remaining := len(s.queue)
		s.mu.Unlock()
		if remaining > 0 {
			time.Sleep(20 * time.Millisecond)
		}
	}
}

// stop halts the ticker goroutine and flushes whatever is left.
func (s *sender) stop(timeout time.Duration) {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	started := s.ticker != nil
	if s.ticker != nil {
		s.ticker.Stop()
	}
	s.mu.Unlock()
	if started {
		close(s.done)
		s.wg.Wait()
	}
	s.flush(timeout)
}

func (s *sender) post(traces []Trace) {
	if len(traces) == 0 {
		return
	}
	payload, err := json.Marshal(CollectBatch{Project: s.cfg.project, Traces: traces})
	if err != nil {
		s.log.debugf("marshal failed: %v", err)
		return
	}
	url := s.cfg.endpoint + "/api/collect"
	for attempt := 0; attempt <= maxRetries; attempt++ {
		status, err := s.doPost(url, payload)
		if err == nil {
			if status >= 200 && status < 300 {
				return
			}
			s.log.debugf("non-2xx %d from /api/collect", status)
			if status >= 400 && status < 500 {
				return // client error — retrying won't help; drop it
			}
		} else {
			s.log.debugf("send failed: %v", err)
		}
		if attempt < maxRetries {
			time.Sleep(backoff * time.Duration(attempt+1))
		}
	}
	s.log.debugf("dropping batch of %d trace(s) after retries", len(traces))
}

func (s *sender) doPost(url string, payload []byte) (int, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body) // drain so the connection can be reused
	return resp.StatusCode, nil
}

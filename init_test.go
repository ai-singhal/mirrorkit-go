package mirrorkit

import (
	"testing"
	"time"
)

func TestInitFailsOpenOnMissingKey(t *testing.T) {
	// A missing APIKey must not panic — Init returns a usable no-op client and
	// leaves the package singleton unset.
	c := Init(Options{})
	if c == nil {
		t.Fatal("Init returned nil; want a no-op client")
	}
	// none of these may panic on the disabled client
	c.LogTrace([]Message{UserMessage("hi")}, WithModel("m"))
	c.Flush(time.Millisecond)
	c.Shutdown(time.Millisecond)

	if current() != nil {
		t.Fatal("singleton should not be set after a failed Init")
	}

	// package-level wrappers are also no-ops before a successful Init
	LogTrace([]Message{UserMessage("hi")})
	Flush(time.Millisecond)
	Shutdown(time.Millisecond)
}

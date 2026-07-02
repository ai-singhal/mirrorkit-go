package mirrorkit

import (
	"testing"
	"time"
)

func TestResolveDefaults(t *testing.T) {
	t.Setenv("MIRRORKIT_ENDPOINT", "")
	t.Setenv("MIRROR_ENDPOINT", "")
	t.Setenv("MIRRORKIT_DEBUG", "")

	cfg, err := resolveConfig(Options{APIKey: "k"})
	if err != nil {
		t.Fatalf("resolveConfig: %v", err)
	}
	if cfg.project != "default" {
		t.Fatalf("project = %q", cfg.project)
	}
	if cfg.endpoint != DefaultEndpoint {
		t.Fatalf("endpoint = %q", cfg.endpoint)
	}
	if cfg.flushInterval != 2*time.Second {
		t.Fatalf("flushInterval = %v", cfg.flushInterval)
	}
	if cfg.maxBatch != 50 || cfg.maxQueue != 10000 {
		t.Fatalf("maxBatch=%d maxQueue=%d", cfg.maxBatch, cfg.maxQueue)
	}
	if cfg.debug {
		t.Fatal("debug should default false")
	}
}

func TestResolveRequiresAPIKey(t *testing.T) {
	if _, err := resolveConfig(Options{}); err == nil {
		t.Fatal("expected error when APIKey is missing")
	}
}

func TestResolveEndpointPrecedenceAndTrim(t *testing.T) {
	t.Setenv("MIRROR_ENDPOINT", "")
	t.Setenv("MIRRORKIT_ENDPOINT", "https://env-endpoint/")

	cfg, err := resolveConfig(Options{APIKey: "k"})
	if err != nil {
		t.Fatalf("resolveConfig: %v", err)
	}
	if cfg.endpoint != "https://env-endpoint" {
		t.Fatalf("env endpoint = %q", cfg.endpoint)
	}

	// an explicit option wins over the env var; trailing slashes are trimmed
	cfg2, err := resolveConfig(Options{APIKey: "k", Endpoint: "https://explicit//"})
	if err != nil {
		t.Fatalf("resolveConfig: %v", err)
	}
	if cfg2.endpoint != "https://explicit" {
		t.Fatalf("explicit endpoint = %q", cfg2.endpoint)
	}
}

func TestResolveDebugFromEnv(t *testing.T) {
	t.Setenv("MIRRORKIT_DEBUG", "1")
	cfg, err := resolveConfig(Options{APIKey: "k"})
	if err != nil {
		t.Fatalf("resolveConfig: %v", err)
	}
	if !cfg.debug {
		t.Fatal("debug should be true when MIRRORKIT_DEBUG=1")
	}
}

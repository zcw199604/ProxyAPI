package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCodexPiUpstreamParityConfig(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("codex:\n  disable-pi-upstream-parity: true\n"), &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if !cfg.Codex.DisablePiUpstreamParity {
		t.Fatal("disable-pi-upstream-parity was not decoded")
	}
}

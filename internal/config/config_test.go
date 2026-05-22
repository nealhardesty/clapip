package config

import (
	"testing"
)

func TestParseDefaults(t *testing.T) {
	cfg, err := Parse(nil, func(string) string { return "" })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != DefaultPort {
		t.Errorf("Port = %d, want %d", cfg.Port, DefaultPort)
	}
	if cfg.Model != DefaultModel {
		t.Errorf("Model = %q, want %q", cfg.Model, DefaultModel)
	}
	if cfg.ClaudePath != DefaultClaudePath {
		t.Errorf("ClaudePath = %q, want %q", cfg.ClaudePath, DefaultClaudePath)
	}
	if cfg.APIKey != "" {
		t.Errorf("APIKey = %q, want empty", cfg.APIKey)
	}
	if cfg.Host != DefaultHost {
		t.Errorf("Host = %q, want %q", cfg.Host, DefaultHost)
	}
}

func TestParseBindAll(t *testing.T) {
	for _, args := range [][]string{{"--bind-all"}, {"-a"}} {
		cfg, err := Parse(args, func(string) string { return "" })
		if err != nil {
			t.Fatalf("%v: unexpected error: %v", args, err)
		}
		if cfg.Host != AllInterfacesHost {
			t.Errorf("%v: Host = %q, want %q", args, cfg.Host, AllInterfacesHost)
		}
	}
}

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"long flags", []string{"--port", "9001", "--model", "opus", "--claude-path", "/bin/claude", "--api-key", "secret"}},
		{"short flags", []string{"-p", "9001", "-m", "opus", "-c", "/bin/claude", "-k", "secret"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Parse(tt.args, func(string) string { return "" })
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Port != 9001 || cfg.Model != "opus" ||
				cfg.ClaudePath != "/bin/claude" || cfg.APIKey != "secret" {
				t.Errorf("unexpected config: %+v", cfg)
			}
		})
	}
}

func TestParseAPIKeyPrecedence(t *testing.T) {
	env := func(k string) string {
		if k == EnvAPIKey {
			return "env-key"
		}
		return ""
	}

	// Flag wins over the environment.
	cfg, err := Parse([]string{"-k", "flag-key"}, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "flag-key" {
		t.Errorf("APIKey = %q, want flag-key", cfg.APIKey)
	}

	// Environment is used when the flag is absent.
	cfg, err = Parse(nil, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "env-key" {
		t.Errorf("APIKey = %q, want env-key", cfg.APIKey)
	}
}

func TestParseInvalidPort(t *testing.T) {
	for _, p := range []string{"0", "70000", "-1"} {
		if _, err := Parse([]string{"-p", p}, nil); err == nil {
			t.Errorf("port %s: expected error, got nil", p)
		}
	}
}

func TestParseVersionFlag(t *testing.T) {
	cfg, err := Parse([]string{"--version"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.ShowVersion {
		t.Error("ShowVersion = false, want true")
	}
}

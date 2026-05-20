// Package config parses clapip's command-line flags and environment-variable
// fallbacks into a single immutable Config value.
package config

import (
	"flag"
	"fmt"
)

// Default flag values.
const (
	DefaultPort       = 8999
	DefaultModel      = "sonnet"
	DefaultClaudePath = "claude"

	// EnvAPIKey is the environment variable consulted when -k is not given.
	EnvAPIKey = "PROXY_API_KEY"
)

// Config holds the fully resolved runtime configuration.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port int
	// Model is the default claude model used when a request omits one.
	Model string
	// ClaudePath is the path to (or PATH-resolvable name of) the claude binary.
	ClaudePath string
	// APIKey, when non-empty, is the bearer token required from clients.
	APIKey string
	// ShowVersion indicates the -v/--version flag was supplied.
	ShowVersion bool
}

// Parse resolves configuration from the given argument list. The API key
// precedence is: -k flag, then the PROXY_API_KEY environment variable, then
// empty (an open proxy). getenv is injected to keep Parse testable.
func Parse(args []string, getenv func(string) string) (Config, error) {
	fs := flag.NewFlagSet("clapip", flag.ContinueOnError)
	var cfg Config

	// Each option registers a long and a short name pointing at the same
	// destination, so whichever the client supplies wins.
	fs.IntVar(&cfg.Port, "port", DefaultPort, "Port to listen on")
	fs.IntVar(&cfg.Port, "p", DefaultPort, "Port to listen on (shorthand)")
	fs.StringVar(&cfg.Model, "model", DefaultModel, "Default claude model")
	fs.StringVar(&cfg.Model, "m", DefaultModel, "Default claude model (shorthand)")
	fs.StringVar(&cfg.ClaudePath, "claude-path", DefaultClaudePath, "Path to the claude CLI binary")
	fs.StringVar(&cfg.ClaudePath, "c", DefaultClaudePath, "Path to the claude CLI binary (shorthand)")
	fs.StringVar(&cfg.APIKey, "api-key", "", "Bearer token required from clients")
	fs.StringVar(&cfg.APIKey, "k", "", "Bearer token required from clients (shorthand)")
	fs.BoolVar(&cfg.ShowVersion, "version", false, "Print version and exit")
	fs.BoolVar(&cfg.ShowVersion, "v", false, "Print version and exit (shorthand)")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	if cfg.APIKey == "" && getenv != nil {
		cfg.APIKey = getenv(EnvAPIKey)
	}

	if cfg.Port < 1 || cfg.Port > 65535 {
		return Config{}, fmt.Errorf("invalid port %d: must be between 1 and 65535", cfg.Port)
	}

	return cfg, nil
}

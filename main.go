// Command clapip is a lightweight Go proxy that exposes an OpenAI-compatible
// HTTP API but executes inferences by wrapping the Anthropic claude CLI as a
// local subprocess.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nealhardesty/clapip/internal/claude"
	"github.com/nealhardesty/clapip/internal/config"
	"github.com/nealhardesty/clapip/internal/server"
)

func main() {
	cfg, err := config.Parse(os.Args[1:], os.Getenv)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		log.Fatalf("clapip: %v", err)
	}
	if cfg.ShowVersion {
		fmt.Printf("clapip %s\n", Version)
		return
	}

	runner := claude.New(cfg.ClaudePath, cfg.Model)

	// Startup sanity check: a missing or broken claude CLI is fatal.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	if verr := runner.Verify(ctx); verr != nil {
		cancel()
		log.Fatalf("clapip: claude CLI verification failed: %v", verr)
	}
	cancel()

	srv := server.New(cfg, runner, Version)
	host := cfg.Host
	if host == config.AllInterfacesHost {
		host = "0.0.0.0"
	}
	log.Printf("clapip %s listening on %s:%d (model=%s, claude=%s, auth=%t)",
		Version, host, cfg.Port, cfg.Model, cfg.ClaudePath, cfg.APIKey != "")
	if err := srv.Run(); err != nil {
		log.Fatalf("clapip: %v", err)
	}
}

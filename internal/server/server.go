// Package server implements clapip's OpenAI-compatible HTTP surface.
package server

import (
	"context"
	"crypto/subtle"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/nealhardesty/clapip/internal/claude"
	"github.com/nealhardesty/clapip/internal/config"
)

// Server wires configuration, the claude runner, and HTTP handlers together.
type Server struct {
	cfg     config.Config
	runner  *claude.Runner
	version string
}

// New constructs a Server.
func New(cfg config.Config, runner *claude.Runner, version string) *Server {
	return &Server{cfg: cfg, runner: runner, version: version}
}

// Handler returns the fully routed HTTP handler with request logging applied.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/v1/models", s.requireAuth(s.handleModels))
	mux.HandleFunc("/v1/chat/completions", s.requireAuth(s.handleChatCompletions))
	return logging(mux)
}

// Run starts the HTTP server and blocks until an interrupt or SIGTERM triggers
// a graceful shutdown.
func (s *Server) Run() error {
	srv := &http.Server{
		Addr:              net.JoinHostPort("", strconv.Itoa(s.cfg.Port)),
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-stop:
		log.Println("clapip: shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}

// requireAuth wraps a handler with bearer-token authentication when an API key
// is configured. The token comparison is constant-time.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.APIKey != "" {
			const prefix = "Bearer "
			header := r.Header.Get("Authorization")
			token := strings.TrimPrefix(header, prefix)
			if !strings.HasPrefix(header, prefix) ||
				subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.APIKey)) != 1 {
				writeError(w, http.StatusUnauthorized, "missing or invalid API key", "invalid_api_key")
				return
			}
		}
		next(w, r)
	}
}

// logging records the method, path, and duration of each request.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

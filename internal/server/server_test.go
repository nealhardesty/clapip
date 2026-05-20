package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nealhardesty/clapip/internal/claude"
	"github.com/nealhardesty/clapip/internal/config"
	"github.com/nealhardesty/clapip/internal/openai"
)

// fakeClaude writes an executable shell script standing in for the claude CLI.
func fakeClaude(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func newTestServer(t *testing.T, claudeBody, apiKey string) http.Handler {
	t.Helper()
	runner := claude.New(fakeClaude(t, claudeBody), "sonnet")
	cfg := config.Config{Port: config.DefaultPort, Model: "sonnet", APIKey: apiKey}
	return New(cfg, runner, "v0.0.0-test").Handler()
}

func TestHealth(t *testing.T) {
	h := newTestServer(t, "true", "")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"version":"v0.0.0-test"`) {
		t.Errorf("body missing version: %s", rec.Body.String())
	}
}

func TestModels(t *testing.T) {
	h := newTestServer(t, "true", "")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var list openai.ModelList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list.Data) != len(openai.SupportedModels) {
		t.Errorf("got %d models, want %d", len(list.Data), len(openai.SupportedModels))
	}
}

func TestAuth(t *testing.T) {
	h := newTestServer(t, "true", "secret")

	// Missing header is rejected.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no auth: status = %d, want 401", rec.Code)
	}

	// Wrong token is rejected.
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("bad auth: status = %d, want 401", rec.Code)
	}

	// Correct token is accepted.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("good auth: status = %d, want 200", rec.Code)
	}
}

func TestChatCompletionsJSON(t *testing.T) {
	h := newTestServer(t, `echo "hello from claude"`, "")
	body := `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp openai.ChatCompletionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := resp.Choices[0].Message.Content.Text; got != "hello from claude" {
		t.Errorf("content = %q, want %q", got, "hello from claude")
	}
	if resp.Object != "chat.completion" {
		t.Errorf("object = %q, want chat.completion", resp.Object)
	}
}

func TestChatCompletionsStreaming(t *testing.T) {
	h := newTestServer(t, `echo "streamed text"`, "")
	body := `{"messages":[{"role":"user","content":"hi"}],"stream":true}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	out := rec.Body.String()
	if !strings.Contains(out, "streamed text") {
		t.Errorf("stream missing content: %s", out)
	}
	if !strings.Contains(out, "chat.completion.chunk") {
		t.Errorf("stream missing chunk object: %s", out)
	}
	if !strings.Contains(out, "data: [DONE]") {
		t.Errorf("stream missing [DONE] sentinel: %s", out)
	}
}

func TestChatCompletionsError(t *testing.T) {
	h := newTestServer(t, `echo "needs re-auth" >&2; exit 1`, "")
	body := `{"messages":[{"role":"user","content":"hi"}]}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "needs re-auth") {
		t.Errorf("error body missing stderr: %s", rec.Body.String())
	}
}

func TestChatCompletionsBadRequest(t *testing.T) {
	h := newTestServer(t, "true", "")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("not json")))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad json: status = %d, want 400", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"messages":[]}`)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("empty messages: status = %d, want 400", rec.Code)
	}
}

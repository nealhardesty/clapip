package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nealhardesty/clapip/internal/openai"
)

// handleHealth reports liveness and dynamically verifies the claude binary is
// still reachable, returning 503 when it is not.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request")
		return
	}
	if err := s.runner.Available(); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error(), "service_unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": s.version,
	})
}

// handleModels returns the hardcoded list of supported claude models.
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request")
		return
	}
	now := time.Now().Unix()
	list := openai.ModelList{Object: "list"}
	for _, id := range openai.SupportedModels {
		list.Data = append(list.Data, openai.Model{
			ID:      id,
			Object:  "model",
			Created: now,
			OwnedBy: "anthropic",
		})
	}
	writeJSON(w, http.StatusOK, list)
}

// handleChatCompletions is the core inference endpoint. It translates an
// OpenAI request into a claude CLI invocation and streams or buffers the
// result back depending on the stream flag.
func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request")
		return
	}
	var req openai.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error(), "invalid_request")
		return
	}
	if len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "messages array is required", "invalid_request")
		return
	}

	model := openai.MapModel(req.Model, s.runner.DefaultModel)
	tools := openai.MapTools(req.Tools)
	prompt := openai.MergePrompt(req.Messages)

	if req.Stream {
		s.streamCompletion(w, r, model, tools, prompt)
		return
	}
	s.jsonCompletion(w, r, model, tools, prompt)
}

// jsonCompletion runs claude to completion and returns a single JSON object.
func (s *Server) jsonCompletion(w http.ResponseWriter, r *http.Request, model string, tools []string, prompt string) {
	cmd := s.runner.Command(r.Context(), model, tools, prompt)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if r.Context().Err() != nil {
			return // client disconnected; the subprocess was already killed
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		writeError(w, http.StatusInternalServerError, msg, "upstream_error")
		return
	}

	resp := openai.ChatCompletionResponse{
		ID:      newID(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []openai.Choice{{
			Index: 0,
			Message: openai.Message{
				Role:    "assistant",
				Content: openai.Content{Text: strings.TrimRight(stdout.String(), "\n")},
			},
			FinishReason: "stop",
		}},
	}
	writeJSON(w, http.StatusOK, resp)
}

// streamCompletion runs claude and relays its stdout as Server-Sent Events in
// the OpenAI chat.completion.chunk format.
func (s *Server) streamCompletion(w http.ResponseWriter, r *http.Request, model string, tools []string, prompt string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported by server", "server_error")
		return
	}

	cmd := s.runner.Command(r.Context(), model, tools, prompt)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "server_error")
		return
	}
	if err := cmd.Start(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "server_error")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	id := newID()
	created := time.Now().Unix()
	writeChunk(w, flusher, id, model, created, openai.Delta{Role: "assistant"}, nil)

	buf := make([]byte, 4096)
	for {
		n, rerr := stdout.Read(buf)
		if n > 0 {
			writeChunk(w, flusher, id, model, created, openai.Delta{Content: string(buf[:n])}, nil)
		}
		if rerr != nil {
			break
		}
	}

	// Surface a non-zero exit (e.g. the CLI needing re-authentication) as an
	// inline delta, since the SSE response headers are already committed.
	if werr := cmd.Wait(); werr != nil && r.Context().Err() == nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = werr.Error()
		}
		writeChunk(w, flusher, id, model, created,
			openai.Delta{Content: "\n\n[clapip error] " + msg}, nil)
	}

	stop := "stop"
	writeChunk(w, flusher, id, model, created, openai.Delta{}, &stop)
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// writeChunk marshals and flushes a single SSE chat.completion.chunk event.
func writeChunk(w http.ResponseWriter, f http.Flusher, id, model string, created int64, delta openai.Delta, finish *string) {
	chunk := openai.ChatCompletionChunk{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   model,
		Choices: []openai.ChunkChoice{{Index: 0, Delta: delta, FinishReason: finish}},
	}
	data, err := json.Marshal(chunk)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	f.Flush()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message, code string) {
	errType := "invalid_request_error"
	if status >= 500 {
		errType = "server_error"
	}
	writeJSON(w, status, openai.ErrorResponse{
		Error: openai.ErrorDetail{Message: message, Type: errType, Code: code},
	})
}

// newID returns an OpenAI-style completion identifier.
func newID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "chatcmpl-clapip"
	}
	return "chatcmpl-" + hex.EncodeToString(b)
}

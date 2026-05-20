// Package openai defines the OpenAI-compatible request and response types
// together with the translation helpers that map them onto claude CLI
// invocations.
package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Content holds message text while tolerating both the plain-string form and
// the structured content-parts array form that OpenAI clients may send.
type Content struct {
	Text string
}

// UnmarshalJSON accepts a JSON string, null, or an array of content parts.
// For the array form the text of every part is concatenated.
func (c *Content) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		c.Text = ""
		return nil
	}
	switch data[0] {
	case '"':
		return json.Unmarshal(data, &c.Text)
	case '[':
		var parts []struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(data, &parts); err != nil {
			return err
		}
		var b strings.Builder
		for _, p := range parts {
			b.WriteString(p.Text)
		}
		c.Text = b.String()
		return nil
	default:
		return fmt.Errorf("unsupported message content encoding")
	}
}

// MarshalJSON always emits the plain-string form.
func (c Content) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.Text)
}

// Message is a single chat message in OpenAI format.
type Message struct {
	Role    string  `json:"role"`
	Content Content `json:"content"`
}

// ToolFunction names a tool inside an OpenAI tool definition.
type ToolFunction struct {
	Name string `json:"name"`
}

// Tool is an OpenAI tool definition supplied by IDE clients.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ChatCompletionRequest is the POST /v1/chat/completions request body.
type ChatCompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	Tools    []Tool    `json:"tools"`
}

// Usage reports token accounting. The claude CLI does not expose token counts
// in print mode, so clapip reports zeros for OpenAI client compatibility.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Choice is a non-streaming completion choice.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// ChatCompletionResponse is the non-streaming response body.
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Delta is the incremental payload of a streaming chunk.
type Delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// ChunkChoice is a streaming completion choice.
type ChunkChoice struct {
	Index        int     `json:"index"`
	Delta        Delta   `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

// ChatCompletionChunk is one SSE event in a streaming response.
type ChatCompletionChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
}

// Model is one entry in the GET /v1/models listing.
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelList is the GET /v1/models response body.
type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// ErrorDetail describes a single error.
type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

// ErrorResponse is the OpenAI-compatible error envelope.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// SupportedModels lists the claude CLI model aliases exposed by /v1/models.
var SupportedModels = []string{"sonnet", "opus", "haiku"}

var modelAliases = map[string]string{
	"sonnet": "sonnet", "claude-sonnet": "sonnet", "claude-3-5-sonnet": "sonnet",
	"opus": "opus", "claude-opus": "opus", "claude-3-opus": "opus",
	"haiku": "haiku", "claude-haiku": "haiku", "claude-3-haiku": "haiku",
}

// MapModel resolves a client-supplied model name to a claude CLI model alias.
// It falls back to def when the request omits a model or names an unknown one.
func MapModel(requested, def string) string {
	lr := strings.ToLower(strings.TrimSpace(requested))
	if lr == "" {
		return def
	}
	if v, ok := modelAliases[lr]; ok {
		return v
	}
	switch {
	case strings.Contains(lr, "opus"):
		return "opus"
	case strings.Contains(lr, "haiku"):
		return "haiku"
	case strings.Contains(lr, "sonnet"):
		return "sonnet"
	default:
		return def
	}
}

var toolAliases = map[string]string{
	"run_command": "Bash", "execute_command": "Bash", "bash": "Bash",
	"terminal": "Bash", "shell": "Bash",
	"read_file": "Read", "read": "Read", "view_file": "Read",
	"edit_file": "Edit", "edit": "Edit", "apply_diff": "Edit", "apply_patch": "Edit",
	"write_file": "Write", "create_file": "Write", "write": "Write",
}

// MapTools translates OpenAI tool definitions into the set of native claude
// allowed-tool names. Unrecognized tools are ignored to keep the proxy
// stateless. The returned slice is sorted and deduplicated.
func MapTools(tools []Tool) []string {
	set := make(map[string]struct{})
	for _, t := range tools {
		name := strings.ToLower(strings.TrimSpace(t.Function.Name))
		if cli, ok := toolAliases[name]; ok {
			set[cli] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// MergePrompt flattens an OpenAI messages array into a single role-labelled
// string suitable for passing to the claude CLI as a positional argument.
func MergePrompt(messages []Message) string {
	var b strings.Builder
	for i, m := range messages {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(roleLabel(m.Role))
		b.WriteString(": ")
		b.WriteString(m.Content.Text)
	}
	return b.String()
}

func roleLabel(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "system":
		return "System"
	case "assistant":
		return "Assistant"
	case "tool":
		return "Tool"
	default:
		return "User"
	}
}

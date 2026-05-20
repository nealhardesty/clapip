package openai

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestContentUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"string", `"hello"`, "hello"},
		{"null", `null`, ""},
		{"empty array", `[]`, ""},
		{"parts array", `[{"type":"text","text":"foo"},{"type":"text","text":"bar"}]`, "foobar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Content
			if err := json.Unmarshal([]byte(tt.in), &c); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if c.Text != tt.want {
				t.Errorf("Text = %q, want %q", c.Text, tt.want)
			}
		})
	}
}

func TestContentMarshal(t *testing.T) {
	b, err := json.Marshal(Content{Text: "hi"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != `"hi"` {
		t.Errorf("marshal = %s, want \"hi\"", b)
	}
}

func TestMessageRoundTrip(t *testing.T) {
	in := `{"role":"user","content":[{"type":"text","text":"hey"}]}`
	var m Message
	if err := json.Unmarshal([]byte(in), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Content.Text != "hey" {
		t.Errorf("Content.Text = %q, want hey", m.Content.Text)
	}
}

func TestMapModel(t *testing.T) {
	tests := []struct {
		requested, def, want string
	}{
		{"", "sonnet", "sonnet"},
		{"opus", "sonnet", "opus"},
		{"claude-3-5-sonnet", "haiku", "sonnet"},
		{"gpt-4o", "sonnet", "sonnet"},
		{"some-opus-thing", "sonnet", "opus"},
		{"unknown-model", "haiku", "haiku"},
	}
	for _, tt := range tests {
		if got := MapModel(tt.requested, tt.def); got != tt.want {
			t.Errorf("MapModel(%q, %q) = %q, want %q", tt.requested, tt.def, got, tt.want)
		}
	}
}

func TestMapTools(t *testing.T) {
	tools := []Tool{
		{Function: ToolFunction{Name: "run_command"}},
		{Function: ToolFunction{Name: "read_file"}},
		{Function: ToolFunction{Name: "edit_file"}},
		{Function: ToolFunction{Name: "some_custom_tool"}},
		{Function: ToolFunction{Name: "bash"}},
	}
	got := MapTools(tools)
	want := []string{"Bash", "Edit", "Read"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MapTools = %v, want %v", got, want)
	}

	if got := MapTools(nil); len(got) != 0 {
		t.Errorf("MapTools(nil) = %v, want empty", got)
	}
}

func TestMergePrompt(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: Content{Text: "be terse"}},
		{Role: "user", Content: Content{Text: "hello"}},
		{Role: "assistant", Content: Content{Text: "hi"}},
	}
	want := "System: be terse\n\nUser: hello\n\nAssistant: hi"
	if got := MergePrompt(msgs); got != want {
		t.Errorf("MergePrompt = %q, want %q", got, want)
	}
}

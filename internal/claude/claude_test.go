package claude

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// fakeClaude writes an executable shell script that runs body and returns its
// path, for use as a stand-in claude binary.
func fakeClaude(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestArgs(t *testing.T) {
	r := New("claude", "sonnet")

	got := r.Args("opus", nil, "hello")
	want := []string{"--print", "--model", "opus", "hello"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Args without tools = %v, want %v", got, want)
	}

	got = r.Args("sonnet", []string{"Bash", "Read"}, "do it")
	want = []string{"--print", "--model", "sonnet", "--allowedTools", "Bash,Read", "do it"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Args with tools = %v, want %v", got, want)
	}
}

func TestAvailable(t *testing.T) {
	if err := New(fakeClaude(t, "true"), "sonnet").Available(); err != nil {
		t.Errorf("Available on existing binary: %v", err)
	}
	if err := New("/no/such/claude/binary", "sonnet").Available(); err == nil {
		t.Error("Available on missing binary: expected error")
	}
}

func TestVerify(t *testing.T) {
	ctx := context.Background()

	if err := New(fakeClaude(t, `echo "2.0.0"`), "sonnet").Verify(ctx); err != nil {
		t.Errorf("Verify on healthy CLI: %v", err)
	}

	failing := fakeClaude(t, `echo "boom" >&2; exit 1`)
	if err := New(failing, "sonnet").Verify(ctx); err == nil {
		t.Error("Verify on failing CLI: expected error")
	}
}

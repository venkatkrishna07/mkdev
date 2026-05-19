package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/venkatkrishna07/mkdev/internal/version"
)

func TestVersionCommand_PrintsVersionString(t *testing.T) {
	cmd := newVersionCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	got := strings.TrimSpace(out.String())
	want := version.String()
	if got != want {
		t.Fatalf("version output mismatch: got %q want %q", got, want)
	}
}

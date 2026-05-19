package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompletionCommand_EmitsScriptPerShell(t *testing.T) {
	cases := []struct {
		shell    string
		sentinel string
	}{
		{"bash", "_mkdev"},
		{"zsh", "compdef"},
		{"fish", "complete -c mkdev"},
		{"powershell", "Register-ArgumentCompleter"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.shell, func(t *testing.T) {
			root := New()
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetArgs([]string{"completion", tc.shell})

			if err := root.Execute(); err != nil {
				t.Fatalf("execute %s: %v", tc.shell, err)
			}
			if out.Len() == 0 {
				t.Fatalf("%s completion produced empty output", tc.shell)
			}
			if !strings.Contains(out.String(), tc.sentinel) {
				t.Fatalf("%s completion missing sentinel %q; first 200 chars:\n%s",
					tc.shell, tc.sentinel, firstN(out.String(), 200))
			}
		})
	}
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func TestCompletionCommand_RejectsUnknownShell(t *testing.T) {
	root := New()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"completion", "tcsh"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for unknown shell, got nil")
	}
}

package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// styled returns true when stdout is a TTY and we should emit color.
func styled() bool {
	if os.Stdout == nil {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

var (
	okColor   = lipgloss.AdaptiveColor{Light: "#10B981", Dark: "#34D399"}
	warnColor = lipgloss.AdaptiveColor{Light: "#F59E0B", Dark: "#FBBF24"}
	errColor  = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"}
	infoColor = lipgloss.AdaptiveColor{Light: "#3B82F6", Dark: "#60A5FA"}
	dimColor  = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
)

func writeLine(w io.Writer, prefix, msg string) {
	fmt.Fprintln(w, prefix+msg)
}

// Step prints a dim "→" step line (sub-task within a command).
func Step(w io.Writer, msg string) {
	prefix := "→ "
	if styled() {
		prefix = lipgloss.NewStyle().Foreground(dimColor).Render("→") + " "
	}
	writeLine(w, prefix, msg)
}

// Success prints a green ✓ + bold message.
func Success(w io.Writer, msg string) {
	prefix := "✓ "
	if styled() {
		prefix = lipgloss.NewStyle().Foreground(okColor).Bold(true).Render("✓") + " "
	}
	writeLine(w, prefix, msg)
}

// Warn prints a yellow ! + message.
func Warn(w io.Writer, msg string) {
	prefix := "! "
	if styled() {
		prefix = lipgloss.NewStyle().Foreground(warnColor).Bold(true).Render("!") + " "
	}
	writeLine(w, prefix, msg)
}

// Errorf prints a red ✗ + formatted message and returns the formatted error.
func Errorf(w io.Writer, format string, a ...any) error {
	msg := fmt.Sprintf(format, a...)
	prefix := "✗ "
	if styled() {
		prefix = lipgloss.NewStyle().Foreground(errColor).Bold(true).Render("✗") + " "
	}
	writeLine(w, prefix, msg)
	return fmt.Errorf("%s", msg)
}

// Info prints a blue i + message.
func Info(w io.Writer, msg string) {
	prefix := "i "
	if styled() {
		prefix = lipgloss.NewStyle().Foreground(infoColor).Bold(true).Render("ℹ") + " "
	}
	writeLine(w, prefix, msg)
}

// Dim prints a dim line with no glyph.
func Dim(w io.Writer, msg string) {
	if styled() {
		fmt.Fprintln(w, lipgloss.NewStyle().Foreground(dimColor).Render(msg))
	} else {
		fmt.Fprintln(w, msg)
	}
}

// Banner prints a one-line app banner: bold name + version + tagline.
func Banner(w io.Writer, name, version, tagline string) {
	if !styled() {
		fmt.Fprintf(w, "%s v%s · %s\n", name, version, tagline)
		return
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(infoColor).Render(name)
	v := lipgloss.NewStyle().Foreground(dimColor).Render(" · v" + version)
	t := lipgloss.NewStyle().Foreground(dimColor).Render(" · " + tagline)
	fmt.Fprintln(w, title+v+t)
}

// Box wraps body in a rounded border with title at top.
func Box(w io.Writer, title, body string) {
	if !styled() {
		fmt.Fprintf(w, "[%s]\n%s\n", title, body)
		return
	}
	s := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(infoColor).
		Padding(0, 1)
	titleStyled := lipgloss.NewStyle().Bold(true).Foreground(infoColor).Render(title)
	fmt.Fprintln(w, s.Render(titleStyled+"\n\n"+body))
}

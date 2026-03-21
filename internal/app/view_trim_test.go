package app

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestTrimLine_AnsiContent_UsesAnsiAwareTruncation(t *testing.T) {
	line := "\x1b[31mabcdef\x1b[0m"

	got := trimLine(line, 4)
	want := ansi.Truncate(line, 4, "...")

	if got != want {
		t.Fatalf("expected ansi-aware truncation %q, got %q", want, got)
	}
	if lipgloss.Width(got) > 4 {
		t.Fatalf("expected rendered width <= 4, got %d", lipgloss.Width(got))
	}
}

package app

import "testing"

func TestNewModel_InitialLayout(t *testing.T) {
	m := NewModel(Config{})
	if m.ActiveTab != TabOverview {
		t.Fatalf("expected default tab %q, got %q", TabOverview, m.ActiveTab)
	}
	if m.LeftPaneWidth != 30 {
		t.Fatalf("expected left pane width %d, got %d", 30, m.LeftPaneWidth)
	}
	if m.CenterPaneWidth != 80 {
		t.Fatalf("expected center pane width %d, got %d", 80, m.CenterPaneWidth)
	}
	if m.RightPaneWidth != 40 {
		t.Fatalf("expected right pane width %d, got %d", 40, m.RightPaneWidth)
	}
}

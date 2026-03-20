package app

import "testing"

func TestNewModel_InitialLayout(t *testing.T) {
	m := NewModel(Config{})
	if m.ActiveTab != TabOverview {
		t.Fatalf("expected default tab %q, got %q", TabOverview, m.ActiveTab)
	}
	if m.LeftPaneWidth <= 0 || m.CenterPaneWidth <= 0 || m.RightPaneWidth <= 0 {
		t.Fatalf("expected pane widths to be initialized")
	}
}

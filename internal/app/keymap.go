package app

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	NextWorkspace    key.Binding
	PrevWorkspace    key.Binding
	WorkspaceOverlay key.Binding
	OverlayCreate    key.Binding
	OverlaySave      key.Binding
	NextTab          key.Binding
	PrevTab          key.Binding
	TabOverview      key.Binding
	TabWorktrees     key.Binding
	TabLazygit       key.Binding
	TabDiff          key.Binding
	AddRepo          key.Binding
	SelectRepo       key.Binding
	RemoveRepo       key.Binding
	ManualRefresh    key.Binding
	TogglePolling    key.Binding
	Quit             key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		NextWorkspace: key.NewBinding(
			key.WithKeys("]"),
			key.WithHelp("]", "next workspace"),
		),
		PrevWorkspace: key.NewBinding(
			key.WithKeys("["),
			key.WithHelp("[", "prev workspace"),
		),
		WorkspaceOverlay: key.NewBinding(
			key.WithKeys("w"),
			key.WithHelp("w", "workspace overlay"),
		),
		OverlayCreate: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "overlay create"),
		),
		OverlaySave: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "overlay save"),
		),
		NextTab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next center tab"),
		),
		PrevTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev center tab"),
		),
		TabOverview: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "overview tab"),
		),
		TabWorktrees: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "worktrees tab"),
		),
		TabLazygit: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "lazygit tab"),
		),
		TabDiff: key.NewBinding(
			key.WithKeys("4"),
			key.WithHelp("4", "diff tab"),
		),
		AddRepo: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add repo path"),
		),
		SelectRepo: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select repo/fix path"),
		),
		RemoveRepo: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "remove repo"),
		),
		ManualRefresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh selected repo"),
		),
		TogglePolling: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "toggle auto polling"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

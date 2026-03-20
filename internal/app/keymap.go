package app

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	NextWorkspace key.Binding
	PrevWorkspace key.Binding
	AddRepo       key.Binding
	SelectRepo    key.Binding
	RemoveRepo    key.Binding
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
	}
}

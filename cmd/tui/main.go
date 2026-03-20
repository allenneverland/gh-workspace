package main

import (
	"log"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/allenneverland/gh-workspace/internal/app"
)

func main() {
	program := tea.NewProgram(app.NewModel(app.Config{}))
	if _, err := program.Run(); err != nil {
		log.Fatalf("tui exited with error: %v", err)
	}
}

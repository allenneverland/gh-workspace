package main

import (
	"context"
	"log"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	model, closeFn, err := composeRuntimeModel(context.Background())
	if err != nil {
		log.Fatalf("failed to compose runtime model: %v", err)
	}
	if closeFn != nil {
		defer func() {
			if err := closeFn(); err != nil {
				log.Printf("failed to close runtime store: %v", err)
			}
		}()
	}

	program := tea.NewProgram(model)
	if _, err := program.Run(); err != nil {
		log.Fatalf("tui exited with error: %v", err)
	}
}

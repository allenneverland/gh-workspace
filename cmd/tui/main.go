package main

import (
	"context"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := runInitCommand(context.Background(), os.Args[2:]); err != nil {
			log.Fatalf("init failed: %v", err)
		}
		return
	}

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

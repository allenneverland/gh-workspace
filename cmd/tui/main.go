package main

import (
	"context"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/allenneverland/gh-workspace/internal/app"
)

var composeRuntimeModelForLaunch = func(ctx context.Context, _ LaunchOptions) (app.Model, func() error, error) {
	return composeRuntimeModel(ctx)
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "init" {
		if err := runInitCommand(context.Background(), args[1:]); err != nil {
			return fmt.Errorf("init failed: %w", err)
		}
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve current directory: %w", err)
	}
	opts, err := ParseLaunchOptions(args, cwd)
	if err != nil {
		return fmt.Errorf("%w\n\n%s", err, launchOptionsUsage())
	}

	model, closeFn, err := composeRuntimeModelForLaunch(context.Background(), opts)
	if err != nil {
		return fmt.Errorf("failed to compose runtime model: %w", err)
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
		return fmt.Errorf("tui exited with error: %w", err)
	}
	return nil
}

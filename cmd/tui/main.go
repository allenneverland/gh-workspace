package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

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
	if err := rejectUnsupportedLaunchIntent(opts, cwd); err != nil {
		return err
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

func rejectUnsupportedLaunchIntent(opts LaunchOptions, cwd string) error {
	switch opts.Mode {
	case LaunchWorkspace:
		return errors.New("workspace launch via -w is not supported until runtime wiring is implemented")
	case LaunchFolder:
		matchesCWD, err := launchPathMatchesCWD(opts.Path, cwd)
		if err != nil {
			return fmt.Errorf("resolve launch path: %w", err)
		}
		if matchesCWD {
			return nil
		}
		return fmt.Errorf(
			"folder launch for path %q is not supported until runtime wiring is implemented; only current directory %q is currently supported",
			opts.Path,
			cwd,
		)
	default:
		return fmt.Errorf("unsupported launch mode %q", opts.Mode)
	}
}

func launchPathMatchesCWD(path, cwd string) (bool, error) {
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}
	cwdAbs, err := filepath.Abs(cwd)
	if err != nil {
		return false, err
	}
	return filepath.Clean(pathAbs) == filepath.Clean(cwdAbs), nil
}

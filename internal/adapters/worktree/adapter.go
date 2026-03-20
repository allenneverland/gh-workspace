package worktree

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type Adapter struct {
	runner Runner
}

type Entry struct {
	ID     string
	Path   string
	Branch string
	Commit string
}

func NewAdapter(runner Runner) *Adapter {
	if runner == nil {
		runner = commandRunner{}
	}
	return &Adapter{runner: runner}
}

func (a *Adapter) Create(ctx context.Context, repoPath, branch, targetPath string) error {
	_, err := a.runGit(ctx, "-C", repoPath, "worktree", "add", targetPath, branch)
	return err
}

func (a *Adapter) List(ctx context.Context, repoPath string) ([]Entry, error) {
	out, err := a.runGit(ctx, "-C", repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parsePorcelainList(string(out)), nil
}

func (a *Adapter) ValidateSwitchTarget(ctx context.Context, worktreePath string) error {
	out, err := a.runGit(ctx, "-C", worktreePath, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(out)) != "true" {
		return fmt.Errorf("path %q is not inside a git worktree", worktreePath)
	}
	return nil
}

func parsePorcelainList(raw string) []Entry {
	lines := strings.Split(raw, "\n")
	entries := make([]Entry, 0, 4)
	var current Entry
	hasCurrent := false

	flush := func() {
		if !hasCurrent || current.Path == "" {
			current = Entry{}
			hasCurrent = false
			return
		}
		if current.ID == "" {
			current.ID = current.Path
		}
		entries = append(entries, current)
		current = Entry{}
		hasCurrent = false
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			continue
		}
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			path := strings.TrimPrefix(line, "worktree ")
			current = Entry{
				ID:   path,
				Path: path,
			}
			hasCurrent = true
		case strings.HasPrefix(line, "branch "):
			if !hasCurrent {
				continue
			}
			branch := strings.TrimPrefix(line, "branch ")
			branch = strings.TrimPrefix(branch, "refs/heads/")
			current.Branch = branch
		case strings.HasPrefix(line, "HEAD "):
			if !hasCurrent {
				continue
			}
			current.Commit = strings.TrimPrefix(line, "HEAD ")
		}
	}
	flush()
	return entries
}

func (a *Adapter) runGit(ctx context.Context, args ...string) ([]byte, error) {
	out, err := a.runner.Run(ctx, "git", args...)
	if err == nil {
		return out, nil
	}

	command := "git " + strings.Join(args, " ")
	output := strings.TrimSpace(string(out))
	if output == "" {
		return nil, fmt.Errorf("%s failed: %w", command, err)
	}
	return nil, fmt.Errorf("%s failed: %w: %s", command, err, output)
}

type commandRunner struct{}

func (commandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

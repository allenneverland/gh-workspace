package repository

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
)

var commandContext = exec.CommandContext

func ResolveRepoRoot(ctx context.Context, path string) (string, bool, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return "", false, nil
	}

	abs, err := filepath.Abs(trimmedPath)
	if err != nil {
		return "", false, err
	}

	cmd := commandContext(ctx, "git", "-C", abs, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		if isExpectedLookupMiss(err) {
			return "", false, nil
		}
		return "", false, err
	}

	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", false, nil
	}
	return root, true, nil
}

func isExpectedLookupMiss(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}

	stderr := strings.ToLower(strings.TrimSpace(string(exitErr.Stderr)))
	if stderr == "" {
		return false
	}

	return strings.Contains(stderr, "not a git repository") ||
		strings.Contains(stderr, "cannot change to") ||
		strings.Contains(stderr, "no such file or directory") ||
		strings.Contains(stderr, "permission denied")
}

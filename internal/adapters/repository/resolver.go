package repository

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
)

func ResolveRepoRoot(ctx context.Context, path string) (string, bool, error) {
	abs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", false, nil
	}

	cmd := exec.CommandContext(ctx, "git", "-C", abs, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", false, nil
	}

	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", false, nil
	}
	return root, true, nil
}

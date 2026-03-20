package diff

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var ErrDeltaNotFound = errors.New("delta not found")

type Renderer struct {
	lookPath       func(file string) (string, error)
	commandContext func(ctx context.Context, name string, arg ...string) *exec.Cmd
}

func NewRenderer() *Renderer {
	return &Renderer{
		lookPath:       exec.LookPath,
		commandContext: exec.CommandContext,
	}
}

func (r *Renderer) Render(ctx context.Context, repoPath string) (string, error) {
	if strings.TrimSpace(repoPath) == "" {
		return "", errors.New("repo path is empty")
	}

	if _, err := r.lookPath("delta"); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", ErrDeltaNotFound
		}
		return "", fmt.Errorf("lookup delta: %w", err)
	}

	gitDiff := r.commandContext(ctx, "git", "-C", repoPath, "diff", "--no-ext-diff")
	delta := r.commandContext(ctx, "delta", "--paging=never")

	gitStdout, err := gitDiff.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("prepare git diff output: %w", err)
	}

	var gitStderr bytes.Buffer
	gitDiff.Stderr = &gitStderr

	var deltaOut bytes.Buffer
	var deltaStderr bytes.Buffer
	delta.Stdin = gitStdout
	delta.Stdout = &deltaOut
	delta.Stderr = &deltaStderr

	if err := delta.Start(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", ErrDeltaNotFound
		}
		return "", fmt.Errorf("start delta: %w", err)
	}
	if err := gitDiff.Start(); err != nil {
		_ = delta.Wait()
		return "", fmt.Errorf("start git diff: %w", err)
	}

	gitErr := gitDiff.Wait()
	deltaErr := delta.Wait()

	if gitErr != nil {
		message := strings.TrimSpace(gitStderr.String())
		if message == "" {
			return "", fmt.Errorf("git diff failed: %w", gitErr)
		}
		return "", fmt.Errorf("git diff failed: %w: %s", gitErr, message)
	}
	if deltaErr != nil {
		message := strings.TrimSpace(deltaStderr.String())
		if message == "" {
			return "", fmt.Errorf("delta render failed: %w", deltaErr)
		}
		return "", fmt.Errorf("delta render failed: %w: %s", deltaErr, message)
	}

	return deltaOut.String(), nil
}

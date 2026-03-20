package diff

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderer_RenderDiff_UsesRepoPathAndDeltaPagingNever(t *testing.T) {
	tools := fakePathWithScripts(t, true)
	t.Setenv("PATH", tools.scriptDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	const repoPath = "/tmp/repo-path"
	renderer := NewRenderer()

	out, err := renderer.Render(context.Background(), repoPath)
	if err != nil {
		t.Fatalf("render returned error: %v", err)
	}
	if got, want := strings.TrimSpace(out), "diff for /tmp/repo-path"; got != want {
		t.Fatalf("expected output %q, got %q", want, got)
	}

	gitArgsRaw, err := os.ReadFile(tools.gitArgsLog)
	if err != nil {
		t.Fatalf("read git args log: %v", err)
	}
	if got, want := strings.TrimSpace(string(gitArgsRaw)), "-C /tmp/repo-path diff --no-ext-diff"; got != want {
		t.Fatalf("expected git args %q, got %q", want, got)
	}

	deltaArgsRaw, err := os.ReadFile(tools.deltaArgsLog)
	if err != nil {
		t.Fatalf("read delta args log: %v", err)
	}
	if got, want := strings.TrimSpace(string(deltaArgsRaw)), "--paging=never"; got != want {
		t.Fatalf("expected delta args %q, got %q", want, got)
	}
}

func TestRenderer_RenderDiff_DeltaMissing_ReturnsErrDeltaNotFound(t *testing.T) {
	tools := fakePathWithScripts(t, false)
	t.Setenv("PATH", tools.scriptDir)

	renderer := NewRenderer()
	_, err := renderer.Render(context.Background(), "/tmp/repo-path")
	if err == nil {
		t.Fatal("expected delta missing error")
	}
	if !errors.Is(err, ErrDeltaNotFound) {
		t.Fatalf("expected ErrDeltaNotFound, got %v", err)
	}
}

type fakeToolPaths struct {
	scriptDir    string
	gitArgsLog   string
	deltaArgsLog string
}

func fakePathWithScripts(t *testing.T, includeDelta bool) fakeToolPaths {
	t.Helper()

	scriptDir := t.TempDir()
	gitArgsLog := filepath.Join(scriptDir, "git-args.log")
	deltaArgsLog := filepath.Join(scriptDir, "delta-args.log")

	writeExecutableScript(t, filepath.Join(scriptDir, "git"), `#!/bin/sh
printf '%s\n' "$*" > "`+gitArgsLog+`"
if [ "$1" != "-C" ]; then
  exit 2
fi
printf 'diff for %s\n' "$2"
`)

	if includeDelta {
		writeExecutableScript(t, filepath.Join(scriptDir, "delta"), `#!/bin/sh
printf '%s\n' "$*" > "`+deltaArgsLog+`"
cat
`)
	}

	return fakeToolPaths{
		scriptDir:    scriptDir,
		gitArgsLog:   gitArgsLog,
		deltaArgsLog: deltaArgsLog,
	}
}

func writeExecutableScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write script %q: %v", path, err)
	}
}

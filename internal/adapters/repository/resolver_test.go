package repository

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolver_ResolveRepoRoot_SubdirReturnsTopLevel(t *testing.T) {
	root := initTempGitRepo(t)
	subdir := filepath.Join(root, "nested")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	resolved, ok, err := ResolveRepoRoot(context.Background(), subdir)
	if err != nil {
		t.Fatalf("ResolveRepoRoot() error = %v", err)
	}
	if !ok {
		t.Fatal("ResolveRepoRoot() ok = false, want true")
	}
	if canonicalPath(t, resolved) != canonicalPath(t, root) {
		t.Fatalf("ResolveRepoRoot() = %q, want path equivalent to %q", resolved, root)
	}
}

func TestResolver_ResolveRepoRoot_NonGitPathReturnsNotFoundWithoutError(t *testing.T) {
	path := t.TempDir()

	resolved, ok, err := ResolveRepoRoot(context.Background(), path)
	if err != nil {
		t.Fatalf("ResolveRepoRoot() error = %v", err)
	}
	if ok {
		t.Fatal("ResolveRepoRoot() ok = true, want false")
	}
	if resolved != "" {
		t.Fatalf("ResolveRepoRoot() = %q, want empty string", resolved)
	}
}

func TestResolver_ResolveRepoRoot_MissingPathReturnsNotFoundWithoutError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist")

	resolved, ok, err := ResolveRepoRoot(context.Background(), path)
	if err != nil {
		t.Fatalf("ResolveRepoRoot() error = %v", err)
	}
	if ok {
		t.Fatal("ResolveRepoRoot() ok = true, want false")
	}
	if resolved != "" {
		t.Fatalf("ResolveRepoRoot() = %q, want empty string", resolved)
	}
}

func TestResolver_ResolveRepoRoot_UnreadablePathReturnsNotFoundWithoutError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permissions-based unreadable path test is not portable on windows")
	}

	path := filepath.Join(t.TempDir(), "private")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("mkdir path: %v", err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatalf("chmod unreadable path: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(path, 0o700)
	})

	resolved, ok, err := ResolveRepoRoot(context.Background(), path)
	if err != nil {
		t.Fatalf("ResolveRepoRoot() error = %v", err)
	}
	if ok {
		t.Fatal("ResolveRepoRoot() ok = true, want false")
	}
	if resolved != "" {
		t.Fatalf("ResolveRepoRoot() = %q, want empty string", resolved)
	}
}

func initTempGitRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	runGit(t, root, "init")
	return root
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	gitArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", gitArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v (out=%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
}

func canonicalPath(t *testing.T, p string) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return p
	}
	return resolved
}

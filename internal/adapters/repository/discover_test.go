package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestDiscoverRepoRoots(t *testing.T) {
	t.Run("root containing two git repos discovers both", func(t *testing.T) {
		root := t.TempDir()
		repoA := initGitRepoAtPath(t, filepath.Join(root, "alpha"))
		repoB := initGitRepoAtPath(t, filepath.Join(root, "beta"))
		if err := os.MkdirAll(filepath.Join(root, "notes"), 0o755); err != nil {
			t.Fatalf("MkdirAll(notes) error = %v", err)
		}

		got, err := DiscoverRepoRoots(context.Background(), root)
		if err != nil {
			t.Fatalf("DiscoverRepoRoots() error = %v", err)
		}

		want := canonicalPaths(t, []string{repoA, repoB})
		assertSamePaths(t, got, want)
	})

	t.Run("non-existent root returns clear error", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "does-not-exist")

		got, err := DiscoverRepoRoots(context.Background(), root)
		if err == nil {
			t.Fatal("DiscoverRepoRoots() error = nil, want error")
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("DiscoverRepoRoots() error = %v, want os.ErrNotExist", err)
		}
		if !strings.Contains(err.Error(), root) {
			t.Fatalf("DiscoverRepoRoots() error = %q, want path %q in message", err.Error(), root)
		}
		if got != nil {
			t.Fatalf("DiscoverRepoRoots() = %#v, want nil", got)
		}
	})

	t.Run("non-git folders ignored", func(t *testing.T) {
		root := t.TempDir()
		repo := initGitRepoAtPath(t, filepath.Join(root, "repo"))
		if err := os.MkdirAll(filepath.Join(root, "plain", "nested"), 0o755); err != nil {
			t.Fatalf("MkdirAll(plain nested) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "plain", "nested", "README.md"), []byte("hi"), 0o644); err != nil {
			t.Fatalf("WriteFile(README) error = %v", err)
		}

		got, err := DiscoverRepoRoots(context.Background(), root)
		if err != nil {
			t.Fatalf("DiscoverRepoRoots() error = %v", err)
		}

		assertSamePaths(t, got, canonicalPaths(t, []string{repo}))
	})

	t.Run("nested paths in same repo dedup to repo root", func(t *testing.T) {
		root := t.TempDir()
		repo := initGitRepoAtPath(t, filepath.Join(root, "monorepo"))
		for _, path := range []string{
			filepath.Join(repo, "services", "api"),
			filepath.Join(repo, "services", "web"),
			filepath.Join(repo, "docs"),
		} {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatalf("MkdirAll(%q) error = %v", path, err)
			}
		}

		got, err := DiscoverRepoRoots(context.Background(), root)
		if err != nil {
			t.Fatalf("DiscoverRepoRoots() error = %v", err)
		}

		assertSamePaths(t, got, canonicalPaths(t, []string{repo}))
	})
}

func initGitRepoAtPath(t *testing.T, path string) string {
	t.Helper()

	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	runGit(t, path, "init")
	return path
}

func assertSamePaths(t *testing.T, got, want []string) {
	t.Helper()

	got = canonicalPaths(t, got)
	want = canonicalPaths(t, want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("paths mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func canonicalPaths(t *testing.T, paths []string) []string {
	t.Helper()

	if paths == nil {
		return nil
	}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		out = append(out, canonicalPath(t, path))
	}
	sort.Strings(out)
	return out
}

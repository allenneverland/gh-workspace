package repository

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxDiscoveredRepoRoots = 500

func DiscoverRepoRoots(ctx context.Context, rootPath string) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	trimmedRoot := strings.TrimSpace(rootPath)
	if trimmedRoot == "" {
		return nil, nil
	}

	root, err := canonicalDiscoverPath(trimmedRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("discover repo roots under %q: %w", trimmedRoot, err)
		}
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	roots := make([]string, 0, 16)
	seen := make(map[string]struct{}, 16)
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, fs.ErrPermission) {
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d == nil || !d.IsDir() {
			return nil
		}

		hasGit, err := hasGitMetadata(path)
		if err != nil {
			if errors.Is(err, fs.ErrPermission) {
				return filepath.SkipDir
			}
			return err
		}
		if !hasGit {
			return nil
		}

		repoRoot, ok, err := ResolveRepoRoot(ctx, path)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		repoRoot, err = canonicalDiscoverPath(repoRoot)
		if err != nil {
			return err
		}
		if _, exists := seen[repoRoot]; !exists {
			seen[repoRoot] = struct{}{}
			roots = append(roots, repoRoot)
			if len(roots) >= maxDiscoveredRepoRoots {
				return fs.SkipAll
			}
		}
		return filepath.SkipDir
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(roots)
	return roots, nil
}

func hasGitMetadata(path string) (bool, error) {
	gitPath := filepath.Join(path, ".git")
	info, err := os.Lstat(gitPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if info.IsDir() {
		return true, nil
	}
	return info.Mode().IsRegular(), nil
}

func canonicalDiscoverPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		return filepath.Clean(abs), nil
	}
	return filepath.Clean(resolved), nil
}

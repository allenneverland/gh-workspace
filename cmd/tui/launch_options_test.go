package main

import (
	"strings"
	"testing"
)

func TestParseLaunchOptions_DefaultUsesCWD(t *testing.T) {
	opts, err := ParseLaunchOptions([]string{}, "/tmp/current")
	if err != nil {
		t.Fatalf("ParseLaunchOptions(default) error = %v", err)
	}
	if opts.Mode != LaunchFolder {
		t.Fatalf("expected mode %q, got %q", LaunchFolder, opts.Mode)
	}
	if opts.Path != "/tmp/current" {
		t.Fatalf("expected folder path %q, got %q", "/tmp/current", opts.Path)
	}
}

func TestParseLaunchOptions_FolderFlag(t *testing.T) {
	opts, err := ParseLaunchOptions([]string{"-f", "/tmp/repo"}, "/tmp/current")
	if err != nil {
		t.Fatalf("ParseLaunchOptions(-f) error = %v", err)
	}
	if opts.Mode != LaunchFolder {
		t.Fatalf("expected mode %q, got %q", LaunchFolder, opts.Mode)
	}
	if opts.Path != "/tmp/repo" {
		t.Fatalf("expected folder path %q, got %q", "/tmp/repo", opts.Path)
	}
	if opts.WorkspaceName != "" {
		t.Fatalf("expected empty workspace name in folder mode, got %q", opts.WorkspaceName)
	}
}

func TestParseLaunchOptions_WorkspaceFlag(t *testing.T) {
	opts, err := ParseLaunchOptions([]string{"-w", "team-a"}, "/tmp/current")
	if err != nil {
		t.Fatalf("ParseLaunchOptions(-w) error = %v", err)
	}
	if opts.Mode != LaunchWorkspace {
		t.Fatalf("expected mode %q, got %q", LaunchWorkspace, opts.Mode)
	}
	if opts.WorkspaceName != "team-a" {
		t.Fatalf("expected workspace name %q, got %q", "team-a", opts.WorkspaceName)
	}
	if opts.Path != "" {
		t.Fatalf("expected empty folder path in workspace mode, got %q", opts.Path)
	}
}

func TestParseLaunchOptions_RejectsFolderAndWorkspaceFlagsTogether(t *testing.T) {
	_, err := ParseLaunchOptions([]string{"-f", "/tmp/repo", "-w", "team-a"}, "/tmp/current")
	if err == nil {
		t.Fatal("expected error when using -f and -w together")
	}
	if !strings.Contains(err.Error(), "cannot use -f with -w") {
		t.Fatalf("expected combo validation error, got %v", err)
	}
}

func TestParseLaunchOptions_RejectsEmptyFolderFlagValue(t *testing.T) {
	_, err := ParseLaunchOptions([]string{"-f", ""}, "/tmp/current")
	if err == nil {
		t.Fatal("expected error when -f value is empty")
	}
	if !strings.Contains(err.Error(), "-f cannot be empty") {
		t.Fatalf("expected -f empty validation error, got %v", err)
	}
}

func TestParseLaunchOptions_RejectsEmptyWorkspaceFlagValue(t *testing.T) {
	_, err := ParseLaunchOptions([]string{"-w", ""}, "/tmp/current")
	if err == nil {
		t.Fatal("expected error when -w value is empty")
	}
	if !strings.Contains(err.Error(), "-w cannot be empty") {
		t.Fatalf("expected -w empty validation error, got %v", err)
	}
}

func TestParseLaunchOptions_RejectsReservedWorkspaceName(t *testing.T) {
	_, err := ParseLaunchOptions([]string{"-w", "__local_internal__"}, "/tmp/current")
	if err == nil {
		t.Fatal("expected error for reserved workspace name")
	}
	if !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected reserved-name validation error, got %v", err)
	}
}

func TestParseLaunchOptions_RejectsUnexpectedPositionalArguments(t *testing.T) {
	_, err := ParseLaunchOptions([]string{"unexpected"}, "/tmp/current")
	if err == nil {
		t.Fatal("expected error for unexpected positional arguments")
	}
	if !strings.Contains(err.Error(), "unexpected arguments") {
		t.Fatalf("expected positional-argument validation error, got %v", err)
	}
}

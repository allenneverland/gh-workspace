package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

type LaunchMode string

const (
	LaunchFolder    LaunchMode = "folder"
	LaunchWorkspace LaunchMode = "workspace"

	reservedWorkspaceName = "__local_internal__"
)

type LaunchOptions struct {
	Mode          LaunchMode
	Path          string
	WorkspaceName string
}

func ParseLaunchOptions(args []string, cwd string) (LaunchOptions, error) {
	fs := flag.NewFlagSet("gh-workspace", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var folderFlag trackedStringFlag
	var workspaceFlag trackedStringFlag
	fs.Var(&folderFlag, "f", "folder path")
	fs.Var(&workspaceFlag, "w", "workspace name")

	if err := fs.Parse(args); err != nil {
		return LaunchOptions{}, err
	}
	if fs.NArg() > 0 {
		return LaunchOptions{}, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}

	if folderFlag.set && workspaceFlag.set {
		return LaunchOptions{}, errors.New("cannot use -f with -w")
	}
	if folderFlag.set {
		path := strings.TrimSpace(folderFlag.value)
		if path == "" {
			return LaunchOptions{}, errors.New("-f cannot be empty")
		}
		return LaunchOptions{Mode: LaunchFolder, Path: path}, nil
	}
	if workspaceFlag.set {
		name := strings.TrimSpace(workspaceFlag.value)
		if name == "" {
			return LaunchOptions{}, errors.New("-w cannot be empty")
		}
		if name == reservedWorkspaceName {
			return LaunchOptions{}, fmt.Errorf("workspace name %q is reserved", name)
		}
		return LaunchOptions{Mode: LaunchWorkspace, WorkspaceName: name}, nil
	}

	path := strings.TrimSpace(cwd)
	if path == "" {
		return LaunchOptions{}, errors.New("current directory cannot be empty")
	}
	return LaunchOptions{Mode: LaunchFolder, Path: path}, nil
}

func launchOptionsUsage() string {
	return strings.TrimSpace(`
Usage: gh-workspace [-f <path> | -w <workspace-name>]

Options:
  -f <path>            Launch in folder mode using the provided path.
  -w <workspace-name>  Launch in workspace mode using the provided workspace name.
`)
}

type trackedStringFlag struct {
	value string
	set   bool
}

func (f *trackedStringFlag) String() string {
	return f.value
}

func (f *trackedStringFlag) Set(value string) error {
	f.value = value
	f.set = true
	return nil
}

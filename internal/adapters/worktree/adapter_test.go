package worktree

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestAdapter_Create_UsesGitWorktreeAdd(t *testing.T) {
	runner := &fakeRunner{}
	adapter := NewAdapter(runner)

	if err := adapter.Create(context.Background(), "/repo", "feature/a", "../repo-feature-a"); err != nil {
		t.Fatalf("create returned error: %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 command, got %d", len(runner.calls))
	}
	want := commandCall{
		name: "git",
		args: []string{"-C", "/repo", "worktree", "add", "../repo-feature-a", "feature/a"},
	}
	if !reflect.DeepEqual(runner.calls[0], want) {
		t.Fatalf("unexpected command:\nwant=%#v\ngot=%#v", want, runner.calls[0])
	}
}

func TestAdapter_List_UsesGitWorktreeListPorcelain(t *testing.T) {
	runner := &fakeRunner{
		outputByCommand: map[string][]byte{
			"git|-C|/repo|worktree|list|--porcelain": []byte("worktree /repo\nbranch refs/heads/main\n"),
		},
	}
	adapter := NewAdapter(runner)

	entries, err := adapter.List(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("list returned error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected parsed entries from porcelain output")
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 command, got %d", len(runner.calls))
	}
	want := commandCall{
		name: "git",
		args: []string{"-C", "/repo", "worktree", "list", "--porcelain"},
	}
	if !reflect.DeepEqual(runner.calls[0], want) {
		t.Fatalf("unexpected command:\nwant=%#v\ngot=%#v", want, runner.calls[0])
	}
}

func TestAdapter_List_ParsesPorcelainFields(t *testing.T) {
	runner := &fakeRunner{
		outputByCommand: map[string][]byte{
			"git|-C|/repo|worktree|list|--porcelain": []byte(strings.Join([]string{
				"worktree /repo",
				"HEAD 1111111111111111111111111111111111111111",
				"branch refs/heads/main",
				"",
				"worktree ../repo-feature-a",
				"HEAD 2222222222222222222222222222222222222222",
				"branch refs/heads/feature/a",
				"",
			}, "\n")),
		},
	}
	adapter := NewAdapter(runner)

	entries, err := adapter.List(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("list returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0].Path != "/repo" {
		t.Fatalf("expected first path %q, got %q", "/repo", entries[0].Path)
	}
	if entries[0].Branch != "main" {
		t.Fatalf("expected first branch %q, got %q", "main", entries[0].Branch)
	}
	if entries[0].Commit != "1111111111111111111111111111111111111111" {
		t.Fatalf("expected first commit to be parsed, got %q", entries[0].Commit)
	}

	if entries[1].Path != "../repo-feature-a" {
		t.Fatalf("expected second path %q, got %q", "../repo-feature-a", entries[1].Path)
	}
	if entries[1].Branch != "feature/a" {
		t.Fatalf("expected second branch %q, got %q", "feature/a", entries[1].Branch)
	}
	if entries[1].Commit != "2222222222222222222222222222222222222222" {
		t.Fatalf("expected second commit to be parsed, got %q", entries[1].Commit)
	}
}

func TestAdapter_ValidateSwitchTarget_UsesGitRevParseInsideWorkTree(t *testing.T) {
	runner := &fakeRunner{
		outputByCommand: map[string][]byte{
			"git|-C|../repo-feature-a|rev-parse|--is-inside-work-tree": []byte("true\n"),
		},
	}
	adapter := NewAdapter(runner)

	if err := adapter.ValidateSwitchTarget(context.Background(), "../repo-feature-a"); err != nil {
		t.Fatalf("validate switch target returned error: %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 command, got %d", len(runner.calls))
	}
	want := commandCall{
		name: "git",
		args: []string{"-C", "../repo-feature-a", "rev-parse", "--is-inside-work-tree"},
	}
	if !reflect.DeepEqual(runner.calls[0], want) {
		t.Fatalf("unexpected command:\nwant=%#v\ngot=%#v", want, runner.calls[0])
	}
}

func TestAdapter_Create_WhenCommandFails_ContainsCommandAndOutput(t *testing.T) {
	runner := &fakeRunner{
		outputByCommand: map[string][]byte{
			"git|-C|/repo|worktree|add|../repo-feature-a|feature/a": []byte("fatal: cannot lock ref\n"),
		},
		errByCommand: map[string]error{
			"git|-C|/repo|worktree|add|../repo-feature-a|feature/a": errors.New("exit status 128"),
		},
	}
	adapter := NewAdapter(runner)

	err := adapter.Create(context.Background(), "/repo", "feature/a", "../repo-feature-a")
	if err == nil {
		t.Fatal("expected create to return error")
	}

	if !strings.Contains(err.Error(), "git -C /repo worktree add ../repo-feature-a feature/a") {
		t.Fatalf("expected command context in error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "fatal: cannot lock ref") {
		t.Fatalf("expected command output in error, got %q", err.Error())
	}
}

type fakeRunner struct {
	calls           []commandCall
	outputByCommand map[string][]byte
	errByCommand    map[string]error
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	call := commandCall{name: name, args: append([]string(nil), args...)}
	f.calls = append(f.calls, call)

	key := call.key()
	out, hasOut := f.outputByCommand[key]
	if err := f.errByCommand[key]; err != nil {
		if hasOut {
			return append([]byte(nil), out...), err
		}
		return nil, err
	}
	if hasOut {
		return append([]byte(nil), out...), nil
	}
	return nil, nil
}

type commandCall struct {
	name string
	args []string
}

func (c commandCall) key() string {
	key := c.name
	for _, arg := range c.args {
		key += "|" + arg
	}
	return key
}

var _ Runner = (*fakeRunner)(nil)

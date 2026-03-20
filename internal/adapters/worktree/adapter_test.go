package worktree

import (
	"context"
	"reflect"
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

type fakeRunner struct {
	calls           []commandCall
	outputByCommand map[string][]byte
	errByCommand    map[string]error
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	call := commandCall{name: name, args: append([]string(nil), args...)}
	f.calls = append(f.calls, call)

	key := call.key()
	if err := f.errByCommand[key]; err != nil {
		return nil, err
	}
	if out, ok := f.outputByCommand[key]; ok {
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

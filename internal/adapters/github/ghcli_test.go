package github

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

func TestAdapter_CheckAuth_UsesGhAuthStatus(t *testing.T) {
	runner := &fakeRunner{}
	adapter := NewAdapter(runner)

	if err := adapter.CheckAuth(context.Background()); err != nil {
		t.Fatalf("CheckAuth() error = %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 command, got %d", len(runner.calls))
	}

	want := commandCall{
		name: "gh",
		args: []string{"auth", "status", "--hostname", "github.com"},
	}
	if !reflect.DeepEqual(runner.calls[0], want) {
		t.Fatalf("unexpected command:\nwant=%#v\ngot=%#v", want, runner.calls[0])
	}
}

func TestAdapter_FetchRepoStatus_UsesConfiguredReleaseWorkflowRef(t *testing.T) {
	runner := &fakeRunner{
		outputByCommand: map[string][]byte{
			"gh|auth|status|--hostname|github.com":                                            []byte("ok"),
			"gh|pr|list|--repo|acme/svc|--state|open|--base|main|--limit|1|--json|state":      []byte(`[{"state":"OPEN"}]`),
			"gh|api|repos/acme/svc/actions/runs?branch=main&per_page=1":                       []byte(`{"workflow_runs":[{"status":"completed","conclusion":"success"}]}`),
			"gh|api|repos/acme/svc/actions/workflows/release.yml/runs?branch=main&per_page=1": []byte(`{"workflow_runs":[{"status":"completed","conclusion":"success","display_title":"publish","event":"push","html_url":"https://github.com/acme/svc/actions/runs/42","updated_at":"2026-03-21T07:30:00Z"}]}`),
		},
	}
	adapter := NewAdapter(runner)

	st, err := adapter.FetchRepoStatus(context.Background(), workspace.Repo{
		Name:               "acme/svc",
		DefaultBranch:      "main",
		ReleaseWorkflowRef: "release.yml",
	})
	if err != nil {
		t.Fatalf("FetchRepoStatus() error = %v", err)
	}

	if st.PR != workspace.StatusInProgress {
		t.Fatalf("expected PR status %q, got %q", workspace.StatusInProgress, st.PR)
	}
	if st.CI != workspace.StatusSuccess {
		t.Fatalf("expected CI status %q, got %q", workspace.StatusSuccess, st.CI)
	}
	if st.Release != workspace.StatusSuccess {
		t.Fatalf("expected release status %q, got %q", workspace.StatusSuccess, st.Release)
	}
	if st.ReleaseRun.Name != "publish" {
		t.Fatalf("expected release run name %q, got %q", "publish", st.ReleaseRun.Name)
	}
	if st.ReleaseRun.Event != "push" {
		t.Fatalf("expected release run event %q, got %q", "push", st.ReleaseRun.Event)
	}
	if st.ReleaseRun.URL != "https://github.com/acme/svc/actions/runs/42" {
		t.Fatalf("expected release run url %q, got %q", "https://github.com/acme/svc/actions/runs/42", st.ReleaseRun.URL)
	}

	wantReleaseCall := "gh|api|repos/acme/svc/actions/workflows/release.yml/runs?branch=main&per_page=1"
	if !containsCall(runner.calls, wantReleaseCall) {
		t.Fatalf("expected release lookup command %q, calls=%v", wantReleaseCall, callKeys(runner.calls))
	}
}

func TestAdapter_FetchRepoStatus_ReleaseWorkflowFallbackToGlobalRunsWhenBranchScopedEmpty(t *testing.T) {
	runner := &fakeRunner{
		outputByCommand: map[string][]byte{
			"gh|auth|status|--hostname|github.com":                                            []byte("ok"),
			"gh|pr|list|--repo|acme/svc|--state|open|--base|main|--limit|1|--json|state":      []byte(`[]`),
			"gh|api|repos/acme/svc/actions/runs?branch=main&per_page=1":                       []byte(`{"workflow_runs":[{"status":"completed","conclusion":"success"}]}`),
			"gh|api|repos/acme/svc/actions/workflows/release.yml/runs?branch=main&per_page=1": []byte(`{"workflow_runs":[]}`),
			"gh|api|repos/acme/svc/actions/workflows/release.yml/runs?per_page=1":             []byte(`{"workflow_runs":[{"status":"in_progress","conclusion":"","display_title":"publish tag v1.2.3","event":"release","html_url":"https://github.com/acme/svc/actions/runs/99","updated_at":"2026-03-21T08:15:00Z"}]}`),
		},
	}
	adapter := NewAdapter(runner)

	st, err := adapter.FetchRepoStatus(context.Background(), workspace.Repo{
		Name:               "acme/svc",
		DefaultBranch:      "main",
		ReleaseWorkflowRef: "release.yml",
	})
	if err != nil {
		t.Fatalf("FetchRepoStatus() error = %v", err)
	}

	if st.Release != workspace.StatusInProgress {
		t.Fatalf("expected release status %q, got %q", workspace.StatusInProgress, st.Release)
	}
	if st.ReleaseRun.Name != "publish tag v1.2.3" {
		t.Fatalf("expected release run name %q, got %q", "publish tag v1.2.3", st.ReleaseRun.Name)
	}
	if st.ReleaseRun.Event != "release" {
		t.Fatalf("expected release run event %q, got %q", "release", st.ReleaseRun.Event)
	}
	if st.ReleaseRun.URL != "https://github.com/acme/svc/actions/runs/99" {
		t.Fatalf("expected release run url %q, got %q", "https://github.com/acme/svc/actions/runs/99", st.ReleaseRun.URL)
	}

	wantFallbackCall := "gh|api|repos/acme/svc/actions/workflows/release.yml/runs?per_page=1"
	if !containsCall(runner.calls, wantFallbackCall) {
		t.Fatalf("expected release fallback lookup command %q, calls=%v", wantFallbackCall, callKeys(runner.calls))
	}
}

func TestAdapter_FetchRepoStatus_ReleaseUnconfiguredWhenWorkflowRefMissing(t *testing.T) {
	runner := &fakeRunner{
		outputByCommand: map[string][]byte{
			"gh|auth|status|--hostname|github.com":                                       []byte("ok"),
			"gh|pr|list|--repo|acme/svc|--state|open|--base|main|--limit|1|--json|state": []byte(`[]`),
			"gh|api|repos/acme/svc/actions/runs?branch=main&per_page=1":                  []byte(`{"workflow_runs":[{"status":"completed","conclusion":"success"}]}`),
		},
	}
	adapter := NewAdapter(runner)

	st, err := adapter.FetchRepoStatus(context.Background(), workspace.Repo{
		Name:               "acme/svc",
		DefaultBranch:      "main",
		ReleaseWorkflowRef: "",
	})
	if err != nil {
		t.Fatalf("FetchRepoStatus() error = %v", err)
	}

	if st.Release != workspace.StatusUnconfigured {
		t.Fatalf("expected release status %q, got %q", workspace.StatusUnconfigured, st.Release)
	}

	for _, call := range runner.calls {
		if strings.Contains(call.key(), "/actions/workflows/") {
			t.Fatalf("expected no release workflow command when ref is empty, got call %q", call.key())
		}
	}
}

func TestAdapter_FetchRepoStatus_ResolvesRepoSlugFromGitRemoteWhenNameIsLocalLabel(t *testing.T) {
	runner := &fakeRunner{
		outputByCommand: map[string][]byte{
			"gh|auth|status|--hostname|github.com":                                                            []byte("ok"),
			"git|-C|/tmp/gh-workspace|remote|get-url|origin":                                                  []byte("git@github.com:allenneverland/gh-workspace.git\n"),
			"gh|pr|list|--repo|allenneverland/gh-workspace|--state|open|--base|master|--limit|1|--json|state": []byte(`[]`),
			"gh|api|repos/allenneverland/gh-workspace/actions/runs?branch=master&per_page=1":                  []byte(`{"workflow_runs":[{"status":"completed","conclusion":"success"}]}`),
		},
	}
	adapter := NewAdapter(runner)

	st, err := adapter.FetchRepoStatus(context.Background(), workspace.Repo{
		Name:          "gh-workspace",
		Path:          "/tmp/gh-workspace",
		DefaultBranch: "master",
	})
	if err != nil {
		t.Fatalf("FetchRepoStatus() error = %v", err)
	}
	if st.CI != workspace.StatusSuccess {
		t.Fatalf("expected CI status %q, got %q", workspace.StatusSuccess, st.CI)
	}
	if !containsCall(runner.calls, "git|-C|/tmp/gh-workspace|remote|get-url|origin") {
		t.Fatalf("expected git remote lookup call, calls=%v", callKeys(runner.calls))
	}
}

func TestAdapter_CheckAuth_WhenCommandFails_IncludesCommandAndOutput(t *testing.T) {
	runner := &fakeRunner{
		outputByCommand: map[string][]byte{
			"gh|auth|status|--hostname|github.com": []byte("not logged in"),
		},
		errByCommand: map[string]error{
			"gh|auth|status|--hostname|github.com": errors.New("exit status 1"),
		},
	}
	adapter := NewAdapter(runner)

	err := adapter.CheckAuth(context.Background())
	if err == nil {
		t.Fatal("CheckAuth() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "gh auth status --hostname github.com") {
		t.Fatalf("expected command context in error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "not logged in") {
		t.Fatalf("expected command output in error, got %q", err.Error())
	}
}

func TestAdapter_FetchRepoStatus_MergedStateDoesNotMapSuccessWhenQueryIsOpenOnly(t *testing.T) {
	runner := &fakeRunner{
		outputByCommand: map[string][]byte{
			"gh|auth|status|--hostname|github.com":                                       []byte("ok"),
			"gh|pr|list|--repo|acme/svc|--state|open|--base|main|--limit|1|--json|state": []byte(`[{"state":"MERGED"}]`),
			"gh|api|repos/acme/svc/actions/runs?branch=main&per_page=1":                  []byte(`{"workflow_runs":[{"status":"completed","conclusion":"success"}]}`),
		},
	}
	adapter := NewAdapter(runner)

	st, err := adapter.FetchRepoStatus(context.Background(), workspace.Repo{
		Name:          "acme/svc",
		DefaultBranch: "main",
	})
	if err != nil {
		t.Fatalf("FetchRepoStatus() error = %v", err)
	}
	if st.PR != workspace.StatusNeutral {
		t.Fatalf("expected PR status %q, got %q", workspace.StatusNeutral, st.PR)
	}
}

func TestAdapter_FetchRepoStatus_InvalidPRJSON_IncludesContextAndSnippet(t *testing.T) {
	runner := &fakeRunner{
		outputByCommand: map[string][]byte{
			"gh|auth|status|--hostname|github.com":                                       []byte("ok"),
			"gh|pr|list|--repo|acme/svc|--state|open|--base|main|--limit|1|--json|state": []byte(`[{`),
		},
	}
	adapter := NewAdapter(runner)

	_, err := adapter.FetchRepoStatus(context.Background(), workspace.Repo{
		Name:          "acme/svc",
		DefaultBranch: "main",
	})
	if err == nil {
		t.Fatal("FetchRepoStatus() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "parse gh pr list output") {
		t.Fatalf("expected parse context in error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "gh pr list --repo acme/svc --state open --base main --limit 1 --json state") {
		t.Fatalf("expected command context in error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "[{") {
		t.Fatalf("expected raw snippet in error, got %q", err.Error())
	}
}

func TestAdapter_FetchRepoStatus_InvalidWorkflowJSON_IncludesEndpointAndSnippet(t *testing.T) {
	runner := &fakeRunner{
		outputByCommand: map[string][]byte{
			"gh|auth|status|--hostname|github.com":                                       []byte("ok"),
			"gh|pr|list|--repo|acme/svc|--state|open|--base|main|--limit|1|--json|state": []byte(`[]`),
			"gh|api|repos/acme/svc/actions/runs?branch=main&per_page=1":                  []byte(`{"workflow_runs":[`),
		},
	}
	adapter := NewAdapter(runner)

	_, err := adapter.FetchRepoStatus(context.Background(), workspace.Repo{
		Name:          "acme/svc",
		DefaultBranch: "main",
	})
	if err == nil {
		t.Fatal("FetchRepoStatus() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "parse gh workflow runs output") {
		t.Fatalf("expected parse context in error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "repos/acme/svc/actions/runs?branch=main&per_page=1") {
		t.Fatalf("expected endpoint context in error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "workflow_runs") {
		t.Fatalf("expected raw snippet in error, got %q", err.Error())
	}
}

func TestAdapter_FetchRepoStatus_WhenWorkflowCommandFails_ReturnsCommandError(t *testing.T) {
	runner := &fakeRunner{
		outputByCommand: map[string][]byte{
			"gh|auth|status|--hostname|github.com":                                       []byte("ok"),
			"gh|pr|list|--repo|acme/svc|--state|open|--base|main|--limit|1|--json|state": []byte(`[]`),
			"gh|api|repos/acme/svc/actions/runs?branch=main&per_page=1":                  []byte(`{"message":"internal error"}`),
		},
		errByCommand: map[string]error{
			"gh|api|repos/acme/svc/actions/runs?branch=main&per_page=1": errors.New("exit status 1"),
		},
	}
	adapter := NewAdapter(runner)

	_, err := adapter.FetchRepoStatus(context.Background(), workspace.Repo{
		Name:          "acme/svc",
		DefaultBranch: "main",
	})
	if err == nil {
		t.Fatal("FetchRepoStatus() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "gh api repos/acme/svc/actions/runs?branch=main&per_page=1") {
		t.Fatalf("expected command context in error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "internal error") {
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

func containsCall(calls []commandCall, want string) bool {
	for _, call := range calls {
		if call.key() == want {
			return true
		}
	}
	return false
}

func callKeys(calls []commandCall) []string {
	keys := make([]string, 0, len(calls))
	for _, call := range calls {
		keys = append(keys, call.key())
	}
	return keys
}

var _ Runner = (*fakeRunner)(nil)

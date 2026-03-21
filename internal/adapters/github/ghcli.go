package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type Adapter struct {
	runner Runner
}

func NewAdapter(runner Runner) *Adapter {
	if runner == nil {
		runner = commandRunner{}
	}
	return &Adapter{runner: runner}
}

func (a *Adapter) CheckAuth(ctx context.Context) error {
	_, err := a.runGH(ctx, "auth", "status", "--hostname", "github.com")
	return err
}

func (a *Adapter) FetchRepoStatus(ctx context.Context, repo workspace.Repo) (workspace.RepoStatus, error) {
	if strings.TrimSpace(repo.Name) == "" {
		return workspace.RepoStatus{}, fmt.Errorf("repo name is required")
	}
	repoSlug, err := a.resolveRepoSlug(ctx, repo)
	if err != nil {
		return workspace.RepoStatus{}, err
	}

	if err := a.CheckAuth(ctx); err != nil {
		return workspace.RepoStatus{}, err
	}

	prStatus, err := a.fetchPullRequestStatus(ctx, repoSlug, repo.DefaultBranch)
	if err != nil {
		return workspace.RepoStatus{}, err
	}

	ciStatus, err := a.fetchLatestWorkflowStatus(ctx, repoSlug, "", repo.DefaultBranch)
	if err != nil {
		return workspace.RepoStatus{}, err
	}

	releaseStatus := workspace.StatusUnconfigured
	releaseRun := workspace.ReleaseRun{}
	if strings.TrimSpace(repo.ReleaseWorkflowRef) != "" {
		releaseStatus, releaseRun, err = a.fetchReleaseWorkflowStatus(ctx, repoSlug, repo.ReleaseWorkflowRef, repo.DefaultBranch)
		if err != nil {
			return workspace.RepoStatus{}, err
		}
	}

	return workspace.RepoStatus{
		PR:         prStatus,
		CI:         ciStatus,
		Release:    releaseStatus,
		ReleaseRun: releaseRun,
	}, nil
}

type pullRequest struct {
	State string `json:"state"`
}

func (a *Adapter) fetchPullRequestStatus(ctx context.Context, repoSlug, defaultBranch string) (workspace.Status, error) {
	args := []string{
		"pr", "list",
		"--repo", repoSlug,
		"--state", "open",
	}
	if strings.TrimSpace(defaultBranch) != "" {
		args = append(args, "--base", defaultBranch)
	}
	args = append(args, "--limit", "1", "--json", "state")

	out, err := a.runGH(ctx, args...)
	if err != nil {
		return workspace.StatusNeutral, err
	}

	var pulls []pullRequest
	if err := json.Unmarshal(out, &pulls); err != nil {
		command := "gh " + strings.Join(args, " ")
		return workspace.StatusNeutral, fmt.Errorf("parse gh pr list output for %s: %w (raw=%q)", command, err, safeSnippet(out))
	}
	if len(pulls) == 0 {
		return workspace.StatusNeutral, nil
	}

	switch strings.ToLower(strings.TrimSpace(pulls[0].State)) {
	case "open":
		return workspace.StatusInProgress, nil
	default:
		return workspace.StatusNeutral, nil
	}
}

func (a *Adapter) resolveRepoSlug(ctx context.Context, repo workspace.Repo) (string, error) {
	name := strings.TrimSpace(repo.Name)
	if strings.Contains(name, "/") {
		return name, nil
	}

	path := strings.TrimSpace(repo.Path)
	if path == "" {
		return "", fmt.Errorf("repo %q must be in OWNER/REPO format or include repo path", name)
	}

	out, err := a.runner.Run(ctx, "git", "-C", path, "remote", "get-url", "origin")
	if err != nil {
		output := strings.TrimSpace(string(out))
		if output == "" {
			return "", fmt.Errorf("git remote get-url origin failed for %q: %w", path, err)
		}
		return "", fmt.Errorf("git remote get-url origin failed for %q: %w: %s", path, err, output)
	}

	slug, ok := repoSlugFromGitRemoteURL(string(out))
	if !ok {
		return "", fmt.Errorf("could not parse OWNER/REPO from origin remote url %q", strings.TrimSpace(string(out)))
	}
	return slug, nil
}

func repoSlugFromGitRemoteURL(raw string) (string, bool) {
	remote := strings.TrimSpace(raw)
	if remote == "" {
		return "", false
	}
	remote = strings.TrimSuffix(remote, ".git")

	path := remote
	if strings.Contains(remote, "://") {
		parsed, err := url.Parse(remote)
		if err != nil {
			return "", false
		}
		path = parsed.Path
	} else if idx := strings.Index(remote, ":"); idx >= 0 {
		path = remote[idx+1:]
	}

	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		return "", false
	}
	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSpace(parts[1])
	if owner == "" || repo == "" {
		return "", false
	}
	return owner + "/" + repo, true
}

type workflowRunsResponse struct {
	WorkflowRuns []workflowRun `json:"workflow_runs"`
}

type workflowRun struct {
	Status       string `json:"status"`
	Conclusion   string `json:"conclusion"`
	DisplayTitle string `json:"display_title"`
	Event        string `json:"event"`
	HTMLURL      string `json:"html_url"`
	UpdatedAt    string `json:"updated_at"`
}

func (a *Adapter) fetchLatestWorkflowStatus(ctx context.Context, repoName, workflowRef, branch string) (workspace.Status, error) {
	status, _, found, err := a.fetchLatestWorkflowStatusRun(ctx, repoName, workflowRef, branch)
	if err != nil {
		return workspace.StatusNeutral, err
	}
	if !found {
		return workspace.StatusNeutral, nil
	}
	return status, nil
}

func (a *Adapter) fetchReleaseWorkflowStatus(ctx context.Context, repoName, workflowRef, branch string) (workspace.Status, workspace.ReleaseRun, error) {
	status, run, found, err := a.fetchLatestWorkflowStatusRun(ctx, repoName, workflowRef, branch)
	if err != nil {
		return workspace.StatusNeutral, workspace.ReleaseRun{}, err
	}
	if found {
		return status, releaseRunFromWorkflowRun(run), nil
	}
	if strings.TrimSpace(branch) == "" {
		return workspace.StatusNeutral, workspace.ReleaseRun{}, nil
	}

	status, run, found, err = a.fetchLatestWorkflowStatusRun(ctx, repoName, workflowRef, "")
	if err != nil {
		return workspace.StatusNeutral, workspace.ReleaseRun{}, err
	}
	if !found {
		return workspace.StatusNeutral, workspace.ReleaseRun{}, nil
	}
	return status, releaseRunFromWorkflowRun(run), nil
}

func (a *Adapter) fetchLatestWorkflowStatusRun(ctx context.Context, repoName, workflowRef, branch string) (workspace.Status, workflowRun, bool, error) {
	endpoint := workflowRunsEndpoint(repoName, workflowRef, branch)
	out, err := a.runGH(ctx, "api", endpoint)
	if err != nil {
		return workspace.StatusNeutral, workflowRun{}, false, err
	}

	var runs workflowRunsResponse
	if err := json.Unmarshal(out, &runs); err != nil {
		return workspace.StatusNeutral, workflowRun{}, false, fmt.Errorf("parse gh workflow runs output for endpoint %q: %w (raw=%q)", endpoint, err, safeSnippet(out))
	}
	if len(runs.WorkflowRuns) == 0 {
		return workspace.StatusNeutral, workflowRun{}, false, nil
	}

	latest := runs.WorkflowRuns[0]
	return workspace.StatusFromGitHubRun(latest.Status, latest.Conclusion), latest, true, nil
}

func workflowRunsEndpoint(repoName, workflowRef, branch string) string {
	base := fmt.Sprintf("repos/%s/actions/runs", repoName)
	if strings.TrimSpace(workflowRef) != "" {
		base = fmt.Sprintf("repos/%s/actions/workflows/%s/runs", repoName, url.PathEscape(workflowRef))
	}

	query := url.Values{}
	if strings.TrimSpace(branch) != "" {
		query.Set("branch", branch)
	}
	query.Set("per_page", "1")
	return base + "?" + query.Encode()
}

func (a *Adapter) runGH(ctx context.Context, args ...string) ([]byte, error) {
	out, err := a.runner.Run(ctx, "gh", args...)
	if err == nil {
		return out, nil
	}

	command := "gh " + strings.Join(args, " ")
	output := strings.TrimSpace(string(out))
	if output == "" {
		return nil, fmt.Errorf("%s failed: %w", command, err)
	}
	return nil, fmt.Errorf("%s failed: %w: %s", command, err, output)
}

func safeSnippet(raw []byte) string {
	const limit = 160
	snippet := strings.TrimSpace(string(raw))
	if len(snippet) <= limit {
		return snippet
	}
	return snippet[:limit] + "..."
}

func releaseRunFromWorkflowRun(run workflowRun) workspace.ReleaseRun {
	return workspace.ReleaseRun{
		Name:      strings.TrimSpace(run.DisplayTitle),
		Event:     strings.TrimSpace(run.Event),
		URL:       strings.TrimSpace(run.HTMLURL),
		UpdatedAt: parseGitHubTimestamp(run.UpdatedAt),
	}
}

func parseGitHubTimestamp(raw string) time.Time {
	text := strings.TrimSpace(raw)
	if text == "" {
		return time.Time{}
	}
	ts, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return time.Time{}
	}
	return ts
}

type commandRunner struct{}

func (commandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

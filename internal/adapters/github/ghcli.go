package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

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

	if err := a.CheckAuth(ctx); err != nil {
		return workspace.RepoStatus{}, err
	}

	prStatus, err := a.fetchPullRequestStatus(ctx, repo)
	if err != nil {
		return workspace.RepoStatus{}, err
	}

	ciStatus, err := a.fetchLatestWorkflowStatus(ctx, repo.Name, "", repo.DefaultBranch)
	if err != nil {
		return workspace.RepoStatus{}, err
	}

	releaseStatus := workspace.StatusUnconfigured
	if strings.TrimSpace(repo.ReleaseWorkflowRef) != "" {
		releaseStatus, err = a.fetchLatestWorkflowStatus(ctx, repo.Name, repo.ReleaseWorkflowRef, repo.DefaultBranch)
		if err != nil {
			return workspace.RepoStatus{}, err
		}
	}

	return workspace.RepoStatus{
		PR:      prStatus,
		CI:      ciStatus,
		Release: releaseStatus,
	}, nil
}

type pullRequest struct {
	State string `json:"state"`
}

func (a *Adapter) fetchPullRequestStatus(ctx context.Context, repo workspace.Repo) (workspace.Status, error) {
	args := []string{
		"pr", "list",
		"--repo", repo.Name,
		"--state", "open",
	}
	if strings.TrimSpace(repo.DefaultBranch) != "" {
		args = append(args, "--base", repo.DefaultBranch)
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

type workflowRunsResponse struct {
	WorkflowRuns []workflowRun `json:"workflow_runs"`
}

type workflowRun struct {
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

func (a *Adapter) fetchLatestWorkflowStatus(ctx context.Context, repoName, workflowRef, branch string) (workspace.Status, error) {
	endpoint := workflowRunsEndpoint(repoName, workflowRef, branch)
	out, err := a.runGH(ctx, "api", endpoint)
	if err != nil {
		return workspace.StatusNeutral, err
	}

	var runs workflowRunsResponse
	if err := json.Unmarshal(out, &runs); err != nil {
		return workspace.StatusNeutral, fmt.Errorf("parse gh workflow runs output for endpoint %q: %w (raw=%q)", endpoint, err, safeSnippet(out))
	}
	if len(runs.WorkflowRuns) == 0 {
		return workspace.StatusNeutral, nil
	}

	latest := runs.WorkflowRuns[0]
	return workspace.StatusFromGitHubRun(latest.Status, latest.Conclusion), nil
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

type commandRunner struct{}

func (commandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

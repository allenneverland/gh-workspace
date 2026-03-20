package workspace

import "testing"

func TestStatusFromGitHubRun_MapsWorkflowStatesAndConclusions(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		conclusion string
		want       Status
	}{
		{
			name:       "success conclusion maps to success",
			status:     "completed",
			conclusion: "success",
			want:       StatusSuccess,
		},
		{
			name:       "failure conclusion maps to failure",
			status:     "completed",
			conclusion: "failure",
			want:       StatusFailure,
		},
		{
			name:       "cancelled conclusion maps to neutral",
			status:     "completed",
			conclusion: "cancelled",
			want:       StatusNeutral,
		},
		{
			name:       "skipped conclusion maps to neutral",
			status:     "completed",
			conclusion: "skipped",
			want:       StatusNeutral,
		},
		{
			name:   "running status maps to in progress",
			status: "running",
			want:   StatusInProgress,
		},
		{
			name:   "in progress status maps to in progress",
			status: "in_progress",
			want:   StatusInProgress,
		},
		{
			name:   "queued status maps to in progress",
			status: "queued",
			want:   StatusInProgress,
		},
		{
			name:   "direct success status maps to success",
			status: "success",
			want:   StatusSuccess,
		},
		{
			name:   "direct failure status maps to failure",
			status: "failure",
			want:   StatusFailure,
		},
		{
			name:   "unknown values default to neutral",
			status: "completed",
			want:   StatusNeutral,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StatusFromGitHubRun(tt.status, tt.conclusion); got != tt.want {
				t.Fatalf("StatusFromGitHubRun(%q, %q) = %q, want %q", tt.status, tt.conclusion, got, tt.want)
			}
		})
	}
}

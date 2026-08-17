//go:build !integration

package main

import (
	"context"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestLabelPolicyKeepsEventsByDefault(t *testing.T) {
	policy := testLabelPolicy(t, nil)

	if reason := policy.FilterReason(eventLabels{
		Repository: "User/Repo",
		Branch:     "feature/login",
		Workflow:   "CI",
	}); reason != "" {
		t.Fatalf("expected default policy to keep the event, got reason %q", reason)
	}
}

func TestLabelPolicyFiltersRepositories(t *testing.T) {
	policy := testLabelPolicy(t, func(policy *labelPolicy) {
		policy.repoAllow = parseSetList("acme/api, acme/web")
		policy.repoDeny = parseSetList("acme/web")
	})

	testCases := []struct {
		name       string
		repository string
		wantReason string
	}{
		{name: "allowlist miss", repository: "acme/docs", wantReason: filterReasonRepository},
		{name: "denylist hit", repository: "acme/web", wantReason: filterReasonRepository},
		{name: "allowlist hit", repository: "ACME/api", wantReason: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := policy.FilterReason(eventLabels{Repository: tc.repository, Branch: "main"})
			if got != tc.wantReason {
				t.Fatalf("FilterReason() = %q, want %q", got, tc.wantReason)
			}
		})
	}
}

func TestLabelPolicyFiltersBranchesAndWorkflows(t *testing.T) {
	policy := testLabelPolicy(t, func(policy *labelPolicy) {
		policy.branchAllow = regexp.MustCompile(`^(main|release/.+)$`)
		policy.branchDeny = regexp.MustCompile(`^release/legacy$`)
		policy.workflowAllow = regexp.MustCompile(`^(CI|Release)$`)
		policy.workflowDeny = regexp.MustCompile(`^Release$`)
	})

	testCases := []struct {
		name       string
		labels     eventLabels
		wantReason string
	}{
		{
			name:       "branch allow miss",
			labels:     eventLabels{Repository: "acme/api", Branch: "feature/login", Workflow: "CI"},
			wantReason: filterReasonBranch,
		},
		{
			name:       "branch deny hit",
			labels:     eventLabels{Repository: "acme/api", Branch: "release/legacy", Workflow: "CI"},
			wantReason: filterReasonBranch,
		},
		{
			name:       "workflow allow miss",
			labels:     eventLabels{Repository: "acme/api", Branch: "main", Workflow: "Nightly"},
			wantReason: filterReasonWorkflow,
		},
		{
			name:       "workflow deny hit",
			labels:     eventLabels{Repository: "acme/api", Branch: "main", Workflow: "Release"},
			wantReason: filterReasonWorkflow,
		},
		{
			name:       "push without workflow skips workflow filters",
			labels:     eventLabels{Repository: "acme/api", Branch: "main"},
			wantReason: "",
		},
		{
			name:       "kept workflow event",
			labels:     eventLabels{Repository: "acme/api", Branch: "main", Workflow: "CI"},
			wantReason: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := policy.FilterReason(tc.labels)
			if got != tc.wantReason {
				t.Fatalf("FilterReason() = %q, want %q", got, tc.wantReason)
			}
		})
	}
}

func TestLabelPolicyNormalizesBranches(t *testing.T) {
	policy := testLabelPolicy(t, func(policy *labelPolicy) {
		policy.normalizeBranches = true
	})

	testCases := []struct {
		branch string
		want   string
	}{
		{branch: "main", want: branchClassDefault},
		{branch: "Master", want: branchClassDefault},
		{branch: "release/1.4", want: branchClassRelease},
		{branch: "hotfix/login", want: branchClassRelease},
		{branch: "feature/login", want: branchClassFeature},
		{branch: "dependabot/npm/lodash", want: branchClassFeature},
		{branch: "", want: ""},
	}

	for _, tc := range testCases {
		name := tc.branch
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			got := policy.NormalizeBranch(tc.branch)
			if got != tc.want {
				t.Fatalf("NormalizeBranch(%q) = %q, want %q", tc.branch, got, tc.want)
			}
		})
	}

	raw := testLabelPolicy(t, nil)
	if got := raw.NormalizeBranch("feature/login"); got != "feature/login" {
		t.Fatalf("raw policy should preserve branch names, got %q", got)
	}
}

func TestBranchFromRef(t *testing.T) {
	testCases := []struct {
		ref  string
		want string
	}{
		{ref: "refs/heads/main", want: "main"},
		{ref: "refs/heads/feature/login", want: "feature/login"},
		{ref: "refs/tags/v1.2.3", want: "v1.2.3"},
		{ref: "main", want: "main"},
	}

	for _, tc := range testCases {
		if got := branchFromRef(tc.ref); got != tc.want {
			t.Fatalf("branchFromRef(%q) = %q, want %q", tc.ref, got, tc.want)
		}
	}
}

func TestLoadLabelPolicyFromEnv(t *testing.T) {
	clearLabelPolicyEnv(t)

	policy, err := loadLabelPolicyFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if policy.normalizeBranches {
		t.Fatal("expected branch normalization to be off by default")
	}
	if _, ok := policy.defaultBranches["main"]; !ok {
		t.Fatal("expected default branch list to include main")
	}

	t.Setenv(envRepoAllowlist, "acme/api, acme/web")
	t.Setenv(envRepoDenylist, "acme/legacy")
	t.Setenv(envBranchAllowRegex, `^(main|release/.+)$`)
	t.Setenv(envBranchDenyRegex, `^dependabot/`)
	t.Setenv(envWorkflowAllowRegex, `^CI$`)
	t.Setenv(envWorkflowDenyRegex, `^Nightly$`)
	t.Setenv(envNormalizeBranches, "true")
	t.Setenv(envDefaultBranches, "main,develop")
	t.Setenv(envReleaseBranchRegex, `^release/.+`)

	policy, err = loadLabelPolicyFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !policy.normalizeBranches {
		t.Fatal("expected branch normalization to be enabled")
	}
	if _, ok := policy.repoAllow["acme/api"]; !ok {
		t.Fatal("expected allowlist to include acme/api")
	}
	if _, ok := policy.defaultBranches["develop"]; !ok {
		t.Fatal("expected custom default branches to include develop")
	}
	if policy.NormalizeBranch("release/2") != branchClassRelease {
		t.Fatalf("expected custom release regex to classify release/2")
	}
}

func TestLoadLabelPolicyFromEnvRejectsInvalidConfig(t *testing.T) {
	clearLabelPolicyEnv(t)
	t.Setenv(envBranchAllowRegex, "(")

	if _, err := loadLabelPolicyFromEnv(); err == nil {
		t.Fatal("expected invalid branch regex to fail")
	}

	clearLabelPolicyEnv(t)
	t.Setenv(envNormalizeBranches, "maybe")
	if _, err := loadLabelPolicyFromEnv(); err == nil {
		t.Fatal("expected invalid boolean to fail")
	}
}

func TestApplyLabelPolicyFiltersAndNormalizesEvents(t *testing.T) {
	useInMemoryStateBackends(t)
	filteredEventsCounter.Reset()
	workflowStatusCounter.Reset()
	workflowQueuedGauge.Reset()
	workflowInProgressGauge.Reset()
	workflowCompletedGauge.Reset()
	workflowDurationHistogram.Reset()

	useLabelPolicy(t, testLabelPolicy(t, func(policy *labelPolicy) {
		policy.repoDeny = parseSetList("user/repo")
	}))

	body, err := os.ReadFile("../test_data/workflow_run.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	updateWorkflowMetrics(context.Background(), body)

	if err := testutil.CollectAndCompare(filteredEventsCounter, strings.NewReader(`
		# HELP promgithub_event_filtered_total Total number of webhook events dropped by the configured label policy
		# TYPE promgithub_event_filtered_total counter
		promgithub_event_filtered_total{event_type="workflow_run",reason="repository"} 1
	`)); err != nil {
		t.Fatalf("unexpected filtered metric: %v", err)
	}
	if got := testutil.CollectAndCount(workflowStatusCounter); got != 0 {
		t.Fatalf("expected denied repository to skip workflow metrics, got %d series", got)
	}

	filteredEventsCounter.Reset()
	useLabelPolicy(t, testLabelPolicy(t, func(policy *labelPolicy) {
		policy.normalizeBranches = true
	}))
	updateWorkflowMetrics(context.Background(), body)

	if err := testutil.CollectAndCompare(workflowStatusCounter, strings.NewReader(`
		# HELP promgithub_workflow_status Total number of workflow runs with status
		# TYPE promgithub_workflow_status counter
		promgithub_workflow_status{branch="default",conclusion="success",repository="user/repo",workflow_name="CI",workflow_status="completed"} 1
	`)); err != nil {
		t.Fatalf("unexpected normalized workflow metric: %v", err)
	}
}

func TestParseEnvBool(t *testing.T) {
	t.Setenv("PROMGITHUB_TEST_BOOL", "")
	got, err := parseEnvBool("PROMGITHUB_TEST_BOOL", true)
	if err != nil || !got {
		t.Fatalf("empty value should use default true, got %v err %v", got, err)
	}

	t.Setenv("PROMGITHUB_TEST_BOOL", "YES")
	got, err = parseEnvBool("PROMGITHUB_TEST_BOOL", false)
	if err != nil || !got {
		t.Fatalf("YES should parse as true, got %v err %v", got, err)
	}

	t.Setenv("PROMGITHUB_TEST_BOOL", "off")
	got, err = parseEnvBool("PROMGITHUB_TEST_BOOL", true)
	if err != nil || got {
		t.Fatalf("off should parse as false, got %v err %v", got, err)
	}
}

func clearLabelPolicyEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		envRepoAllowlist,
		envRepoDenylist,
		envBranchAllowRegex,
		envBranchDenyRegex,
		envWorkflowAllowRegex,
		envWorkflowDenyRegex,
		envNormalizeBranches,
		envDefaultBranches,
		envReleaseBranchRegex,
	} {
		t.Setenv(key, "")
	}
}

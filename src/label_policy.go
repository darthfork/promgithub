package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

const (
	filterReasonRepository = "repository"
	filterReasonBranch     = "branch"
	filterReasonWorkflow   = "workflow"

	branchClassDefault = "default"
	branchClassRelease = "release"
	branchClassFeature = "feature"

	envRepoAllowlist       = "PROMGITHUB_REPO_ALLOWLIST"
	envRepoDenylist        = "PROMGITHUB_REPO_DENYLIST"
	envBranchAllowRegex    = "PROMGITHUB_BRANCH_ALLOW_REGEX"
	envBranchDenyRegex     = "PROMGITHUB_BRANCH_DENY_REGEX"
	envWorkflowAllowRegex  = "PROMGITHUB_WORKFLOW_ALLOW_REGEX"
	envWorkflowDenyRegex   = "PROMGITHUB_WORKFLOW_DENY_REGEX"
	envNormalizeBranches   = "PROMGITHUB_NORMALIZE_BRANCHES"
	envDefaultBranches     = "PROMGITHUB_DEFAULT_BRANCHES"
	envReleaseBranchRegex  = "PROMGITHUB_RELEASE_BRANCH_REGEX"
	defaultBranchList      = "main,master"
	defaultReleaseBranchRE = `^(release/|hotfix/).+`
	refsHeadsPrefix        = "refs/heads/"
	refsTagsPrefix         = "refs/tags/"
)

type eventLabels struct {
	Repository string
	Branch     string
	Workflow   string
}

type labelPolicy struct {
	repoAllow         map[string]struct{}
	repoDeny          map[string]struct{}
	branchAllow       *regexp.Regexp
	branchDeny        *regexp.Regexp
	workflowAllow     *regexp.Regexp
	workflowDeny      *regexp.Regexp
	normalizeBranches bool
	defaultBranches   map[string]struct{}
	releaseBranch     *regexp.Regexp
}

var defaultLabelPolicy labelPolicy

func loadLabelPolicyFromEnv() (labelPolicy, error) {
	policy := labelPolicy{
		repoAllow:       parseSetList(os.Getenv(envRepoAllowlist)),
		repoDeny:        parseSetList(os.Getenv(envRepoDenylist)),
		defaultBranches: parseSetList(getEnvOrDefault(envDefaultBranches, defaultBranchList)),
	}

	var err error
	if policy.branchAllow, err = compileOptionalRegex(envBranchAllowRegex); err != nil {
		return labelPolicy{}, err
	}
	if policy.branchDeny, err = compileOptionalRegex(envBranchDenyRegex); err != nil {
		return labelPolicy{}, err
	}
	if policy.workflowAllow, err = compileOptionalRegex(envWorkflowAllowRegex); err != nil {
		return labelPolicy{}, err
	}
	if policy.workflowDeny, err = compileOptionalRegex(envWorkflowDenyRegex); err != nil {
		return labelPolicy{}, err
	}

	policy.normalizeBranches, err = parseEnvBool(envNormalizeBranches, false)
	if err != nil {
		return labelPolicy{}, err
	}

	releasePattern := strings.TrimSpace(os.Getenv(envReleaseBranchRegex))
	if releasePattern == "" {
		releasePattern = defaultReleaseBranchRE
	}
	policy.releaseBranch, err = regexp.Compile(releasePattern)
	if err != nil {
		return labelPolicy{}, fmt.Errorf("parse %s: %w", envReleaseBranchRegex, err)
	}

	return policy, nil
}

func (p labelPolicy) FilterReason(labels eventLabels) string {
	repository := normalizeRepoName(labels.Repository)
	if len(p.repoAllow) > 0 {
		if _, ok := p.repoAllow[repository]; !ok {
			return filterReasonRepository
		}
	}
	if _, denied := p.repoDeny[repository]; denied {
		return filterReasonRepository
	}

	branch := strings.TrimSpace(labels.Branch)
	if p.branchAllow != nil && !p.branchAllow.MatchString(branch) {
		return filterReasonBranch
	}
	if p.branchDeny != nil && p.branchDeny.MatchString(branch) {
		return filterReasonBranch
	}

	workflow := strings.TrimSpace(labels.Workflow)
	if workflow == "" {
		return ""
	}
	if p.workflowAllow != nil && !p.workflowAllow.MatchString(workflow) {
		return filterReasonWorkflow
	}
	if p.workflowDeny != nil && p.workflowDeny.MatchString(workflow) {
		return filterReasonWorkflow
	}

	return ""
}

func (p labelPolicy) NormalizeBranch(branch string) string {
	branch = strings.TrimSpace(branch)
	if !p.normalizeBranches || branch == "" {
		return branch
	}

	if _, ok := p.defaultBranches[strings.ToLower(branch)]; ok {
		return branchClassDefault
	}
	if p.releaseBranch != nil && p.releaseBranch.MatchString(branch) {
		return branchClassRelease
	}

	return branchClassFeature
}

func applyLabelPolicy(eventType string, labels eventLabels) (eventLabels, bool) {
	if reason := defaultLabelPolicy.FilterReason(labels); reason != "" {
		defaultMetricRecorder.RecordFilteredEvent(eventType, reason)
		if logger != nil {
			logger.Debug("Dropping event due to label policy",
				zap.String("eventType", eventType),
				zap.String("reason", reason),
				zap.String(filterReasonRepository, labels.Repository),
				zap.String(filterReasonBranch, labels.Branch),
				zap.String(filterReasonWorkflow, labels.Workflow),
			)
		}
		return labels, false
	}

	labels.Branch = defaultLabelPolicy.NormalizeBranch(labels.Branch)
	return labels, true
}

func branchFromRef(ref string) string {
	ref = strings.TrimSpace(ref)
	switch {
	case strings.HasPrefix(ref, refsHeadsPrefix):
		return strings.TrimPrefix(ref, refsHeadsPrefix)
	case strings.HasPrefix(ref, refsTagsPrefix):
		return strings.TrimPrefix(ref, refsTagsPrefix)
	default:
		return ref
	}
}

func normalizeRepoName(repository string) string {
	return strings.ToLower(strings.TrimSpace(repository))
}

func parseSetList(value string) map[string]struct{} {
	items := strings.Split(value, ",")
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		set[item] = struct{}{}
	}
	return set
}

func compileOptionalRegex(key string) (*regexp.Regexp, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil, nil
	}

	compiled, err := regexp.Compile(value)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", key, err)
	}
	return compiled, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	return value
}

func logLabelPolicy(logger *zap.Logger, policy labelPolicy) {
	if logger == nil {
		return
	}

	logger.Info("Label policy loaded",
		zap.Int("repoAllowlist", len(policy.repoAllow)),
		zap.Int("repoDenylist", len(policy.repoDeny)),
		zap.Bool("branchAllowRegex", policy.branchAllow != nil),
		zap.Bool("branchDenyRegex", policy.branchDeny != nil),
		zap.Bool("workflowAllowRegex", policy.workflowAllow != nil),
		zap.Bool("workflowDenyRegex", policy.workflowDeny != nil),
		zap.Bool("normalizeBranches", policy.normalizeBranches),
	)
}

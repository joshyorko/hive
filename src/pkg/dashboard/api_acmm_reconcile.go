package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	gh "github.com/google/go-github/v72/github"
)

const acmmIssueMarkerPrefix = "<!-- hive-acmm-remediation:"

// Automatic reconciliation shares the governor's existing evaluation loop but
// is deliberately slower than a normal governor tick. This keeps repository
// evidence fresh enough to retire healed findings without creating a second
// scheduler or a tight GitHub API loop.
const acmmAutomaticReconcileInterval = time.Hour

type ACMMReconcileRequest struct {
	Repos  []string `json:"repos,omitempty"`
	DryRun bool     `json:"dry_run"`
}

type ACMMReconcileIssue struct {
	Repo                  string `json:"repo"`
	IssueNumber           int    `json:"issue_number"`
	CriterionID           string `json:"criterion_id"`
	CanonicalIssue        int    `json:"canonical_issue,omitempty"`
	CurrentPassed         bool   `json:"current_passed"`
	EvaluatorGapCandidate bool   `json:"evaluator_gap_candidate,omitempty"`
	Disposition           string `json:"disposition"`
	Reason                string `json:"reason"`
	EvaluatedRef          string `json:"evaluated_ref,omitempty"`
	MutationApplied       bool   `json:"mutation_applied"`
}

type ACMMReconcileRepo struct {
	Repo         string               `json:"repo"`
	EvaluatedRef string               `json:"evaluated_ref,omitempty"`
	Issues       []ACMMReconcileIssue `json:"issues"`
	Error        string               `json:"error,omitempty"`
}

type ACMMReconcileResponse struct {
	DryRun bool                `json:"dry_run"`
	Repos  []ACMMReconcileRepo `json:"repos"`
}

// ReconcileACMMAutomatically performs one cadence-bounded, fresh-evidence pass
// over the configured project repositories. It is called by the existing
// governor evaluation loop; the explicit owner API remains available for an
// immediate operator-requested pass.
func (s *Server) ReconcileACMMAutomatically(ctx context.Context, now time.Time) (bool, ACMMReconcileResponse) {
	response := ACMMReconcileResponse{DryRun: false}
	if s.deps == nil || s.deps.Config == nil || s.deps.GHClient == nil || s.deps.GHClient.GoGitHub() == nil {
		return false, response
	}

	s.acmmAutomaticMu.Lock()
	defer s.acmmAutomaticMu.Unlock()
	if !s.acmmAutomaticAt.IsZero() && now.Sub(s.acmmAutomaticAt) < acmmAutomaticReconcileInterval {
		return false, response
	}

	passCtx, cancel := context.WithTimeout(ctx, acmmReconcileTimeout)
	defer cancel()
	s.acmmMutationMu.Lock()
	for _, repo := range s.deps.Config.Project.Repos {
		result, err := s.reconcileACMMRepo(passCtx, s.deps.Config.Project.Org, repo, false)
		if err != nil {
			result = ACMMReconcileRepo{Repo: s.deps.Config.Project.Org + "/" + repo, Error: err.Error()}
		}
		response.Repos = append(response.Repos, result)
	}
	s.acmmMutationMu.Unlock()
	s.acmmAutomaticAt = now

	mutations := 0
	for _, repoResult := range response.Repos {
		for _, issue := range repoResult.Issues {
			if issue.MutationApplied {
				mutations++
				s.AuditLog("system", "acmm_reconcile_issue", auditDetail(
					"repo", issue.Repo, "number", fmt.Sprintf("%d", issue.IssueNumber),
					"criterion", issue.CriterionID, "disposition", issue.Disposition), "")
			}
		}
	}
	s.AuditLog("system", "acmm_reconcile", auditDetail(
		"trigger", "governor", "repos", fmt.Sprintf("%d", len(response.Repos)),
		"mutations", fmt.Sprintf("%d", mutations)), "")
	return true, response
}

func acmmCriterionByID(id string) *ACMMCriterion {
	for i := range universalCriteria {
		if universalCriteria[i].ID == id {
			return &universalCriteria[i]
		}
	}
	return nil
}

func acmmIssueMarker(repo, criterion string) string {
	return fmt.Sprintf("%s repo=%s criterion=%s -->", acmmIssueMarkerPrefix,
		url.QueryEscape(repo), url.QueryEscape(criterion))
}

func parseACMMIssueIdentity(body string) (repo, criterion string, ok bool) {
	if start := strings.Index(body, acmmIssueMarkerPrefix); start >= 0 {
		end := strings.Index(body[start:], "-->")
		if end >= 0 {
			marker := strings.TrimSpace(body[start+len(acmmIssueMarkerPrefix) : start+end])
			values, err := url.ParseQuery(strings.ReplaceAll(marker, " ", "&"))
			if err == nil {
				repo, _ = url.QueryUnescape(values.Get("repo"))
				criterion, _ = url.QueryUnescape(values.Get("criterion"))
				if repo != "" && criterion != "" {
					return repo, criterion, true
				}
			}
		}
	}

	// Backward compatibility for issues created before the durable marker.
	const legacy = "**Criterion ID:** `"
	start := strings.Index(body, legacy)
	if start < 0 {
		return "", "", false
	}
	start += len(legacy)
	end := strings.Index(body[start:], "`")
	if end <= 0 {
		return "", "", false
	}
	return "", strings.TrimSpace(body[start : start+end]), true
}

func buildACMMIssueBody(repo string, criterion ACMMCriterion) string {
	levelName := acmmLevelNames[criterion.Level]
	if levelName == "" {
		levelName = "Unknown"
	}
	var patterns strings.Builder
	for _, pattern := range criterion.Patterns {
		fmt.Fprintf(&patterns, "- `%s`\n", pattern)
	}
	return fmt.Sprintf("%s\n\n## ACMM Gap: %s\n\n"+
		"**Level:** L%d %s\n**Category:** %s\n**Criterion ID:** `%s`\n\n"+
		"### Current evaluator evidence\n\nThe current ACMM evaluator did not detect any supported evidence path:\n\n%s\n"+
		"This issue tracks the capability gap or an evaluator-detection gap. Implement the capability using the repository's conventions; do not add empty placeholder files solely to satisfy detection.\n\n"+
		"### Why it matters\n\n%s\n\n---\n*Opened by Hive ACMM Evaluation*",
		acmmIssueMarker(repo, criterion.ID), criterion.Name, criterion.Level, levelName,
		criterion.Category, criterion.ID, patterns.String(),
		acmmCriterionWhyItMatters(criterion.Level, criterion.Category))
}

func githubStatus(err error) int {
	var response *gh.ErrorResponse
	if errors.As(err, &response) && response.Response != nil {
		return response.Response.StatusCode
	}
	return 0
}

// evaluateACMMRepoFresh deliberately bypasses the one-hour display cache. It
// is used only by mutation paths and returns an error for inaccessible or
// indeterminate source evidence instead of turning it into a false failure.
func (s *Server) evaluateACMMRepoFresh(ctx context.Context, owner, repo string) (RepoEvaluation, string, error) {
	client := s.deps.GHClient.GoGitHub()
	if client == nil {
		return RepoEvaluation{}, "", fmt.Errorf("GitHub client not initialized")
	}

	dirs := make(map[string]struct{})
	for _, criterion := range universalCriteria {
		for _, pattern := range criterion.Patterns {
			clean := strings.TrimSuffix(pattern, "/")
			parent := ""
			if idx := strings.LastIndex(clean, "/"); idx >= 0 {
				parent = clean[:idx]
			}
			dirs[parent] = struct{}{}
		}
	}
	cache := make(map[string]map[string]bool, len(dirs))
	for dir := range dirs {
		_, contents, _, err := client.Repositories.GetContents(ctx, owner, repo, dir, nil)
		if err != nil {
			if githubStatus(err) == http.StatusNotFound {
				cache[dir] = map[string]bool{}
				continue
			}
			return RepoEvaluation{}, "", fmt.Errorf("reading %s/%s:%s: %w", owner, repo, dir, err)
		}
		entries := make(map[string]bool, len(contents)*2)
		for _, entry := range contents {
			entries[entry.GetName()] = true
			if entry.GetType() == "dir" {
				entries[entry.GetName()+"/"] = true
			}
		}
		cache[dir] = entries
	}

	results := make([]CriterionResult, 0, len(universalCriteria))
	for _, criterion := range universalCriteria {
		passed := s.checkCriterion(ctx, owner, repo, criterion, cache)
		results = append(results, CriterionResult{ID: criterion.ID, Name: criterion.Name,
			Level: criterion.Level, Category: criterion.Category, Patterns: criterion.Patterns,
			Passed: passed, Repo: repo})
	}
	scored := s.scoreResults(results)
	repoEval := RepoEvaluation{Repo: repo, CodebaseLevel: scored.CodebaseLevel,
		LevelName: scored.CodebaseLevelName, CriteriaTotal: scored.CriteriaTotal,
		CriteriaPassed: scored.CriteriaPassed, Levels: scored.Levels, CriteriaResults: results}

	ref := ""
	if commit, _, err := client.Repositories.GetCommit(ctx, owner, repo, "HEAD", nil); err == nil {
		ref = commit.GetSHA()
	}
	return repoEval, ref, nil
}

func evaluatorGapCandidate(criterion ACMMCriterion) bool {
	if len(criterion.Patterns) == 0 {
		return false
	}
	for _, pattern := range criterion.Patterns {
		lower := strings.ToLower(pattern)
		if !strings.Contains(lower, "claude") && !strings.Contains(lower, "cursor") && !strings.Contains(lower, "copilot") {
			return false
		}
	}
	return true
}

func (s *Server) listACMMIssues(ctx context.Context, owner, repo string) ([]*gh.Issue, error) {
	client := s.deps.GHClient.GoGitHub()
	var all []*gh.Issue
	opts := &gh.IssueListByRepoOptions{State: "all", Labels: []string{acmmIssueLabelName},
		ListOptions: gh.ListOptions{PerPage: 100}}
	for {
		issues, response, err := client.Issues.ListByRepo(ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}
		for _, issue := range issues {
			if issue.PullRequestLinks == nil {
				all = append(all, issue)
			}
		}
		if response == nil || response.NextPage == 0 {
			break
		}
		opts.ListOptions.Page = response.NextPage
	}
	return all, nil
}

func (s *Server) closeACMMIssue(ctx context.Context, owner, repo string, issueNumber int, stateReason string, canonicalID int64) error {
	client := s.deps.GHClient.GoGitHub()
	if stateReason != "duplicate" {
		_, _, err := client.Issues.Edit(ctx, owner, repo, issueNumber,
			&gh.IssueRequest{State: gh.Ptr("closed"), StateReason: gh.Ptr(stateReason)})
		return err
	}
	if canonicalID == 0 {
		return fmt.Errorf("canonical issue database ID is unavailable")
	}
	// GitHub's duplicate lifecycle requires duplicate_issue_id. go-github v72's
	// IssueRequest predates that field, so use the client's authenticated REST
	// transport with the current documented request shape.
	path := fmt.Sprintf("repos/%s/%s/issues/%d", owner, repo, issueNumber)
	req, err := client.NewRequest(http.MethodPatch, path, map[string]any{
		"state": "closed", "state_reason": "duplicate", "duplicate_issue_id": canonicalID,
	})
	if err != nil {
		return err
	}
	_, err = client.Do(ctx, req, new(gh.Issue))
	return err
}

func (s *Server) reconcileACMMRepo(ctx context.Context, owner, repo string, dryRun bool) (ACMMReconcileRepo, error) {
	evaluation, ref, err := s.evaluateACMMRepoFresh(ctx, owner, repo)
	if err != nil {
		return ACMMReconcileRepo{}, err
	}
	passed := make(map[string]bool, len(evaluation.CriteriaResults))
	for _, result := range evaluation.CriteriaResults {
		passed[result.ID] = result.Passed
	}
	issues, err := s.listACMMIssues(ctx, owner, repo)
	if err != nil {
		return ACMMReconcileRepo{}, err
	}

	groups := make(map[string][]*gh.Issue)
	fullRepo := owner + "/" + repo
	for _, issue := range issues {
		markedRepo, criterion, ok := parseACMMIssueIdentity(issue.GetBody())
		if !ok || (markedRepo != "" && !strings.EqualFold(markedRepo, fullRepo)) {
			continue
		}
		groups[criterion] = append(groups[criterion], issue)
	}
	var ids []string
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	result := ACMMReconcileRepo{Repo: fullRepo, EvaluatedRef: ref}
	for _, id := range ids {
		criterion := acmmCriterionByID(id)
		group := groups[id]
		sort.Slice(group, func(i, j int) bool { return group[i].GetNumber() < group[j].GetNumber() })
		if criterion == nil {
			for _, issue := range group {
				if strings.EqualFold(issue.GetState(), "open") {
					result.Issues = append(result.Issues, ACMMReconcileIssue{Repo: fullRepo, IssueNumber: issue.GetNumber(),
						CriterionID: id, Disposition: "evaluator_gap", Reason: "criterion is no longer known to the current evaluator", EvaluatedRef: ref})
				}
			}
			continue
		}

		var open []*gh.Issue
		for _, issue := range group {
			if strings.EqualFold(issue.GetState(), "open") {
				open = append(open, issue)
			} else if strings.EqualFold(issue.GetStateReason(), "not_planned") {
				result.Issues = append(result.Issues, ACMMReconcileIssue{Repo: fullRepo, IssueNumber: issue.GetNumber(),
					CriterionID: id, CurrentPassed: passed[id], Disposition: "human_dispositioned",
					Reason: "human closed this remediation as not planned; Hive will not recreate it automatically", EvaluatedRef: ref})
			}
		}
		if len(open) == 0 {
			continue
		}
		canonical := open[0].GetNumber()
		canonicalID := open[0].GetID()
		for idx, issue := range open {
			disposition := "still_failing"
			reason := "criterion still fails according to current evaluator evidence"
			stateReason := ""
			if idx > 0 {
				disposition = "duplicate"
				reason = fmt.Sprintf("duplicates canonical ACMM remediation issue #%d", canonical)
				stateReason = "duplicate"
			} else if passed[id] {
				disposition = "satisfied"
				reason = "criterion passes according to current evaluator evidence"
				stateReason = "completed"
			} else if evaluatorGapCandidate(*criterion) {
				disposition = "evaluator_gap"
				reason = "criterion uses tool-specific evidence paths; semantic satisfaction requires evaluator follow-up"
			}

			row := ACMMReconcileIssue{Repo: fullRepo, IssueNumber: issue.GetNumber(), CriterionID: id,
				CanonicalIssue: canonical, CurrentPassed: passed[id], EvaluatorGapCandidate: evaluatorGapCandidate(*criterion),
				Disposition: disposition, Reason: reason, EvaluatedRef: ref}
			if !dryRun && stateReason != "" {
				receipt := fmt.Sprintf("Hive ACMM reconciliation\n\n- Criterion: `%s`\n- Repository: `%s`\n- Evaluated ref: `%s`\n- Disposition: **%s**\n- Reason: %s\n\n%s",
					id, fullRepo, ref, disposition, reason, acmmIssueMarker(fullRepo, id))
				if _, _, err := s.deps.GHClient.GoGitHub().Issues.CreateComment(ctx, owner, repo, issue.GetNumber(), &gh.IssueComment{Body: gh.Ptr(receipt)}); err != nil {
					return result, fmt.Errorf("commenting on %s#%d: %w", fullRepo, issue.GetNumber(), err)
				}
				if err := s.closeACMMIssue(ctx, owner, repo, issue.GetNumber(), stateReason, canonicalID); err != nil {
					return result, fmt.Errorf("closing %s#%d: %w", fullRepo, issue.GetNumber(), err)
				}
				row.MutationApplied = true
			}
			result.Issues = append(result.Issues, row)
		}
	}
	return result, nil
}

func (s *Server) handleACMMReconcile(w http.ResponseWriter, r *http.Request) {
	if !requireOwnerRole(w, r) {
		return
	}
	var req ACMMReconcileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if s.deps == nil || s.deps.Config == nil || s.deps.GHClient == nil || s.deps.GHClient.GoGitHub() == nil {
		http.Error(w, "GitHub/config unavailable", http.StatusInternalServerError)
		return
	}
	repos := req.Repos
	if len(repos) == 0 {
		repos = append([]string(nil), s.deps.Config.Project.Repos...)
	}
	allowed := make(map[string]bool)
	for _, repo := range s.deps.Config.Project.Repos {
		allowed[repo] = true
	}
	if primary := s.deps.Config.Project.PrimaryRepo; primary != "" {
		allowed[primary] = true
	}
	ctx, cancel := context.WithTimeout(r.Context(), acmmReconcileTimeout)
	defer cancel()
	response := ACMMReconcileResponse{DryRun: req.DryRun}
	s.acmmMutationMu.Lock()
	defer s.acmmMutationMu.Unlock()
	for _, repo := range repos {
		if !allowed[repo] {
			response.Repos = append(response.Repos, ACMMReconcileRepo{Repo: repo, Error: "repository is outside configured project boundary"})
			continue
		}
		result, err := s.reconcileACMMRepo(ctx, s.deps.Config.Project.Org, repo, req.DryRun)
		if err != nil {
			result = ACMMReconcileRepo{Repo: s.deps.Config.Project.Org + "/" + repo, Error: err.Error()}
		}
		response.Repos = append(response.Repos, result)
	}
	if !req.DryRun {
		for _, repoResult := range response.Repos {
			for _, issue := range repoResult.Issues {
				if issue.MutationApplied {
					s.auditFromRequest(r, "acmm_reconcile_issue", auditDetail(
						"repo", issue.Repo, "number", fmt.Sprintf("%d", issue.IssueNumber),
						"criterion", issue.CriterionID, "disposition", issue.Disposition), "")
				}
			}
		}
		s.auditFromRequest(r, "acmm_reconcile", auditDetail("repos", fmt.Sprintf("%d", len(response.Repos))), "")
	}
	jsonResponse(w, response)
}

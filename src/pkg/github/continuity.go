package github

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	gh "github.com/google/go-github/v72/github"
	"github.com/kubestellar/hive/pkg/continuity"
)

var progressRefPattern = regexpMustCompile(`(?i)\b(progresses|progressing)\b[^.\n#]{0,40}?(?:([\w.-]+/[\w.-]+))?#(\d+)`)

// regexpMustCompile is a tiny seam that keeps the continuity-specific explicit
// relationship grammar beside the existing claim grammar without widening the
// semantics of ParseReferencedIssues for every caller.
func regexpMustCompile(pattern string) *regexp.Regexp { return regexp.MustCompile(pattern) }

// ObserveContinuityPR obtains a bounded current-state snapshot for an explicit
// PR reference. It discovers; it never adopts, labels, updates, or pushes.
func (c *Client) ObserveContinuityPR(ctx context.Context, ref continuity.PRRef) (continuity.Observation, error) {
	if c == nil || c.client == nil {
		return continuity.Observation{}, ErrNoGitHubClient
	}
	if err := ref.Validate(); err != nil {
		return continuity.Observation{}, err
	}
	owner, repo := splitFullRepo(ref.Repo)
	pr, _, err := c.client.PullRequests.Get(ctx, owner, repo, ref.Number)
	if err != nil {
		return continuity.Observation{}, fmt.Errorf("observing %s#%d: %w", ref.Repo, ref.Number, err)
	}
	obs := continuity.Observation{
		Ref: ref, OriginalAuthor: safeGetLogin(pr.GetUser()), Draft: pr.GetDraft(),
		HeadRepo: pr.GetHead().GetRepo().GetFullName(), HeadBranch: pr.GetHead().GetRef(), HeadSHA: pr.GetHead().GetSHA(),
		BaseBranch: pr.GetBase().GetRef(), BaseSHA: pr.GetBase().GetSHA(),
		Mergeable: pr.GetMergeableState(), Provenance: fmt.Sprintf("github:%s/pull/%d@%s", ref.Repo, ref.Number, pr.GetHead().GetSHA()),
		ObservedAt: time.Now().UTC(),
	}
	for _, label := range pr.Labels {
		if label != nil && isHoldLabel(label.GetName()) {
			obs.Hold = true
		}
	}

	if repoState, _, repoErr := c.client.Repositories.Get(ctx, owner, repo); repoErr == nil {
		if obs.HeadRepo != ref.Repo {
			obs.WriteCapability = continuity.CapabilityUnwritable
		} else if !repoState.GetPermissions()["push"] {
			obs.WriteCapability = continuity.CapabilityUnwritable
		} else if branch, _, branchErr := c.client.Repositories.GetBranch(ctx, owner, repo, obs.HeadBranch, 3); branchErr != nil {
			obs.WriteCapability = continuity.CapabilityUnknown
		} else if branch.GetProtected() {
			// Repository-level push permission does not prove that this App can
			// bypass a branch ruleset. Do not discover by attempting a mutation.
			obs.WriteCapability = continuity.CapabilityUnknown
		} else {
			obs.WriteCapability = continuity.CapabilityWritable
		}
	} else {
		obs.WriteCapability = continuity.CapabilityUnknown
	}

	if obs.BaseSHA != "" && obs.HeadSHA != "" {
		if comparison, _, compareErr := c.client.Repositories.CompareCommits(ctx, owner, repo, obs.BaseSHA, obs.HeadSHA, nil); compareErr == nil {
			obs.MergeBaseSHA = comparison.GetMergeBaseCommit().GetSHA()
		}
	}

	files, err := c.listContinuityPRFiles(ctx, owner, repo, ref.Number)
	if err != nil {
		return continuity.Observation{}, err
	}
	obs.ChangedFiles = files
	obs.CIStatus = c.continuityCIStatus(ctx, owner, repo, obs.HeadSHA)
	obs.LinkedWork, obs.Acceptance = continuityRelationships(pr.GetTitle()+"\n"+pr.GetBody(), ref.Repo, ref.Number, pr.GetTitle(), pr.GetDraft())

	openPRs, err := c.listContinuityOpenPRs(ctx, owner, repo)
	if err != nil {
		return continuity.Observation{}, err
	}
	currentFiles := stringSet(files)
	for _, other := range openPRs {
		if other == nil || other.GetNumber() == ref.Number {
			continue
		}
		otherRef := continuity.PRRef{Repo: ref.Repo, Number: other.GetNumber()}
		if other.GetHead().GetRef() == obs.BaseBranch {
			obs.Stack = append(obs.Stack, continuity.StackRelation{PRRef: otherRef, Kind: "stacked_on", Evidence: "current base branch equals parent head branch"})
		}
		if other.GetBase().GetRef() == obs.HeadBranch {
			obs.Stack = append(obs.Stack, continuity.StackRelation{PRRef: otherRef, Kind: "depended_on_by", Evidence: "child base branch equals current head branch"})
		}
		otherFiles, fileErr := c.listContinuityPRFiles(ctx, owner, repo, other.GetNumber())
		if fileErr != nil {
			continue
		}
		if intersects(currentFiles, otherFiles) {
			obs.OverlappingPRs = append(obs.OverlappingPRs, otherRef)
		}
	}
	sort.Slice(obs.OverlappingPRs, func(i, j int) bool { return obs.OverlappingPRs[i].Number < obs.OverlappingPRs[j].Number })

	obs.State, obs.StateReason = classifyContinuityObservation(obs)
	return obs, nil
}

// ValidateContinuityHead is the lightweight exact-subject check performed
// immediately before a contributor receives a credential. A moved branch is
// refused; the next observation sweep can then reacquire under owner authority.
func (c *Client) ValidateContinuityHead(ctx context.Context, ref continuity.PRRef, expectedSHA, expectedBranch string) error {
	if c == nil || c.client == nil {
		return ErrNoGitHubClient
	}
	if err := ref.Validate(); err != nil {
		return err
	}
	owner, repo := splitFullRepo(ref.Repo)
	pr, _, err := c.client.PullRequests.Get(ctx, owner, repo, ref.Number)
	if err != nil {
		return fmt.Errorf("validating continuity head for %s#%d: %w", ref.Repo, ref.Number, err)
	}
	if pr.GetHead().GetRepo().GetFullName() != ref.Repo || pr.GetHead().GetRef() != expectedBranch || pr.GetHead().GetSHA() != expectedSHA {
		return fmt.Errorf("continuity head changed: expected %s:%s@%s, observed %s:%s@%s",
			ref.Repo, expectedBranch, expectedSHA, pr.GetHead().GetRepo().GetFullName(), pr.GetHead().GetRef(), pr.GetHead().GetSHA())
	}
	return nil
}

func splitFullRepo(repo string) (string, string) {
	owner, name, _ := strings.Cut(repo, "/")
	return owner, name
}

func (c *Client) listContinuityOpenPRs(ctx context.Context, owner, repo string) ([]*gh.PullRequest, error) {
	opts := &gh.PullRequestListOptions{State: "open", ListOptions: gh.ListOptions{PerPage: 100}}
	var out []*gh.PullRequest
	for page := 0; page < claimSearchMaxPages; page++ {
		items, resp, err := c.client.PullRequests.List(ctx, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("listing open PR topology for %s/%s: %w", owner, repo, err)
		}
		out = append(out, items...)
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

func (c *Client) listContinuityPRFiles(ctx context.Context, owner, repo string, number int) ([]string, error) {
	opts := &gh.ListOptions{PerPage: 100}
	var out []string
	for page := 0; page < claimSearchMaxPages; page++ {
		files, resp, err := c.client.PullRequests.ListFiles(ctx, owner, repo, number, opts)
		if err != nil {
			return nil, fmt.Errorf("listing files for %s/%s#%d: %w", owner, repo, number, err)
		}
		for _, file := range files {
			if name := file.GetFilename(); name != "" {
				out = append(out, name)
			}
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	sort.Strings(out)
	return out, nil
}

func (c *Client) continuityCIStatus(ctx context.Context, owner, repo, sha string) string {
	if sha == "" {
		return "unknown"
	}
	runs, _, err := c.client.Checks.ListCheckRunsForRef(ctx, owner, repo, sha, &gh.ListCheckRunsOptions{ListOptions: gh.ListOptions{PerPage: 100}})
	if err != nil {
		return "unknown"
	}
	if runs.GetTotal() == 0 {
		return "unknown"
	}
	pending := false
	for _, run := range runs.CheckRuns {
		if run.GetStatus() != "completed" {
			pending = true
			continue
		}
		switch run.GetConclusion() {
		case "success", "neutral", "skipped":
		default:
			return "failure"
		}
	}
	if pending {
		return "pending"
	}
	return "success"
}

func continuityRelationships(text, repo string, prNumber int, title string, draft bool) ([]continuity.WorkRelationship, []continuity.AcceptanceDelta) {
	closing := ParseClaimedIssues(text, repo)
	refs := ParseReferencedIssues(text, repo)
	progress := parseRefs(progressRefPattern, text, repo)
	refs = append(refs, progress...)
	partial := make(map[string]bool, len(progress))
	for _, ref := range progress {
		partial[claimKey(ref.Repo, ref.Issue)] = true
	}
	seen := map[string]bool{}
	var rels []continuity.WorkRelationship
	var acceptance []continuity.AcceptanceDelta
	for _, ref := range closing {
		key := claimKey(ref.Repo, ref.Issue)
		if seen[key] {
			continue
		}
		seen[key] = true
		owned := fmt.Sprintf("PR #%d: %s", prNumber, strings.TrimSpace(title))
		rels = append(rels, continuity.WorkRelationship{WorkRef: key, Relationship: continuity.RelationshipCloses, OwnedSlice: owned, Evidence: "explicit GitHub closing keyword"})
		delta := continuity.AcceptanceDelta{WorkRef: key, Owned: []string{owned}, ClosingKeywordRisk: draft || partial[key]}
		if partial[key] {
			delta.Ambiguous = []string{"PR declares both partial progress and automatic closure for the same work item"}
		}
		acceptance = append(acceptance, delta)
	}
	for _, ref := range refs {
		key := claimKey(ref.Repo, ref.Issue)
		if seen[key] {
			continue
		}
		seen[key] = true
		rels = append(rels, continuity.WorkRelationship{WorkRef: key, Relationship: continuity.RelationshipReferences, Evidence: "explicit non-closing PR relationship", Ambiguous: true})
		acceptance = append(acceptance, continuity.AcceptanceDelta{WorkRef: key, Ambiguous: []string{"PR references work but does not define the acceptance slice it owns"}})
	}
	return rels, acceptance
}

func classifyContinuityObservation(obs continuity.Observation) (continuity.State, string) {
	if obs.HeadRepo == "" || obs.HeadBranch == "" || obs.HeadSHA == "" || obs.BaseBranch == "" {
		return continuity.StateUnknown, "GitHub did not return a complete head/base identity"
	}
	if obs.HeadRepo != obs.Ref.Repo || obs.WriteCapability == continuity.CapabilityUnwritable {
		return continuity.StateBlocked, "head repository or branch is not writable by the Hive App; no substitute branch is allowed"
	}
	if obs.WriteCapability == continuity.CapabilityUnknown {
		return continuity.StateUnknown, "Hive could not prove write capability for the existing head branch"
	}
	if strings.EqualFold(obs.Mergeable, "dirty") {
		return continuity.StateBlocked, "existing PR has merge conflicts"
	}
	if len(obs.LinkedWork) == 0 {
		return continuity.StateUnknown, "PR has no explicit linked work or owned acceptance slice"
	}
	if obs.Draft || obs.CIStatus == "pending" || obs.CIStatus == "failure" || obs.CIStatus == "unknown" {
		return continuity.StateContinue, "existing implementation has remaining verification or draft work"
	}
	for _, delta := range obs.Acceptance {
		if delta.ClosingKeywordRisk || len(delta.Ambiguous) > 0 || len(delta.Missing) > 0 {
			return continuity.StateUnknown, "acceptance ownership is ambiguous or incomplete"
		}
	}
	return continuity.StateReady, "exact head is non-draft, writable, mergeable, and currently green"
}

func isHoldLabel(name string) bool {
	for _, hold := range HoldLabels {
		if strings.EqualFold(strings.TrimSpace(name), hold) {
			return true
		}
	}
	return false
}

func stringSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func intersects(a map[string]bool, b []string) bool {
	for _, value := range b {
		if a[value] {
			return true
		}
	}
	return false
}

// BuildContinuityResult projects only exact-head, writable CONTINUE records
// into implementation work. BLOCKED/UNKNOWN/READY records remain durable and
// continue suppressing replacement work, but are not handed to a contributor.
func BuildContinuityResult(records []continuity.Record) ContinuityResult {
	result := ContinuityResult{}
	for _, rec := range records {
		if rec.Continuable() {
			result.Items = append(result.Items, rec)
		}
	}
	sort.Slice(result.Items, func(i, j int) bool { return result.Items[i].Ref.Key() < result.Items[j].Ref.Key() })
	result.Count = len(result.Items)
	return result
}

// ContinuityIssue is the additive compatibility envelope consumed by the
// existing contributor queue. It names the PR as a source-aware external item
// so it cannot collide with a GitHub issue number.
func ContinuityIssue(rec continuity.Record) Issue {
	return Issue{
		Repo: rec.Ref.Repo, Number: 0,
		Title:      fmt.Sprintf("Continue existing PR #%d", rec.Ref.Number),
		Author:     rec.OriginalAuthor,
		URL:        fmt.Sprintf("https://github.com/%s/pull/%d", rec.Ref.Repo, rec.Ref.Number),
		SourceType: "github_pull_request", ExternalID: fmt.Sprintf("pr-%d", rec.Ref.Number),
		ContinuityPR: rec.Ref.Number, ExistingHeadRepo: rec.HeadRepo,
		ExistingHeadBranch: rec.HeadBranch, ExpectedHeadSHA: rec.ObservedHeadSHA,
		BaseBranch: rec.BaseBranch, OriginalPRAuthor: rec.OriginalAuthor,
	}
}

// FilterContinuityOwnedIssues removes issue slices owned by any active adopted
// PR, independently of whether the PR appeared in the current open-PR scan.
// BLOCKED and UNKNOWN adoptions intentionally suppress replacement work until
// an owner revokes or authoritatively supersedes them.
func FilterContinuityOwnedIssues(result *ActionableResult, records []continuity.Record, logger *slog.Logger) int {
	if result == nil || len(records) == 0 {
		return 0
	}
	owned := map[string][]continuity.PRRef{}
	for _, rec := range records {
		if !rec.Active || rec.State == continuity.StateSuperseded {
			continue
		}
		for _, rel := range rec.LinkedWork {
			if rel.Ambiguous || strings.TrimSpace(rel.OwnedSlice) == "" {
				continue
			}
			owned[rel.WorkRef] = append(owned[rel.WorkRef], rec.Ref)
		}
	}
	if len(owned) == 0 {
		return 0
	}
	kept := make([]Issue, 0, len(result.Issues.Items))
	suppressed := 0
	for _, issue := range result.Issues.Items {
		key := claimKey(issue.Repo, issue.Number)
		claims := owned[key]
		if len(claims) == 0 {
			kept = append(kept, issue)
			continue
		}
		suppressed++
		if logger != nil {
			logger.Info("suppressing replacement implementation for adopted PR-owned slice",
				"work", key, "adopted_prs", claims)
		}
	}
	result.Issues.Items = kept
	result.Issues.Count = len(kept)
	result.Issues.SLAViolations = 0
	for _, issue := range kept {
		if issue.AgeMinutes > slaThresholdMinutes {
			result.Issues.SLAViolations++
		}
	}
	return suppressed
}

package github

import (
	"testing"

	"github.com/kubestellar/hive/pkg/continuity"
)

func adoptedRecord(number int, state continuity.State, writable continuity.WriteCapability) continuity.Record {
	return continuity.Record{
		Ref: continuity.PRRef{Repo: "acme/widgets", Number: number}, Active: true,
		OriginalAuthor: "alice", HeadRepo: "acme/widgets", HeadBranch: "alice/existing",
		BaseBranch: "main", ObservedHeadSHA: "head-1", CurrentHeadSHA: "head-1",
		State: state, WriteCapability: writable,
	}
}

func TestBuildContinuityResultOnlyProjectsSafeContinueRecords(t *testing.T) {
	continueRec := adoptedRecord(17, continuity.StateContinue, continuity.CapabilityWritable)
	blocked := adoptedRecord(18, continuity.StateBlocked, continuity.CapabilityUnwritable)
	moved := adoptedRecord(19, continuity.StateContinue, continuity.CapabilityWritable)
	moved.CurrentHeadSHA = "head-2"
	revoked := adoptedRecord(20, continuity.StateContinue, continuity.CapabilityWritable)
	revoked.Active = false

	result := BuildContinuityResult([]continuity.Record{blocked, continueRec, moved, revoked})
	if result.Count != 1 || len(result.Items) != 1 || result.Items[0].Ref.Number != 17 {
		t.Fatalf("continuity result = %+v", result)
	}
}

func TestContinuityIssueRetainsExistingBranchAndHistoryBinding(t *testing.T) {
	rec := adoptedRecord(17, continuity.StateContinue, continuity.CapabilityWritable)
	issue := ContinuityIssue(rec)
	if issue.SourceType != "github_pull_request" || issue.ExternalID != "pr-17" || issue.Number != 0 {
		t.Fatalf("identity = %+v", issue)
	}
	if issue.ContinuityPR != 17 || issue.ExistingHeadRepo != "acme/widgets" || issue.ExistingHeadBranch != "alice/existing" || issue.ExpectedHeadSHA != "head-1" || issue.BaseBranch != "main" || issue.OriginalPRAuthor != "alice" {
		t.Fatalf("branch/history binding lost: %+v", issue)
	}
}

func TestFilterContinuityOwnedIssuesSuppressesWithoutLivePRClaim(t *testing.T) {
	rec := adoptedRecord(17, continuity.StateUnknown, continuity.CapabilityUnknown)
	rec.LinkedWork = []continuity.WorkRelationship{{WorkRef: "acme/widgets#9", Relationship: continuity.RelationshipCloses, OwnedSlice: "runtime"}}
	result := &ActionableResult{Issues: IssueResult{Count: 2, Items: []Issue{
		{Repo: "acme/widgets", Number: 9}, {Repo: "acme/widgets", Number: 10},
	}}}
	if got := FilterContinuityOwnedIssues(result, []continuity.Record{rec}, nil); got != 1 {
		t.Fatalf("suppressed=%d result=%+v", got, result.Issues)
	}
	if result.Issues.Count != 1 || result.Issues.Items[0].Number != 10 {
		t.Fatalf("active unknown adoption did not locally suppress only its owned slice: %+v", result.Issues)
	}
	rec.Active = false
	result = &ActionableResult{Issues: IssueResult{Count: 1, Items: []Issue{{Repo: "acme/widgets", Number: 9}}}}
	if got := FilterContinuityOwnedIssues(result, []continuity.Record{rec}, nil); got != 0 || result.Issues.Count != 1 {
		t.Fatalf("revoked adoption still suppressed: %+v", result.Issues)
	}
}

func TestFilterContinuityOwnedIssuesHonorsPartialSuppressionWithoutClosingOwnership(t *testing.T) {
	rec := adoptedRecord(17, continuity.StateContinue, continuity.CapabilityWritable)
	rec.LinkedWork = []continuity.WorkRelationship{{WorkRef: "acme/widgets#9", Relationship: continuity.RelationshipReferences, Evidence: "Progresses #9", Ambiguous: true}}
	rec.SuppressionClaims = []continuity.SuppressionClaim{{WorkRef: "acme/widgets#9", Principal: "owner", Provenance: "verified-owner-dashboard", Active: true}}
	result := &ActionableResult{Issues: IssueResult{Count: 2, Items: []Issue{
		{Repo: "acme/widgets", Number: 9}, {Repo: "acme/widgets", Number: 10},
	}}}

	if got := FilterContinuityOwnedIssues(result, []continuity.Record{rec}, nil); got != 1 {
		t.Fatalf("suppressed=%d result=%+v", got, result.Issues)
	}
	if result.Issues.Count != 1 || result.Issues.Items[0].Number != 10 {
		t.Fatalf("partial suppression did not remove only the claimed issue: %+v", result.Issues)
	}
	if rec.LinkedWork[0].Relationship != continuity.RelationshipReferences || !rec.LinkedWork[0].Ambiguous || rec.LinkedWork[0].OwnedSlice != "" {
		t.Fatalf("partial suppression changed closing ownership: %+v", rec.LinkedWork[0])
	}
}

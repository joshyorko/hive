package github

import (
	"testing"
	"time"
)

func TestFilterClaimedIssuesActiveAdoptedHumanPRSuppressesReplacement(t *testing.T) {
	ledger := NewClaimLedger(t.TempDir()+"/claims.json", nil)
	ledger.Reconcile([]IssueClaim{{
		Repo: "acme/widgets", Issue: 9, PRRepo: "acme/widgets", PRNumber: 17,
		PRAuthor: "alice", ExternalAuthor: true, Reference: true, Adopted: true, ObservedAt: time.Now(),
	}}, true)
	result := &ActionableResult{Issues: IssueResult{Count: 1, Items: []Issue{{Repo: "acme/widgets", Number: 9, Title: "owned slice"}}}}
	if got := FilterClaimedIssues(result, ledger, func(string, int) bool { return true }, nil); got != 1 || result.Issues.Count != 0 {
		t.Fatalf("active adoption did not suppress replacement: suppressed=%d result=%+v", got, result.Issues)
	}
}

func TestFilterClaimedIssuesUnadoptedHumanPRStillDoesNotFreezeAgentPipeline(t *testing.T) {
	ledger := NewClaimLedger(t.TempDir()+"/claims.json", nil)
	ledger.Reconcile([]IssueClaim{{
		Repo: "acme/widgets", Issue: 9, PRRepo: "acme/widgets", PRNumber: 17,
		PRAuthor: "alice", ExternalAuthor: true, ObservedAt: time.Now(),
	}}, true)
	result := &ActionableResult{Issues: IssueResult{Count: 1, Items: []Issue{{Repo: "acme/widgets", Number: 9}}}}
	if got := FilterClaimedIssues(result, ledger, nil, nil); got != 0 || result.Issues.Count != 1 {
		t.Fatalf("unadopted human PR changed legacy agent behavior: suppressed=%d result=%+v", got, result.Issues)
	}
}

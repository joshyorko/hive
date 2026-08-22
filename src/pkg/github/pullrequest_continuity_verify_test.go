package github

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kubestellar/hive/pkg/continuity"
)

func continuityDeliveryServer(t *testing.T, author, branch, head, compareStatus, commitAuthor string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/widgets/pulls/17", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"number":17,"html_url":"https://github.com/acme/widgets/pull/17","user":{"login":"`+author+`"},"head":{"ref":"`+branch+`","sha":"`+head+`","repo":{"full_name":"acme/widgets"}},"base":{"ref":"main","repo":{"full_name":"acme/widgets"}}}`)
	})
	mux.HandleFunc("/repos/acme/widgets/compare/old-head..."+head, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"`+compareStatus+`","ahead_by":1,"commits":[{"sha":"`+head+`","author":{"login":"`+commitAuthor+`"},"commit":{"author":{"name":"Hive Contributor"}}}]}`)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func adoptedDeliveryRecord() continuity.Record {
	return continuity.Record{
		Ref: continuity.PRRef{Repo: "acme/widgets", Number: 17}, OriginalAuthor: "alice",
		HeadRepo: "acme/widgets", HeadBranch: "alice/existing", BaseBranch: "main",
		ObservedHeadSHA: "old-head", CurrentHeadSHA: "old-head", Active: true,
	}
}

func TestVerifyContinuityDeliveryRequiresFastForwardCommitByContributor(t *testing.T) {
	ts := continuityDeliveryServer(t, "alice", "alice/existing", "new-head", "ahead", "hive-bot")
	c := NewClientForTest(ts.URL, "acme", []string{"widgets"}, verifyTestLogger())

	res := c.VerifyContinuityDelivery(context.Background(), adoptedDeliveryRecord(), "https://github.com/acme/widgets/pull/17", "hive-bot")
	if !res.Verified || res.Author != "alice" || res.BaseRepo != "acme/widgets" {
		t.Fatalf("continuity delivery not verified: %+v", res)
	}
}

func TestVerifyContinuityDeliveryRejectsUnchangedOrRewrittenHistory(t *testing.T) {
	for _, status := range []string{"identical", "diverged", "behind"} {
		t.Run(status, func(t *testing.T) {
			ts := continuityDeliveryServer(t, "alice", "alice/existing", "new-head", status, "hive-bot")
			c := NewClientForTest(ts.URL, "acme", []string{"widgets"}, verifyTestLogger())
			if res := c.VerifyContinuityDelivery(context.Background(), adoptedDeliveryRecord(), "https://github.com/acme/widgets/pull/17", "hive-bot"); res.Verified {
				t.Fatalf("history status %q verified: %+v", status, res)
			}
		})
	}
}

func TestVerifyContinuityDeliveryPreservesOriginalAuthorBranchAndNewAttribution(t *testing.T) {
	for _, tc := range []struct{ author, branch, commitAuthor string }{
		{author: "mallory", branch: "alice/existing", commitAuthor: "hive-bot"},
		{author: "alice", branch: "replacement", commitAuthor: "hive-bot"},
		{author: "alice", branch: "alice/existing", commitAuthor: "mallory"},
	} {
		ts := continuityDeliveryServer(t, tc.author, tc.branch, "new-head", "ahead", tc.commitAuthor)
		c := NewClientForTest(ts.URL, "acme", []string{"widgets"}, verifyTestLogger())
		if res := c.VerifyContinuityDelivery(context.Background(), adoptedDeliveryRecord(), "https://github.com/acme/widgets/pull/17", "hive-bot"); res.Verified {
			t.Fatalf("identity/history rewrite verified for %+v", tc)
		}
	}
}

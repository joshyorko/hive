package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/kubestellar/hive/pkg/config"
	"github.com/kubestellar/hive/pkg/continuity"
)

func continuityAPIServer(t *testing.T) (*Server, *continuity.Ledger) {
	t.Helper()
	ledger, err := continuity.OpenLedger(filepath.Join(t.TempDir(), "continuity.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Project.Org = "acme"
	cfg.Project.Repos = []string{"widgets"}
	s := &Server{deps: &Dependencies{Config: cfg, ContinuityLedger: ledger,
		ObserveContinuityPR: func(_ context.Context, ref continuity.PRRef) (continuity.Observation, error) {
			return continuity.Observation{
				Ref: ref, OriginalAuthor: "alice", HeadRepo: ref.Repo,
				HeadBranch: "alice/existing", BaseBranch: "main", HeadSHA: "head-1", BaseSHA: "base-1",
				MergeBaseSHA: "merge-base-1", State: continuity.StateContinue,
				WriteCapability: continuity.CapabilityWritable,
				LinkedWork:      []continuity.WorkRelationship{{WorkRef: "acme/widgets#9", Relationship: continuity.RelationshipCloses, OwnedSlice: "runtime"}},
				Provenance:      "github:acme/widgets/pull/17@head-1",
			}, nil
		}}}
	return s, ledger
}

func continuityRequest(t *testing.T, s *Server, body map[string]any, owner bool) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/continuity/pr-adoptions", bytes.NewReader(raw))
	if owner {
		req.Header.Set("X-Hive-User", "owner-user")
		req.Header.Set("X-Hive-Role", config.RoleOwner)
		req.Header.Set(ownerRoleVerifiedHeader, "true")
	}
	w := httptest.NewRecorder()
	s.handleContinuityPRAdoptions(w, req)
	return w
}

func TestContinuityAdoptionRequiresVerifiedOwnerAndIgnoresLabelAuthority(t *testing.T) {
	s, ledger := continuityAPIServer(t)
	w := continuityRequest(t, s, map[string]any{"action": "adopt", "repo": "widgets", "pr_number": 17, "labels": []string{"hive-adopt"}}, false)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if len(ledger.List()) != 0 {
		t.Fatal("agent-applicable label authorized adoption")
	}
}

func TestContinuityOwnerAdoptionIsIdempotentAndAuditable(t *testing.T) {
	s, ledger := continuityAPIServer(t)
	body := map[string]any{"action": "adopt", "repo": "widgets", "pr_number": 17}
	for i := 0; i < 2; i++ {
		w := continuityRequest(t, s, body, true)
		if w.Code != http.StatusOK {
			t.Fatalf("adopt %d status=%d body=%s", i, w.Code, w.Body.String())
		}
	}
	records := ledger.List()
	if len(records) != 1 || records[0].Generation != 1 || records[0].AdoptionPrincipal != "owner-user" || len(records[0].History) != 1 {
		t.Fatalf("records = %+v", records)
	}
}

func TestContinuityDryRunObservesWithoutAuthorityMutation(t *testing.T) {
	s, ledger := continuityAPIServer(t)
	w := continuityRequest(t, s, map[string]any{"action": "dry_run", "repo": "widgets", "pr_number": 17}, true)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if len(ledger.List()) != 0 {
		t.Fatal("dry-run created adoption authority")
	}
}

func TestContinuityRevocationStopsContinuation(t *testing.T) {
	s, ledger := continuityAPIServer(t)
	if w := continuityRequest(t, s, map[string]any{"action": "adopt", "repo": "widgets", "pr_number": 17}, true); w.Code != http.StatusOK {
		t.Fatalf("adopt status=%d", w.Code)
	}
	rec := ledger.List()[0]
	w := continuityRequest(t, s, map[string]any{"action": "revoke", "repo": "widgets", "pr_number": 17, "expected_generation": rec.Generation, "reason": "operator returned ownership"}, true)
	if w.Code != http.StatusOK {
		t.Fatalf("revoke status=%d body=%s", w.Code, w.Body.String())
	}
	got := ledger.List()[0]
	if got.Active || len(ledger.LookupWork("acme/widgets#9")) != 0 {
		t.Fatalf("revoked adoption still owns work: %+v", got)
	}
}

func TestContinuitySuppressionPromotionIsOwnerGatedAndIdempotent(t *testing.T) {
	s, ledger := continuityAPIServer(t)
	s.deps.ObserveContinuityPR = func(_ context.Context, ref continuity.PRRef) (continuity.Observation, error) {
		return continuity.Observation{Ref: ref, OriginalAuthor: "alice", HeadRepo: ref.Repo,
			HeadBranch: "alice/existing", BaseBranch: "main", HeadSHA: "head-1", BaseSHA: "base-1",
			State: continuity.StateContinue, WriteCapability: continuity.CapabilityWritable,
			LinkedWork: []continuity.WorkRelationship{{WorkRef: "acme/widgets#9", Relationship: continuity.RelationshipReferences, Evidence: "Progresses #9", Ambiguous: true}},
			Provenance: "github:acme/widgets/pull/17@head-1"}, nil
	}
	if w := continuityRequest(t, s, map[string]any{"action": "adopt", "repo": "widgets", "pr_number": 17}, true); w.Code != http.StatusOK {
		t.Fatalf("adopt status=%d body=%s", w.Code, w.Body.String())
	}
	rec := ledger.List()[0]
	body := map[string]any{"action": "promote_suppression", "repo": "widgets", "pr_number": 17, "work_ref": "acme/widgets#9", "expected_generation": rec.Generation}
	if w := continuityRequest(t, s, body, false); w.Code != http.StatusForbidden {
		t.Fatalf("non-owner promotion status=%d body=%s", w.Code, w.Body.String())
	}
	w := continuityRequest(t, s, body, true)
	if w.Code != http.StatusOK {
		t.Fatalf("owner promotion status=%d body=%s", w.Code, w.Body.String())
	}
	got := ledger.List()[0]
	if len(got.SuppressionClaims) != 1 || got.SuppressionClaims[0].WorkRef != "acme/widgets#9" {
		t.Fatalf("claims=%+v", got.SuppressionClaims)
	}
}

package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kubestellar/hive/pkg/continuity"
)

func continuityClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	c := NewClient("token", "acme", []string{"widgets"}, nil, "")
	c.client.BaseURL, _ = c.client.BaseURL.Parse(ts.URL + "/")
	c.client.UploadURL = c.client.BaseURL
	return c
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatal(err)
	}
}

func continuityFixture(t *testing.T, fork bool) *Client {
	return continuityFixtureWithProtection(t, fork, false)
}

func continuityFixtureWithProtection(t *testing.T, fork, protected bool) *Client {
	t.Helper()
	headRepo := "acme/widgets"
	if fork {
		headRepo = "alice/widgets"
	}
	return continuityClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/acme/widgets/pulls/17":
			writeJSON(t, w, map[string]any{
				"number": 17, "title": "Finish validation", "body": "Progresses #9\n\nCloses #10",
				"draft": true, "mergeable": true, "mergeable_state": "clean",
				"user":     map[string]any{"login": "alice"},
				"head":     map[string]any{"ref": "alice/existing", "sha": "head-17", "repo": map[string]any{"full_name": headRepo}},
				"base":     map[string]any{"ref": "feature-parent", "sha": "base-17", "repo": map[string]any{"full_name": "acme/widgets"}},
				"labels":   []any{map[string]any{"name": "hold"}},
				"html_url": "https://github.com/acme/widgets/pull/17",
			})
		case r.URL.Path == "/repos/acme/widgets":
			writeJSON(t, w, map[string]any{"full_name": "acme/widgets", "permissions": map[string]any{"push": true}})
		case r.URL.Path == "/repos/acme/widgets/branches/alice/existing":
			writeJSON(t, w, map[string]any{"name": "alice/existing", "protected": protected})
		case r.URL.Path == "/repos/acme/widgets/compare/base-17...head-17":
			writeJSON(t, w, map[string]any{"merge_base_commit": map[string]any{"sha": "merge-base-17"}})
		case r.URL.Path == "/repos/acme/widgets/pulls/17/files":
			writeJSON(t, w, []any{map[string]any{"filename": "pkg/shared.go"}, map[string]any{"filename": "pkg/only17.go"}})
		case r.URL.Path == "/repos/acme/widgets/pulls":
			writeJSON(t, w, []any{
				map[string]any{"number": 16, "head": map[string]any{"ref": "feature-parent", "sha": "head-16"}, "base": map[string]any{"ref": "main"}},
				map[string]any{"number": 18, "head": map[string]any{"ref": "feature-child", "sha": "head-18"}, "base": map[string]any{"ref": "alice/existing"}},
			})
		case r.URL.Path == "/repos/acme/widgets/pulls/16/files":
			writeJSON(t, w, []any{map[string]any{"filename": "pkg/shared.go"}})
		case r.URL.Path == "/repos/acme/widgets/pulls/18/files":
			writeJSON(t, w, []any{map[string]any{"filename": "pkg/child.go"}})
		case r.URL.Path == "/repos/acme/widgets/commits/head-17/check-runs":
			writeJSON(t, w, map[string]any{"total_count": 1, "check_runs": []any{map[string]any{"name": "test", "status": "completed", "conclusion": "success"}}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
}

func TestObserveContinuityPRProtectedBranchDoesNotClaimWriteCapability(t *testing.T) {
	obs, err := continuityFixtureWithProtection(t, false, true).ObserveContinuityPR(context.Background(), continuity.PRRef{Repo: "acme/widgets", Number: 17})
	if err != nil {
		t.Fatal(err)
	}
	if obs.WriteCapability != continuity.CapabilityUnknown || obs.State != continuity.StateUnknown {
		t.Fatalf("protected branch write capability was guessed: %+v", obs)
	}
}

func TestObserveContinuityPRPreservesDraftBranchHistoryAndTopology(t *testing.T) {
	obs, err := continuityFixture(t, false).ObserveContinuityPR(context.Background(), continuity.PRRef{Repo: "acme/widgets", Number: 17})
	if err != nil {
		t.Fatal(err)
	}
	if obs.OriginalAuthor != "alice" || obs.HeadBranch != "alice/existing" || obs.HeadSHA != "head-17" || obs.BaseBranch != "feature-parent" || obs.MergeBaseSHA != "merge-base-17" {
		t.Fatalf("identity/history not preserved: %+v", obs)
	}
	if !obs.Draft || !obs.Hold || obs.State != continuity.StateContinue || obs.WriteCapability != continuity.CapabilityWritable {
		t.Fatalf("draft/hold/state = %+v", obs)
	}
	if len(obs.Stack) != 2 || obs.Stack[0].Kind != "stacked_on" || obs.Stack[0].PRRef.Number != 16 || obs.Stack[1].Kind != "depended_on_by" || obs.Stack[1].PRRef.Number != 18 {
		t.Fatalf("stack topology = %+v", obs.Stack)
	}
	if len(obs.OverlappingPRs) != 1 || obs.OverlappingPRs[0].Number != 16 {
		t.Fatalf("overlap = %+v", obs.OverlappingPRs)
	}
}

func TestObserveContinuityPRAuditsClosingVersusNonClosingSemantics(t *testing.T) {
	obs, err := continuityFixture(t, false).ObserveContinuityPR(context.Background(), continuity.PRRef{Repo: "acme/widgets", Number: 17})
	if err != nil {
		t.Fatal(err)
	}
	if len(obs.LinkedWork) != 2 {
		t.Fatalf("linked work = %+v", obs.LinkedWork)
	}
	if obs.LinkedWork[0].WorkRef != "acme/widgets#10" || obs.LinkedWork[0].Relationship != continuity.RelationshipCloses || obs.LinkedWork[0].OwnedSlice == "" {
		t.Fatalf("closing relationship = %+v", obs.LinkedWork[0])
	}
	if obs.LinkedWork[1].WorkRef != "acme/widgets#9" || obs.LinkedWork[1].Relationship != continuity.RelationshipReferences || !obs.LinkedWork[1].Ambiguous {
		t.Fatalf("non-closing relationship = %+v", obs.LinkedWork[1])
	}
	if len(obs.Acceptance) != 2 || !obs.Acceptance[0].ClosingKeywordRisk {
		t.Fatalf("partial draft closing risk not surfaced: %+v", obs.Acceptance)
	}
}

func TestContinuityRelationshipsSurfaceContradictoryPartialCloses(t *testing.T) {
	rels, acceptance := continuityRelationships("Progresses #10\n\nCloses #10", "acme/widgets", 17, "partial runtime slice", false)
	if len(rels) != 1 || rels[0].Relationship != continuity.RelationshipCloses {
		t.Fatalf("relationships = %+v", rels)
	}
	if len(acceptance) != 1 || !acceptance[0].ClosingKeywordRisk || len(acceptance[0].Ambiguous) == 0 {
		t.Fatalf("contradictory partial closing semantics were not surfaced: %+v", acceptance)
	}
}

func TestHoldRemainsReleaseBoundaryNotContinuationOwnership(t *testing.T) {
	obs := continuity.Observation{
		Ref: continuity.PRRef{Repo: "acme/widgets", Number: 17}, OriginalAuthor: "alice",
		HeadRepo: "acme/widgets", HeadBranch: "alice/existing", BaseBranch: "main", HeadSHA: "head-1",
		WriteCapability: continuity.CapabilityWritable, Mergeable: "clean", CIStatus: "success", Hold: true,
		LinkedWork: []continuity.WorkRelationship{{WorkRef: "acme/widgets#9", Relationship: continuity.RelationshipCloses, OwnedSlice: "runtime"}},
	}
	state, _ := classifyContinuityObservation(obs)
	if state != continuity.StateReady {
		t.Fatalf("hold changed implementation judgment; release policy must remain separate: %s", state)
	}
}

func TestObserveContinuityPRForkFailsClosedWithoutReplacement(t *testing.T) {
	obs, err := continuityFixture(t, true).ObserveContinuityPR(context.Background(), continuity.PRRef{Repo: "acme/widgets", Number: 17})
	if err != nil {
		t.Fatal(err)
	}
	if obs.WriteCapability != continuity.CapabilityUnwritable || obs.State != continuity.StateBlocked || !strings.Contains(obs.StateReason, "head repository") {
		t.Fatalf("fork must be blocked: %+v", obs)
	}
}

func TestOrdinaryHumanDraftRemainsOutsideNormalActionablePRs(t *testing.T) {
	client := continuityClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/acme/widgets/issues" {
			writeJSON(t, w, []any{})
			return
		}
		if r.URL.Path == "/repos/acme/widgets/pulls" {
			writeJSON(t, w, []any{map[string]any{
				"number": 17, "title": "human draft", "draft": true,
				"user":       map[string]any{"login": "alice"},
				"head":       map[string]any{"ref": "alice/existing", "sha": "head-17"},
				"base":       map[string]any{"ref": "main"},
				"created_at": "2026-08-20T00:00:00Z",
			}})
			return
		}
		t.Fatalf("unexpected request %s", r.URL.Path)
	}))
	result, err := client.EnumerateActionable(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.PRs.Items) != 0 || len(result.PRs.StaleDrafts) != 0 {
		t.Fatalf("ordinary human draft leaked into normal actionable paths: %+v", result.PRs)
	}
}

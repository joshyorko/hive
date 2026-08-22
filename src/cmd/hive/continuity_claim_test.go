package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/kubestellar/hive/pkg/config"
	"github.com/kubestellar/hive/pkg/continuity"
)

func TestContinuityClaimAdoptedMatchesExactPRAndOwnedSlice(t *testing.T) {
	ledger, _ := continuity.OpenLedger(filepath.Join(t.TempDir(), "continuity.json"))
	obs := continuity.Observation{
		Ref: continuity.PRRef{Repo: "acme/widgets", Number: 17}, OriginalAuthor: "alice",
		HeadRepo: "acme/widgets", HeadBranch: "alice/existing", BaseBranch: "main", HeadSHA: "head-1",
		State: continuity.StateContinue, WriteCapability: continuity.CapabilityWritable,
		LinkedWork: []continuity.WorkRelationship{{WorkRef: "acme/widgets#9", Relationship: continuity.RelationshipCloses, OwnedSlice: "runtime"}},
		Provenance: "github:pr-17",
	}
	_, _ = ledger.Adopt(obs, "owner", "session", time.Now())
	cfg := &config.Config{}
	cfg.Project.Org = "acme"
	if !continuityClaimAdopted(cfg, ledger, "widgets", 17, "acme/widgets#9") {
		t.Fatal("exact adopted claim not recognized")
	}
	for _, wrong := range []struct {
		repo string
		pr   int
		work string
	}{{"widgets", 18, "acme/widgets#9"}, {"other", 17, "acme/widgets#9"}, {"widgets", 17, "acme/widgets#10"}} {
		if continuityClaimAdopted(cfg, ledger, wrong.repo, wrong.pr, wrong.work) {
			t.Fatalf("unrelated claim matched: %+v", wrong)
		}
	}
}

func TestRefreshContinuityAdoptionsReacquiresEachActiveRecordAndFailsClosedLocally(t *testing.T) {
	ledger, _ := continuity.OpenLedger(filepath.Join(t.TempDir(), "continuity.json"))
	now := time.Now().UTC()
	first := continuity.Observation{
		Ref: continuity.PRRef{Repo: "acme/widgets", Number: 17}, OriginalAuthor: "alice",
		HeadRepo: "acme/widgets", HeadBranch: "alice/existing", BaseBranch: "main", HeadSHA: "head-1",
		State: continuity.StateContinue, WriteCapability: continuity.CapabilityWritable,
		LinkedWork: []continuity.WorkRelationship{{WorkRef: "acme/widgets#9", Relationship: continuity.RelationshipCloses, OwnedSlice: "runtime"}},
		Provenance: "github:pr-17",
	}
	second := first
	second.Ref.Number = 18
	second.HeadBranch = "alice/other"
	second.HeadSHA = "head-2"
	second.LinkedWork = []continuity.WorkRelationship{{WorkRef: "acme/widgets#10", Relationship: continuity.RelationshipCloses, OwnedSlice: "docs"}}
	_, _ = ledger.Adopt(first, "owner", "session", now)
	_, _ = ledger.Adopt(second, "owner", "session", now)

	refreshContinuityAdoptions(context.Background(), ledger, func(_ context.Context, ref continuity.PRRef) (continuity.Observation, error) {
		if ref.Number == 18 {
			return continuity.Observation{}, errors.New("temporary GitHub failure")
		}
		observed := first
		observed.Hold = true
		observed.ObservedAt = now.Add(time.Minute)
		return observed, nil
	}, nil)

	refreshed, _ := ledger.Get(first.Ref)
	if !refreshed.Hold || refreshed.State != continuity.StateContinue {
		t.Fatalf("successful source refresh was not applied: %+v", refreshed)
	}
	degraded, _ := ledger.Get(second.Ref)
	if degraded.State != continuity.StateUnknown || !degraded.Active || degraded.ObservedHeadSHA != "head-2" {
		t.Fatalf("source-local failure did not retain ownership and fail closed: %+v", degraded)
	}
}

package continuity

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func fixtureObservation(head string) Observation {
	return Observation{
		Ref:             PRRef{Repo: "acme/widgets", Number: 17},
		OriginalAuthor:  "alice",
		HeadRepo:        "acme/widgets",
		HeadBranch:      "alice/existing-work",
		BaseBranch:      "main",
		HeadSHA:         head,
		BaseSHA:         "base-1",
		MergeBaseSHA:    "merge-base-1",
		State:           StateContinue,
		WriteCapability: CapabilityWritable,
		LinkedWork: []WorkRelationship{{
			WorkRef:      "acme/widgets#9",
			Relationship: RelationshipCloses,
			OwnedSlice:   "the validation slice",
		}},
		Provenance: "github:pull_request/17",
	}
}

func TestLedgerAdoptionIsDurableIdempotentAndPreservesIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "continuity.json")
	ledger, err := OpenLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 22, 14, 0, 0, 0, time.UTC)
	obs := fixtureObservation("head-1")
	rec, err := ledger.Adopt(obs, "owner-user", "dashboard-owner-session", now)
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if rec.Generation != 1 || !rec.Active || rec.AdoptionPrincipal != "owner-user" {
		t.Fatalf("record = %+v", rec)
	}
	if rec.OriginalAuthor != "alice" || rec.HeadBranch != "alice/existing-work" || rec.ObservedHeadSHA != "head-1" {
		t.Fatalf("adoption rewrote historical identity: %+v", rec)
	}

	again, err := ledger.Adopt(obs, "owner-user", "dashboard-owner-session", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("idempotent Adopt: %v", err)
	}
	if again.Generation != 1 || len(again.History) != 1 {
		t.Fatalf("repeated adoption mutated generation/history: %+v", again)
	}

	reopened, err := OpenLedger(path)
	if err != nil {
		t.Fatalf("restart OpenLedger: %v", err)
	}
	got, ok := reopened.Get(obs.Ref)
	if !ok || got.ObservedHeadSHA != "head-1" || got.HeadBranch != obs.HeadBranch {
		t.Fatalf("restart did not reconstruct adoption: %+v ok=%v", got, ok)
	}
}

func TestLedgerRejectsDiscoveryWithoutOwnerAuthority(t *testing.T) {
	ledger, _ := OpenLedger(filepath.Join(t.TempDir(), "continuity.json"))
	_, err := ledger.Adopt(fixtureObservation("head-1"), "", "github-label:hive-adopt", time.Now())
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("label-only discovery must not authorize adoption: %v", err)
	}
	if len(ledger.List()) != 0 {
		t.Fatal("unauthorized discovery created durable authority")
	}
}

func TestUnexpectedHeadMovementFailsClosedUntilOwnerReacquires(t *testing.T) {
	ledger, _ := OpenLedger(filepath.Join(t.TempDir(), "continuity.json"))
	now := time.Now().UTC()
	rec, _ := ledger.Adopt(fixtureObservation("head-1"), "owner", "session", now)
	moved := fixtureObservation("head-2")

	unknown, err := ledger.Refresh(moved, now.Add(time.Minute))
	if !errors.Is(err, ErrHeadMoved) {
		t.Fatalf("Refresh moved head error = %v", err)
	}
	if unknown.State != StateUnknown || unknown.ObservedHeadSHA != "head-1" || unknown.CurrentHeadSHA != "head-2" || !unknown.Active {
		t.Fatalf("head movement was not fenced: %+v", unknown)
	}

	if _, err := ledger.Reacquire(moved, rec.Generation, "owner", "dashboard-owner-session", now.Add(2*time.Minute)); !errors.Is(err, ErrGenerationConflict) {
		t.Fatalf("stale generation must not reacquire moved head: %v", err)
	}
	current, _ := ledger.Get(moved.Ref)
	reacquired, err := ledger.Reacquire(moved, current.Generation, "owner", "dashboard-owner-session", now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("owner Reacquire: %v", err)
	}
	if reacquired.ObservedHeadSHA != "head-2" || reacquired.State != StateContinue || reacquired.Generation <= current.Generation {
		t.Fatalf("reacquired record = %+v", reacquired)
	}
}

func TestActiveAdoptionClaimsOnlyItsOwnedSlicesAndRevocationReleasesThem(t *testing.T) {
	ledger, _ := OpenLedger(filepath.Join(t.TempDir(), "continuity.json"))
	now := time.Now().UTC()
	first := fixtureObservation("head-1")
	first.LinkedWork = []WorkRelationship{{WorkRef: "acme/widgets#9", Relationship: RelationshipReferences, OwnedSlice: "docs"}}
	second := fixtureObservation("head-2")
	second.Ref.Number = 18
	second.HeadBranch = "alice/other-slice"
	second.LinkedWork = []WorkRelationship{{WorkRef: "acme/widgets#9", Relationship: RelationshipCloses, OwnedSlice: "runtime"}}
	firstRec, _ := ledger.Adopt(first, "owner", "session", now)
	_, _ = ledger.Adopt(second, "owner", "session", now)

	claims := ledger.LookupWork("acme/widgets#9")
	if len(claims) != 2 || claims[0].Ref.Number != 17 || claims[1].Ref.Number != 18 {
		t.Fatalf("many-to-many owned slices collapsed: %+v", claims)
	}
	if _, err := ledger.Revoke(first.Ref, firstRec.Generation, "owner", "operator changed ownership", now.Add(time.Minute)); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	claims = ledger.LookupWork("acme/widgets#9")
	if len(claims) != 1 || claims[0].Ref.Number != 18 {
		t.Fatalf("revocation did not release exactly one slice: %+v", claims)
	}
}

func TestOwnerPromotesAmbiguousLinkedWorkToPartialSuppressionWithoutClosingOwnership(t *testing.T) {
	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "continuity.json"))
	if err != nil {
		t.Fatal(err)
	}
	obs := fixtureObservation("head-1")
	obs.LinkedWork = []WorkRelationship{{WorkRef: "acme/widgets#9", Relationship: RelationshipReferences, Evidence: "Progresses #9", Ambiguous: true}}
	rec, err := ledger.Adopt(obs, "owner", "verified-owner-dashboard", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	promoted, err := ledger.PromoteSuppression(obs.Ref, "acme/widgets#9", rec.Generation, "owner", "verified-owner-dashboard", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(promoted.SuppressionClaims) != 1 || promoted.SuppressionClaims[0].WorkRef != "acme/widgets#9" {
		t.Fatalf("suppression claims=%+v", promoted.SuppressionClaims)
	}
	if promoted.LinkedWork[0].Relationship != RelationshipReferences || !promoted.LinkedWork[0].Ambiguous || promoted.LinkedWork[0].OwnedSlice != "" {
		t.Fatalf("partial suppression was confused with closing acceptance: %+v", promoted.LinkedWork[0])
	}
	if got := ledger.LookupWork("acme/widgets#9"); len(got) != 1 {
		t.Fatalf("promoted partial work did not suppress replacement: %+v", got)
	}
	if _, err := OpenLedger(filepath.Join(filepath.Dir(ledger.path), "continuity.json")); err != nil {
		t.Fatalf("durable suppression ledger did not reload: %v", err)
	}
}

func TestSuppressionPromotionRequiresOwnerAuthorityAndDiscoveredRelationship(t *testing.T) {
	ledger, _ := OpenLedger(filepath.Join(t.TempDir(), "continuity.json"))
	obs := fixtureObservation("head-1")
	obs.LinkedWork = []WorkRelationship{{WorkRef: "acme/widgets#9", Relationship: RelationshipReferences, Ambiguous: true}}
	rec, _ := ledger.Adopt(obs, "owner", "verified-owner-dashboard", time.Now())
	if _, err := ledger.PromoteSuppression(obs.Ref, "acme/widgets#9", rec.Generation, "", "github-label:hive-adopt", time.Now()); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("agent/label authority promoted suppression: %v", err)
	}
	if _, err := ledger.PromoteSuppression(obs.Ref, "acme/widgets#99", rec.Generation, "owner", "verified-owner-dashboard", time.Now()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("undiscovered relationship promoted: %v", err)
	}
	closing := fixtureObservation("head-2")
	closing.Ref.Number = 18
	closing.LinkedWork = []WorkRelationship{{WorkRef: "acme/widgets#10", Relationship: RelationshipCloses, OwnedSlice: "complete issue"}}
	closingRec, _ := ledger.Adopt(closing, "owner", "verified-owner-dashboard", time.Now())
	if _, err := ledger.PromoteSuppression(closing.Ref, "acme/widgets#10", closingRec.Generation, "owner", "verified-owner-dashboard", time.Now()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("closing ownership was promoted as partial suppression: %v", err)
	}
}

func TestSuppressionPromotionRetryIsIdempotent(t *testing.T) {
	ledger, _ := OpenLedger(filepath.Join(t.TempDir(), "continuity.json"))
	obs := fixtureObservation("head-1")
	obs.LinkedWork = []WorkRelationship{{WorkRef: "acme/widgets#9", Relationship: RelationshipReferences, Ambiguous: true}}
	rec, _ := ledger.Adopt(obs, "owner", "verified-owner-dashboard", time.Now())
	first, err := ledger.PromoteSuppression(obs.Ref, "acme/widgets#9", rec.Generation, "owner", "verified-owner-dashboard", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	retry, err := ledger.PromoteSuppression(obs.Ref, "acme/widgets#9", rec.Generation, "owner", "verified-owner-dashboard", time.Now())
	if err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
	if retry.Generation != first.Generation || len(retry.History) != len(first.History) || len(retry.SuppressionClaims) != 1 {
		t.Fatalf("retry mutated authority: first=%+v retry=%+v", first, retry)
	}
}

func TestObservationCannotInjectSuppressionAuthority(t *testing.T) {
	ledger, _ := OpenLedger(filepath.Join(t.TempDir(), "continuity.json"))
	obs := fixtureObservation("head-1")
	obs.LinkedWork = []WorkRelationship{{WorkRef: "acme/widgets#9", Relationship: RelationshipReferences, Ambiguous: true}}
	obs.SuppressionClaims = []SuppressionClaim{{WorkRef: "acme/widgets#9", Principal: "agent", Provenance: "github-label:hive-adopt", Active: true}}

	rec, err := ledger.Adopt(obs, "owner", "verified-owner-dashboard", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.SuppressionClaims) != 0 || len(ledger.LookupWork("acme/widgets#9")) != 0 {
		t.Fatalf("observer injected suppression authority: %+v", rec.SuppressionClaims)
	}
}

func TestForkOrUnwritableObservationCannotBecomeContinuation(t *testing.T) {
	ledger, _ := OpenLedger(filepath.Join(t.TempDir(), "continuity.json"))
	obs := fixtureObservation("head-1")
	obs.HeadRepo = "alice/widgets-fork"
	obs.WriteCapability = CapabilityUnwritable
	obs.State = StateBlocked
	obs.StateReason = "head repository is not writable by the Hive App"
	rec, err := ledger.Adopt(obs, "owner", "session", time.Now())
	if err != nil {
		t.Fatalf("blocked work remains adoptable for ownership/suppression: %v", err)
	}
	if rec.State != StateBlocked || rec.Continuable() {
		t.Fatalf("unwritable branch became continuable: %+v", rec)
	}
}

func TestRefreshIsIdempotentAndObserverFailureFailsClosedLocally(t *testing.T) {
	ledger, _ := OpenLedger(filepath.Join(t.TempDir(), "continuity.json"))
	now := time.Now().UTC()
	obs := fixtureObservation("head-1")
	rec, _ := ledger.Adopt(obs, "owner", "session", now)
	refreshed, err := ledger.Refresh(obs, now.Add(time.Minute))
	if err != nil || refreshed.Generation != rec.Generation || len(refreshed.History) != len(rec.History) {
		t.Fatalf("unchanged refresh mutated authority: %+v err=%v", refreshed, err)
	}
	unknown, err := ledger.Degrade(obs.Ref, "github observer unavailable", "github:error", now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if unknown.State != StateUnknown || !unknown.Active || unknown.Generation <= rec.Generation {
		t.Fatalf("degraded source did not fail closed locally: %+v", unknown)
	}
}

func TestAcceptVerifiedDeliveryAdvancesAuthorityWithoutOwnerReacquisition(t *testing.T) {
	ledger, _ := OpenLedger(filepath.Join(t.TempDir(), "continuity.json"))
	now := time.Now().UTC()
	original := fixtureObservation("head-1")
	_, _ = ledger.Adopt(original, "owner", "session", now)
	moved := fixtureObservation("head-2")
	moved.State = StateReady
	moved.StateReason = "verified continuation delivered"
	_, _ = ledger.Refresh(moved, now.Add(time.Minute))

	delivered, err := ledger.AcceptDelivery(moved, "head-1", "hive-bot", "verified-contributor-receipt", now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("AcceptDelivery: %v", err)
	}
	if delivered.ObservedHeadSHA != "head-2" || delivered.CurrentHeadSHA != "head-2" || delivered.State != StateReady || !delivered.Active {
		t.Fatalf("verified delivery did not advance adopted authority: %+v", delivered)
	}
	if got := delivered.History[len(delivered.History)-1]; got.Verb != "delivery" || got.Principal != "hive-bot" {
		t.Fatalf("delivery receipt not durable/auditable: %+v", got)
	}
	if _, err := ledger.AcceptDelivery(fixtureObservation("head-3"), "stale-head", "hive-bot", "receipt", now.Add(3*time.Minute)); !errors.Is(err, ErrHeadMoved) {
		t.Fatalf("stale delivery receipt must fail closed: %v", err)
	}
}

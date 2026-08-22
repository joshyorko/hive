package adoption

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kubestellar/hive/pkg/convergence"
	"github.com/kubestellar/hive/pkg/convergence/outcome"
	"github.com/kubestellar/hive/pkg/worksource"
)

const actor = "dogfood-operator"

func ref(repo string, number int) worksource.Ref {
	return worksource.Ref{SourceType: "github", Repo: repo, Number: number}
}

func issue(repo string, number int, state string) worksource.Issue {
	return worksource.Issue{SourceType: "github", Repo: repo, Number: number, State: state, UpdatedAt: time.Unix(int64(number), 0)}
}

func snapshot(issues ...worksource.Issue) worksource.DependencySnapshot {
	return worksource.DependencySnapshot{Authority: []string{"joshyorko/actions", "joshyorko/rcc"}, Issues: issues}
}

func projectSnapshot(issues ...worksource.Issue) worksource.DependencySnapshot {
	return snapshot(append([]worksource.Issue{
		issue("joshyorko/actions", 101, "open"),
		issue("joshyorko/actions", 82, "open"),
		issue("joshyorko/rcc", 118, "open"),
	}, issues...)...)
}

func acceptedRecord(t *testing.T, spec Spec) outcome.Record {
	t.Helper()
	ledger, err := outcome.Open(filepath.Join(t.TempDir(), "outcomes.json"), outcome.Options{Principals: []string{actor}})
	if err != nil {
		t.Fatal(err)
	}
	rec, err := Propose(ledger, outcome.Ref{Project: "actions-rcc", Repo: "joshyorko/actions", Outcome: "existing-backlog-adoption"}, spec, actor)
	if err != nil {
		t.Fatal(err)
	}
	rec, err = Promote(ledger, rec.Ref, rec.Generation, spec, projectSnapshot(
		issue("joshyorko/rcc", 120, "open"),
		issue("joshyorko/actions", 134, "open"),
		issue("joshyorko/actions", 143, "open"),
	), actor)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func chainSpec() Spec {
	return Spec{
		Version:      1,
		Project:      "actions-rcc",
		Repositories: []string{"joshyorko/actions", "joshyorko/rcc"},
		Roots:        []string{"joshyorko/actions#101", "joshyorko/actions#82", "joshyorko/rcc#118"},
		Edges: []Edge{
			{From: "joshyorko/actions#134", DependsOn: "joshyorko/rcc#120", State: EdgePromoted, Evidence: Evidence{Source: "joshyorko/actions#101", Excerpt: "publish the RCC release before the first Actions consumer"}},
			{From: "joshyorko/actions#143", DependsOn: "joshyorko/actions#134", State: EdgePromoted, Evidence: Evidence{Source: "joshyorko/actions#101", Excerpt: "land the first consumer before downstream worker integration"}},
		},
	}
}

func TestProposedOutcomeCannotGate(t *testing.T) {
	ledger, err := outcome.Open(filepath.Join(t.TempDir(), "outcomes.json"), outcome.Options{Principals: []string{actor}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Propose(ledger, outcome.Ref{Project: "actions-rcc", Repo: "joshyorko/actions", Outcome: "existing-backlog-adoption"}, chainSpec(), actor); err != nil {
		t.Fatal(err)
	}
	obs := Observe(ledger.List(), snapshot(issue("joshyorko/rcc", 120, "open"), issue("joshyorko/actions", 134, "open")), ref("joshyorko/actions", 134))
	if obs.Found || len(obs.Dependencies) != 0 {
		t.Fatalf("proposed planning must be inert: %+v", obs)
	}
}

func TestPromotedRoadmapChainIsLevelTriggered(t *testing.T) {
	rec := acceptedRecord(t, chainSpec())
	open := snapshot(issue("joshyorko/rcc", 120, "open"), issue("joshyorko/actions", 134, "open"), issue("joshyorko/actions", 143, "open"), issue("joshyorko/rcc", 121, "open"))
	d := convergence.Evaluate(Observe([]outcome.Record{rec}, open, ref("joshyorko/actions", 134)))
	if d.Admitted || d.Reason != convergence.ReasonWaitingForDependency {
		t.Fatalf("#134 must wait for open RCC #120: %+v", d)
	}
	closed := snapshot(issue("joshyorko/rcc", 120, "closed"), issue("joshyorko/actions", 134, "open"), issue("joshyorko/actions", 143, "open"), issue("joshyorko/rcc", 121, "open"))
	if d = convergence.Evaluate(Observe([]outcome.Record{rec}, closed, ref("joshyorko/actions", 134))); !d.Admitted {
		t.Fatalf("#134 must release without restart: %+v", d)
	}
	if d = convergence.Evaluate(Observe([]outcome.Record{rec}, closed, ref("joshyorko/actions", 143))); d.Admitted {
		t.Fatalf("#143 must still wait for open #134: %+v", d)
	}
	if d = convergence.Evaluate(Observe([]outcome.Record{rec}, open, ref("joshyorko/rcc", 121))); !d.Admitted {
		t.Fatalf("unrelated lane must remain parallel: %+v", d)
	}
}

func TestPromotionRejectsMissingSelfAndCycleLocally(t *testing.T) {
	cases := map[string][]Edge{
		"missing": {{From: "joshyorko/actions#134", DependsOn: "joshyorko/rcc#999", State: EdgePromoted}},
		"self":    {{From: "joshyorko/actions#134", DependsOn: "joshyorko/actions#134", State: EdgePromoted}},
		"cycle": {
			{From: "joshyorko/actions#134", DependsOn: "joshyorko/rcc#120", State: EdgePromoted},
			{From: "joshyorko/rcc#120", DependsOn: "joshyorko/actions#134", State: EdgePromoted},
		},
	}
	for name, edges := range cases {
		t.Run(name, func(t *testing.T) {
			spec := chainSpec()
			spec.Edges = edges
			for i := range spec.Edges {
				spec.Edges[i].Evidence = Evidence{Source: "joshyorko/actions#101", Excerpt: "explicit ordering"}
			}
			if err := ValidatePromotion(spec, projectSnapshot(issue("joshyorko/rcc", 120, "open"), issue("joshyorko/actions", 134, "open"))); err == nil {
				t.Fatal("unsafe promoted graph accepted")
			}
		})
	}
}

func TestProposedAndAmbiguousEdgesNeverGate(t *testing.T) {
	spec := chainSpec()
	spec.Edges = []Edge{
		{From: "joshyorko/actions#134", DependsOn: "joshyorko/rcc#120", State: EdgeProposed, Classification: "related-only"},
		{From: "joshyorko/actions#143", DependsOn: "joshyorko/rcc#120", State: EdgeRejected, Classification: "ambiguous-prose"},
	}
	if err := ValidatePromotion(spec, projectSnapshot(issue("joshyorko/rcc", 120, "open"), issue("joshyorko/actions", 134, "open"), issue("joshyorko/actions", 143, "open"))); err != nil {
		t.Fatal(err)
	}
}

func TestRestartReconstructsAcceptedGraphWithoutCreatingWork(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outcomes.json")
	ledger, err := outcome.Open(path, outcome.Options{Principals: []string{actor}})
	if err != nil {
		t.Fatal(err)
	}
	oref := outcome.Ref{Project: "actions-rcc", Repo: "joshyorko/actions", Outcome: "existing-backlog-adoption"}
	rec, err := Propose(ledger, oref, chainSpec(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Promote(ledger, oref, rec.Generation, chainSpec(), projectSnapshot(issue("joshyorko/rcc", 120, "open"), issue("joshyorko/actions", 134, "open"), issue("joshyorko/actions", 143, "open")), actor); err != nil {
		t.Fatal(err)
	}
	reloaded, err := outcome.Open(path, outcome.Options{Principals: []string{actor}})
	if err != nil {
		t.Fatal(err)
	}
	obs := Observe(reloaded.List(), snapshot(issue("joshyorko/rcc", 120, "open"), issue("joshyorko/actions", 134, "open")), ref("joshyorko/actions", 134))
	if !obs.Found || len(obs.Dependencies) != 1 {
		t.Fatalf("accepted graph not reconstructed: %+v", obs)
	}
	if got := reloaded.List()[0].WorkRefs; len(got) != 6 {
		t.Fatalf("canonical existing refs = %v, want 6; adoption must not create replacement work", got)
	}
}

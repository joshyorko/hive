package adoption

import (
	"strings"
	"testing"

	"github.com/kubestellar/hive/pkg/worksource"
)

func TestArchitectInventoryFollowsExistingRefsWithinFiniteScope(t *testing.T) {
	snap := projectSnapshot(
		worksource.Issue{Repo: "joshyorko/actions", Number: 101, State: "open", Body: "Start #134 after joshyorko/rcc#120. Related: outside/repo#9"},
		worksource.Issue{Repo: "joshyorko/actions", Number: 134, State: "open", Body: "Evidence input for #143"},
		worksource.Issue{Repo: "joshyorko/actions", Number: 143, State: "open"},
		worksource.Issue{Repo: "joshyorko/rcc", Number: 120, State: "open"},
		worksource.Issue{Repo: "outside/repo", Number: 9, State: "open"},
	)
	items := Inventory(snap, []string{"joshyorko/actions#101"}, 16)
	keys := map[string]bool{}
	for _, item := range items {
		keys[worksource.RefFromIssue(item).Key()] = true
	}
	for _, want := range []string{"joshyorko/actions#101", "joshyorko/actions#134", "joshyorko/actions#143", "joshyorko/rcc#120"} {
		if !keys[want] {
			t.Errorf("inventory missing %s: %v", want, keys)
		}
	}
	if keys["outside/repo#9"] {
		t.Fatal("inventory escaped configured repository scope")
	}
}

func TestBuildArchitectPromptRequiresExistingRefsEvidenceAndProposalOnly(t *testing.T) {
	seed := chainSpec()
	seed.Edges = nil
	prompt, err := BuildArchitectPrompt(seed, projectSnapshot(
		worksource.Issue{Repo: "joshyorko/actions", Number: 101, State: "open", Title: "roadmap", Body: "Start #134 after rcc#120"},
	), 64)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[agent:architect]", "existing worksource.Ref", "proposed", "exact evidence excerpt", "must not create", "joshyorko/actions#101"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestArchitectOutputIsInertUntilDeterministicallyAuthorized(t *testing.T) {
	raw := `{"version":1,"project":"actions-rcc","repositories":["joshyorko/actions","joshyorko/rcc"],"roots":["joshyorko/actions#101"],"edges":[{"from":"joshyorko/actions#134","depends_on":"joshyorko/rcc#120","state":"proposed","classification":"explicit-roadmap-order","evidence":{"source":"joshyorko/actions#101","excerpt":"Start #134 after joshyorko/rcc#120."}}]}`
	spec, err := ParseArchitectOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Edges[0].State != EdgeProposed {
		t.Fatal("architect output became authoritative during parse")
	}
	snap := snapshot(
		worksource.Issue{Repo: "joshyorko/actions", Number: 101, State: "open", Body: "Start #134 after joshyorko/rcc#120."},
		worksource.Issue{Repo: "joshyorko/actions", Number: 134, State: "open"},
		worksource.Issue{Repo: "joshyorko/rcc", Number: 120, State: "open"},
	)
	authorized, report, err := AuthorizeExplicitEdges(spec, snap)
	if err != nil {
		t.Fatal(err)
	}
	if authorized.Edges[0].State != EdgePromoted || report.Promoted != 1 {
		t.Fatalf("authorized=%+v report=%+v", authorized, report)
	}
}

func TestArchitectAmbiguityAndInventedEvidenceStayProposed(t *testing.T) {
	spec := Spec{Version: 1, Project: "actions-rcc", Repositories: []string{"joshyorko/actions", "joshyorko/rcc"}, Roots: []string{"joshyorko/actions#101"}, Edges: []Edge{
		{From: "joshyorko/actions#134", DependsOn: "joshyorko/rcc#120", State: EdgeProposed, Classification: "ambiguous-prose", Evidence: Evidence{Source: "joshyorko/actions#101", Excerpt: "Related: rcc#120"}},
		{From: "joshyorko/actions#143", DependsOn: "joshyorko/actions#134", State: EdgeProposed, Classification: "explicit-roadmap-order", Evidence: Evidence{Source: "joshyorko/actions#101", Excerpt: "invented quote"}},
		{From: "joshyorko/actions#132", DependsOn: "joshyorko/actions#84", State: EdgeProposed, Classification: "explicit-producer-consumer-order", Evidence: Evidence{Source: "joshyorko/actions#101", Excerpt: "#84 is consumed by #132"}},
	}}
	snap := snapshot(
		worksource.Issue{Repo: "joshyorko/actions", Number: 101, State: "open", Body: "Related: rcc#120\n#84 is consumed by #132"},
		worksource.Issue{Repo: "joshyorko/actions", Number: 84, State: "open"},
		worksource.Issue{Repo: "joshyorko/actions", Number: 132, State: "open"},
		worksource.Issue{Repo: "joshyorko/actions", Number: 134, State: "open"},
		worksource.Issue{Repo: "joshyorko/actions", Number: 143, State: "open"},
		worksource.Issue{Repo: "joshyorko/rcc", Number: 120, State: "open"},
	)
	authorized, report, err := AuthorizeExplicitEdges(spec, snap)
	if err != nil {
		t.Fatal(err)
	}
	if report.Promoted != 0 || report.LeftProposed != 3 {
		t.Fatalf("report=%+v", report)
	}
	for _, edge := range authorized.Edges {
		if edge.State != EdgeProposed {
			t.Fatalf("unsafe edge promoted: %+v", edge)
		}
	}
}

func TestAuthorizeInvalidEdgesStayProposedWithoutSerializingValidGraph(t *testing.T) {
	spec := Spec{Version: 1, Project: "actions-rcc", Repositories: []string{"joshyorko/actions", "joshyorko/rcc"}, Roots: []string{"joshyorko/actions#101"}, Edges: []Edge{
		{From: "joshyorko/actions#134", DependsOn: "joshyorko/rcc#120", State: EdgeProposed, Classification: "explicit-roadmap-order", Evidence: Evidence{Source: "joshyorko/actions#101", Excerpt: "Start #134 after rcc#120."}},
		{From: "joshyorko/actions#134", DependsOn: "joshyorko/actions#128", State: EdgeProposed, Classification: "explicit-roadmap-order", Evidence: Evidence{Source: "joshyorko/actions#101", Excerpt: "Wait for #128."}},
		{From: "joshyorko/actions#140", DependsOn: "joshyorko/actions#141", State: EdgeProposed, Classification: "explicit-roadmap-order", Evidence: Evidence{Source: "joshyorko/actions#101", Excerpt: "Run #141 before #140."}},
		{From: "joshyorko/actions#141", DependsOn: "joshyorko/actions#140", State: EdgeProposed, Classification: "explicit-roadmap-order", Evidence: Evidence{Source: "joshyorko/actions#101", Excerpt: "Run #140 before #141."}},
	}}
	snap := snapshot(
		worksource.Issue{Repo: "joshyorko/actions", Number: 101, State: "open", Body: "Start #134 after rcc#120. Wait for #128. Run #141 before #140. Run #140 before #141."},
		worksource.Issue{Repo: "joshyorko/actions", Number: 134, State: "open"},
		worksource.Issue{Repo: "joshyorko/actions", Number: 140, State: "open"},
		worksource.Issue{Repo: "joshyorko/actions", Number: 141, State: "open"},
		worksource.Issue{Repo: "joshyorko/rcc", Number: 120, State: "open"},
	)

	authorized, report, err := AuthorizeExplicitEdges(spec, snap)
	if err != nil {
		t.Fatal(err)
	}
	if report.Promoted != 1 || report.LeftProposed != 3 {
		t.Fatalf("report=%+v", report)
	}
	for _, edge := range authorized.Edges {
		want := EdgeProposed
		if edge.From == "joshyorko/actions#134" && edge.DependsOn == "joshyorko/rcc#120" {
			want = EdgePromoted
		}
		if edge.State != want {
			t.Fatalf("edge state = %q, want %q: %+v", edge.State, want, edge)
		}
	}
}

package dashboard

import (
	"path/filepath"
	"testing"

	"github.com/kubestellar/hive/pkg/config"
	"github.com/kubestellar/hive/pkg/convergence/outcome"
	ghpkg "github.com/kubestellar/hive/pkg/github"
	"github.com/kubestellar/hive/pkg/planning/adoption"
	"github.com/kubestellar/hive/pkg/worksource"
)

func TestPlanningAdoption_ComposesWithQueueSelectionAndKickProjection(t *testing.T) {
	hub, server := covK2Hub(t)
	server.deps.Config.Project.Org = "joshyorko"
	server.deps.Config.Project.Repos = []string{"actions", "rcc"}
	server.deps.Config.Project.IssueFilter = config.IssueFilterConfig{RequireLabels: []string{"hive-managed"}}
	ledger, err := outcome.Open(filepath.Join(t.TempDir(), "outcomes.json"), outcome.Options{Principals: []string{"operator"}})
	if err != nil {
		t.Fatal(err)
	}
	spec := adoption.Spec{
		Version: 1, Project: "actions-rcc",
		Repositories: []string{"joshyorko/actions", "joshyorko/rcc"},
		Roots:        []string{"joshyorko/actions#101", "joshyorko/actions#82", "joshyorko/rcc#118"},
		Edges: []adoption.Edge{{
			From: "joshyorko/actions#134", DependsOn: "joshyorko/rcc#120", State: adoption.EdgePromoted,
			Evidence: adoption.Evidence{Source: "joshyorko/actions#101", Excerpt: "RCC release before first consumer"},
		}},
	}
	oref := outcome.Ref{Project: "actions-rcc", Repo: "joshyorko/actions", Outcome: "existing-backlog-adoption"}
	rec, err := adoption.Propose(ledger, oref, spec, "operator")
	if err != nil {
		t.Fatal(err)
	}
	source := worksource.DependencySnapshot{
		Authority: []string{"joshyorko/actions", "joshyorko/rcc"},
		Issues: []worksource.Issue{
			{Repo: "joshyorko/actions", Number: 101, State: "open"},
			{Repo: "joshyorko/actions", Number: 82, State: "open"},
			{Repo: "joshyorko/rcc", Number: 118, State: "open"},
			{Repo: "joshyorko/actions", Number: 134, State: "open"},
			{Repo: "joshyorko/rcc", Number: 120, State: "open"},
		},
	}
	if _, err := adoption.Promote(ledger, oref, rec.Generation, spec, source, "operator"); err != nil {
		t.Fatal(err)
	}
	server.deps.PlanningOutcomes = ledger
	server.statusMu.Lock()
	server.status = &StatusPayload{Repos: []FrontendRepo{
		{Full: "joshyorko/actions", Name: "actions", ActionableIssues: []any{
			map[string]any{"number": float64(134), "state": "open", "labels": []any{"hive-managed"}, "title": "first consumer"},
			map[string]any{"number": float64(135), "state": "open", "labels": []any{"hive-managed"}, "title": "independent lane"},
		}, SourceIssues: []any{
			map[string]any{"number": float64(134), "state": "open", "labels": []any{"hive-managed"}},
			map[string]any{"number": float64(135), "state": "open", "labels": []any{"hive-managed"}},
		}},
		{Full: "joshyorko/rcc", Name: "rcc", SourceIssues: []any{
			map[string]any{"number": float64(120), "state": "open", "labels": []any{"hive-managed"}},
		}},
	}}
	server.statusMu.Unlock()

	if got := queueNumbers(hub.ReadyQueue(readyQueueDefaultLimit)); len(got) != 1 || got[0] != 135 {
		t.Fatalf("ReadyQueue = %v, want independent #135 only", got)
	}
	actionable := []ghpkg.Issue{{Repo: "joshyorko/actions", Number: 134, State: "open", Labels: []string{"hive-managed"}}}
	sourceIssues := []ghpkg.Issue{
		actionable[0],
		{Repo: "joshyorko/rcc", Number: 120, State: "open"},
	}
	admitted, withheld, _ := server.ConvergenceKickProjectionDetailed(actionable, sourceIssues)
	if len(admitted) != 0 || len(withheld) != 1 || withheld[0].Issue.Number != 134 {
		t.Fatalf("kick projection admitted=%v withheld=%v, want #134 withheld", admitted, withheld)
	}
}

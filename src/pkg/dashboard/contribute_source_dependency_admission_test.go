package dashboard

import (
	"testing"
	"time"

	"github.com/kubestellar/hive/pkg/beads"
	"github.com/kubestellar/hive/pkg/config"
	"github.com/kubestellar/hive/pkg/convergence"
	ghpkg "github.com/kubestellar/hive/pkg/github"
)

// TestSourceDependencyAdmission_Actions147BodyDependencies is the sanitized
// dogfood RED from joshyorko/actions#147. The issue is enrolled/actionable and
// its explicit hard dependencies are open. Current v4 loses the issue body
// before the convergence observation, so the candidate is incorrectly offered.
func TestSourceDependencyAdmission_Actions147BodyDependencies(t *testing.T) {
	hub, s := covK2Hub(t)
	s.deps.Config.Project.IssueFilter = config.IssueFilterConfig{RequireLabels: []string{"hive-managed"}}
	s.statusMu.Lock()
	s.status = &StatusPayload{Repos: []FrontendRepo{{
		Name: "actions",
		Full: "joshyorko/actions",
		ActionableIssues: []any{
			map[string]any{
				"number": float64(147),
				"title":  "Add typed internal capability invocation",
				"body":   "Parent: #82\nDepends on: #83, #129, #130, #143, #144\nRelated: #85, #86, #87, #90, #92, #132, #133, #137, #138, #141, #142, #145, #146",
				"labels": []any{"hive-managed"},
				"state":  "open",
				"author": "joshyorko",
				"url":    "https://github.com/joshyorko/actions/issues/147",
			},
			map[string]any{"number": float64(83), "title": "open #83", "state": "open"},
			map[string]any{"number": float64(129), "title": "open #129", "state": "open"},
			map[string]any{"number": float64(130), "title": "open #130", "state": "open"},
			map[string]any{"number": float64(143), "title": "open #143", "state": "open"},
			map[string]any{"number": float64(144), "title": "open #144", "state": "open"},
		},
	}}}
	s.statusMu.Unlock()

	for _, item := range queueNumbers(hub.ReadyQueue(readyQueueDefaultLimit)) {
		if item == 147 {
			t.Fatalf("actions#147 was offered even though its open Depends on targets are present; source body was absent from the normalized observation")
		}
	}
}

func TestSourceDependencyAdmission_ReadyQueueAndSelectTaskParity(t *testing.T) {
	hub, s := covK2Hub(t)
	s.deps.Config.Project.Org = "projectbluefin"
	s.deps.Config.Project.Repos = []string{"dakota"}
	s.deps.Config.Project.IssueFilter = config.IssueFilterConfig{RequireLabels: []string{"hive-managed"}}
	s.statusMu.Lock()
	s.status = &StatusPayload{Repos: []FrontendRepo{{
		Name: "dakota", Full: "projectbluefin/dakota",
		ActionableIssues: []any{
			map[string]any{"number": float64(601), "title": "blocked", "state": "open", "labels": []any{"hive-managed"}, "body": "Depends on: #701", "url": "https://github.com/projectbluefin/dakota/issues/601"},
			map[string]any{"number": float64(700), "title": "ready", "state": "open", "labels": []any{"hive-managed"}, "url": "https://github.com/projectbluefin/dakota/issues/700"},
		},
		SourceIssues: []any{
			map[string]any{"number": float64(601), "title": "blocked", "state": "open", "labels": []any{"hive-managed"}, "body": "Depends on: #701"},
			map[string]any{"number": float64(700), "title": "ready", "state": "open", "labels": []any{"hive-managed"}},
			map[string]any{"number": float64(701), "title": "dependency", "state": "open"},
		},
	}}}
	s.statusMu.Unlock()

	if got := queueNumbers(hub.ReadyQueue(readyQueueDefaultLimit)); len(got) != 1 || got[0] != 700 {
		t.Fatalf("ReadyQueue numbers = %v, want only #700", got)
	}
	msg := hub.selectTask(&ContributorConnection{
		profile:  &ContributorProfile{GitHubUsername: "alice", ContributorID: "source-parity", TrustTier: "contributor"},
		lastPong: time.Now(),
	})
	if msg == nil || msg.Type != "task_assign" || msg.Number != 700 {
		t.Fatalf("selectTask = %+v, want assignment for #700", msg)
	}
}

func TestSourceDependencyAdmission_ComposesConservativelyWithBeads(t *testing.T) {
	store := depTestStore(t)
	blockerID := seedDependentBead(t, store, "gh-projectbluefin/dakota#601")
	hub, s := depTestHub(t, map[string]*beads.Store{"scanner": store})

	s.statusMu.Lock()
	s.status.Repos[0].ActionableIssues[0].(map[string]any)["labels"] = []any{"hive-ready"}
	s.status.Repos[0].SourceIssues = []any{
		map[string]any{"number": float64(601), "state": "open", "labels": []any{"hive-ready"}, "body": "Depends on: #701"},
		map[string]any{"number": float64(701), "state": "open"},
	}
	s.statusMu.Unlock()

	assertQueue(t, hub, 700)
	if err := store.Close(blockerID); err != nil {
		t.Fatalf("closing bead blocker: %v", err)
	}
	assertQueue(t, hub, 700)

	s.statusMu.Lock()
	s.status.Repos[0].SourceIssues[1].(map[string]any)["state"] = "closed"
	s.statusMu.Unlock()
	assertQueue(t, hub, 601, 700)

	s.statusMu.Lock()
	s.status.Repos[0].SourceIssues[1].(map[string]any)["state"] = "open"
	s.statusMu.Unlock()
	assertQueue(t, hub, 700)
}

func TestSourceDependencyAdmission_KickProjectionUsesTheSameSnapshot(t *testing.T) {
	_, s := covK2Hub(t)
	s.deps.Config.Project.Org = "projectbluefin"
	s.deps.Config.Project.Repos = []string{"dakota"}
	actionable := []ghpkg.Issue{
		{Repo: "projectbluefin/dakota", Number: 601, State: "open", Labels: []string{"hive-ready"}, Body: "Depends on: #701"},
	}
	source := []ghpkg.Issue{
		actionable[0],
		{Repo: "projectbluefin/dakota", Number: 701, State: "open"},
	}

	admitted, withheld, _ := s.ConvergenceKickProjectionDetailed(actionable, source)
	if len(admitted) != 0 {
		t.Fatalf("admitted = %+v, want no candidate admitted", admitted)
	}
	if len(withheld) != 1 || withheld[0].Issue.Number != 601 {
		t.Fatalf("withheld = %+v, want #601", withheld)
	}
	if withheld[0].Decision.Reason != convergence.ReasonWaitingForDependency {
		t.Fatalf("reason = %q, want WaitingForDependency", withheld[0].Decision.Reason)
	}
}

func TestSourceDependencyAdmission_DuplicateStatusUsesConservativePrecedence(t *testing.T) {
	merged := composeAdmissionObservations(
		convergence.Observation{Found: true, Dependencies: []convergence.Dependency{{ID: "dep", Status: convergence.ConditionUnknown}}},
		convergence.Observation{Found: true, Dependencies: []convergence.Dependency{{ID: "dep", Status: convergence.ConditionTrue}}},
	)
	if len(merged.Dependencies) != 1 || merged.Dependencies[0].Status != convergence.ConditionUnknown {
		t.Fatalf("unknown versus true = %+v, want one Unknown dependency", merged.Dependencies)
	}
	merged = composeAdmissionObservations(
		merged,
		convergence.Observation{Found: true, Dependencies: []convergence.Dependency{{ID: "dep", Status: convergence.ConditionFalse}}},
	)
	if len(merged.Dependencies) != 1 || merged.Dependencies[0].Status != convergence.ConditionFalse {
		t.Fatalf("false versus unknown = %+v, want one False dependency", merged.Dependencies)
	}
}

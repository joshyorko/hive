package worksource_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kubestellar/hive/pkg/convergence"
	"github.com/kubestellar/hive/pkg/worksource"
)

func TestTaskKey_GitHub(t *testing.T) {
	issue := worksource.Issue{
		SourceType: "github",
		Repo:       "my-org/my-repo",
		Number:     42,
		ExternalID: "42",
	}
	got := worksource.TaskKey(issue)
	want := "my-org/my-repo#42"
	if got != want {
		t.Errorf("TaskKey = %q, want %q", got, want)
	}
}

func TestTaskKey_Linear(t *testing.T) {
	issue := worksource.Issue{
		SourceType: "linear",
		Repo:       "my-org/my-repo",
		ExternalID: "ENG-123",
	}
	got := worksource.TaskKey(issue)
	want := "my-org/my-repo!ENG-123"
	if got != want {
		t.Errorf("TaskKey = %q, want %q", got, want)
	}
}

func TestTaskKey_NoCollision(t *testing.T) {
	// Two teams both have issue 42 — keys must be distinct.
	eng := worksource.Issue{SourceType: "linear", Repo: "org/repo", ExternalID: "ENG-42"}
	ops := worksource.Issue{SourceType: "linear", Repo: "org/repo", ExternalID: "OPS-42"}
	if worksource.TaskKey(eng) == worksource.TaskKey(ops) {
		t.Errorf("TaskKey collision: ENG-42 and OPS-42 produced the same key %q", worksource.TaskKey(eng))
	}
}

func TestTaskKey_Jira(t *testing.T) {
	issue := worksource.Issue{
		SourceType: "jira",
		Repo:       "my-org/my-repo",
		ExternalID: "ENG-42",
	}
	got := worksource.TaskKey(issue)
	want := "my-org/my-repo!ENG-42"
	if got != want {
		t.Errorf("TaskKey = %q, want %q", got, want)
	}
}

func TestObserveDependencies_ExactGrammarAndDeduplication(t *testing.T) {
	obs := worksource.ObserveDependencies(worksource.DependencySnapshot{
		Authority: []string{"acme/actions", "acme/rcc"},
		Issues: []worksource.Issue{
			{Repo: "acme/actions", Number: 1, State: "open", Labels: []string{"hive-ready"}, Body: "Parent: #4\nRelated: #5\nProgresses: #6\nDepends on: #2, #2\nBlocked by: acme/rcc#7"},
			{Repo: "acme/actions", Number: 2, State: "closed"},
			{Repo: "acme/rcc", Number: 7, State: "open"},
			{Repo: "acme/actions", Number: 4, State: "open"},
			{Repo: "acme/actions", Number: 5, State: "open"},
			{Repo: "acme/actions", Number: 6, State: "open"},
		},
	}, worksource.Ref{Repo: "acme/actions", Number: 1})
	if !obs.Found || len(obs.Dependencies) != 2 {
		t.Fatalf("observation = %+v, want two hard dependencies", obs)
	}
	if obs.Dependencies[0].ID != "acme/actions#2" || obs.Dependencies[0].Status != convergence.ConditionTrue {
		t.Fatalf("same-repo dependency = %+v, want closed target", obs.Dependencies[0])
	}
	if obs.Dependencies[1].ID != "acme/rcc#7" || obs.Dependencies[1].Status != convergence.ConditionFalse {
		t.Fatalf("cross-repo dependency = %+v, want open target", obs.Dependencies[1])
	}
}

func TestObserveDependencies_ReopenUnknownAndUnauthorizedTargets(t *testing.T) {
	snapshot := worksource.DependencySnapshot{
		Authority: []string{"acme/actions"},
		Issues: []worksource.Issue{
			{Repo: "acme/actions", Number: 1, State: "open", Labels: []string{"hive-ready"}, Body: "Depends on: #2"},
			{Repo: "acme/actions", Number: 2, State: "closed"},
		},
	}
	subject := worksource.Ref{Repo: "acme/actions", Number: 1}
	if got := worksource.ObserveDependencies(snapshot, subject).Dependencies[0].Status; got != convergence.ConditionTrue {
		t.Fatalf("closed target status = %q, want True", got)
	}
	snapshot.Issues[1].State = "open"
	if got := worksource.ObserveDependencies(snapshot, subject).Dependencies[0].Status; got != convergence.ConditionFalse {
		t.Fatalf("reopened target status = %q, want False", got)
	}
	unknown := worksource.ObserveDependencies(worksource.DependencySnapshot{
		Authority: []string{"acme/actions"},
		Issues: []worksource.Issue{
			{Repo: "acme/actions", Number: 1, State: "open", Labels: []string{"hive-ready"}, Body: "Depends on: #404, #405, evil/other#9, not-a-reference"},
			{Repo: "acme/actions", Number: 405},
		},
	}, subject)
	if len(unknown.Dependencies) != 4 {
		t.Fatalf("unknown dependencies = %+v, want four", unknown.Dependencies)
	}
	for _, dep := range unknown.Dependencies {
		if dep.Status != convergence.ConditionUnknown {
			t.Errorf("dependency %q status = %q, want Unknown", dep.ID, dep.Status)
		}
	}
	foundAuthorityDiagnosis := false
	for _, dep := range unknown.Dependencies {
		if dep.ID == "evil/other#9" {
			foundAuthorityDiagnosis = strings.Contains(dep.Detail, "authority")
			break
		}
	}
	if !foundAuthorityDiagnosis {
		t.Errorf("out-of-authority dependency = %+v, want authority diagnosis", unknown.Dependencies)
	}
}

func TestObserveDependencies_ReconstructsFromFreshSnapshot(t *testing.T) {
	snapshot := worksource.DependencySnapshot{
		Authority: []string{"acme/actions"},
		Issues: []worksource.Issue{
			{Repo: "acme/actions", Number: 1, State: "open", Labels: []string{"hive-ready"}, Body: "Depends on: #2", UpdatedAt: time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)},
			{Repo: "acme/actions", Number: 2, State: "closed"},
		},
	}
	subject := worksource.Ref{Repo: "acme/actions", Number: 1}
	first := worksource.ObserveDependencies(snapshot, subject)
	restarted := worksource.DependencySnapshot{
		Authority: append([]string(nil), snapshot.Authority...),
		Issues:    append([]worksource.Issue(nil), snapshot.Issues...),
	}
	second := worksource.ObserveDependencies(restarted, subject)
	if !first.Found || !second.Found || first.RecordID != second.RecordID || first.Generation != second.Generation {
		t.Fatalf("fresh observations differ: first=%+v second=%+v", first, second)
	}
	if len(second.Dependencies) != 1 || second.Dependencies[0].Status != convergence.ConditionTrue {
		t.Fatalf("restarted dependency = %+v, want closed target satisfied", second.Dependencies)
	}
}

func TestObserveDependencies_SelfAndCycleAreDiagnosed(t *testing.T) {
	self := worksource.ObserveDependencies(worksource.DependencySnapshot{
		Authority: []string{"acme/actions"},
		Issues:    []worksource.Issue{{Repo: "acme/actions", Number: 1, State: "open", Labels: []string{"hive-ready"}, Body: "Depends on: #1"}},
	}, worksource.Ref{Repo: "acme/actions", Number: 1})
	if len(self.Dependencies) != 1 || self.Dependencies[0].Status != convergence.ConditionFalse || !strings.Contains(self.Dependencies[0].Detail, "self") {
		t.Fatalf("self dependency = %+v, want diagnosed blocker", self.Dependencies)
	}
	cycle := worksource.ObserveDependencies(worksource.DependencySnapshot{
		Authority: []string{"acme/actions"},
		Issues: []worksource.Issue{
			{Repo: "acme/actions", Number: 2, State: "open", Labels: []string{"hive-ready"}, Body: "Depends on: #3"},
			{Repo: "acme/actions", Number: 3, State: "open", Labels: []string{"hive-ready"}, Body: "Depends on: #2"},
		},
	}, worksource.Ref{Repo: "acme/actions", Number: 2})
	if len(cycle.Dependencies) != 1 || cycle.Dependencies[0].Status != convergence.ConditionFalse || !strings.Contains(cycle.Dependencies[0].Detail, "cycle") {
		t.Fatalf("cycle dependency = %+v, want diagnosed blocker", cycle.Dependencies)
	}
}

func TestObserveDependencies_EnrollmentAndNonBlockingRelationships(t *testing.T) {
	for _, body := range []string{"Parent: #2", "Related: #2", "Progresses: #2"} {
		obs := worksource.ObserveDependencies(worksource.DependencySnapshot{
			Authority: []string{"acme/actions"},
			Issues: []worksource.Issue{
				{Repo: "acme/actions", Number: 1, State: "open", Labels: []string{"hive-ready"}, Body: body},
				{Repo: "acme/actions", Number: 2, State: "open"},
			},
		}, worksource.Ref{Repo: "acme/actions", Number: 1})
		if obs.Found || len(obs.Dependencies) != 0 {
			t.Fatalf("%q created a hard dependency: %+v", body, obs)
		}
	}
	obs := worksource.ObserveDependencies(worksource.DependencySnapshot{
		Authority: []string{"acme/actions"},
		Issues: []worksource.Issue{
			{Repo: "acme/actions", Number: 1, State: "open", Body: "Depends on: #2"},
			{Repo: "acme/actions", Number: 2, State: "open"},
		},
	}, worksource.Ref{Repo: "acme/actions", Number: 1})
	if obs.Found || len(obs.Dependencies) != 0 {
		t.Fatalf("unenrolled source text must not gate admission: %+v", obs)
	}
}

func TestObserveDependencies_EmptyAuthorityDoesNotFollowOtherSnapshotRepos(t *testing.T) {
	obs := worksource.ObserveDependencies(worksource.DependencySnapshot{
		Issues: []worksource.Issue{
			{Repo: "acme/actions", Number: 1, State: "open", Labels: []string{"hive-ready"}, Body: "Depends on: acme/rcc#2"},
			{Repo: "acme/rcc", Number: 2, State: "open"},
		},
	}, worksource.Ref{Repo: "acme/actions", Number: 1})
	if len(obs.Dependencies) != 1 || obs.Dependencies[0].Status != convergence.ConditionUnknown {
		t.Fatalf("unconfigured cross-repo dependency = %+v, want Unknown", obs.Dependencies)
	}
}

func TestObserveDependencies_ConfiguredEnrollmentLabelControlsCandidate(t *testing.T) {
	snapshot := worksource.DependencySnapshot{
		Authority:        []string{"acme/actions"},
		EnrollmentLabels: []string{"hive-managed"},
		Issues: []worksource.Issue{
			{Repo: "acme/actions", Number: 1, State: "open", Labels: []string{"hive-ready"}, Body: "Depends on: #2"},
			{Repo: "acme/actions", Number: 2, State: "open"},
		},
	}
	subject := worksource.Ref{Repo: "acme/actions", Number: 1}
	if obs := worksource.ObserveDependencies(snapshot, subject); obs.Found || len(obs.Dependencies) != 0 {
		t.Fatalf("legacy readiness label enrolled candidate under hive-managed policy: %+v", obs)
	}
	snapshot.Issues[0].Labels = append(snapshot.Issues[0].Labels, "hive-managed")
	if obs := worksource.ObserveDependencies(snapshot, subject); !obs.Found || len(obs.Dependencies) != 1 || obs.Dependencies[0].Status != convergence.ConditionFalse {
		t.Fatalf("hive-managed candidate observation = %+v, want one open blocker", obs)
	}
}

func TestObserveDependencies_ExplicitURLsAndMarkdownWrappedFields(t *testing.T) {
	obs := worksource.ObserveDependencies(worksource.DependencySnapshot{
		Authority:        []string{"joshyorko/actions", "joshyorko/rcc"},
		EnrollmentLabels: []string{"hive-managed"},
		Issues: []worksource.Issue{
			{
				Repo: "joshyorko/actions", Number: 147, State: "open", Labels: []string{"hive-managed"},
				Body: "**Depends on:** [#83](https://github.com/joshyorko/actions/issues/83), https://github.com/joshyorko/rcc/issues/120\n- **Blocked by:** `#129`",
			},
			{Repo: "joshyorko/actions", Number: 83, State: "closed"},
			{Repo: "joshyorko/actions", Number: 129, State: "open"},
			{Repo: "joshyorko/rcc", Number: 120, State: "closed"},
		},
	}, worksource.Ref{Repo: "joshyorko/actions", Number: 147})
	if !obs.Found || len(obs.Dependencies) != 3 {
		t.Fatalf("markdown/url observation = %+v, want three explicit dependencies", obs)
	}
	want := map[string]convergence.ConditionStatus{
		"joshyorko/actions#83":  convergence.ConditionTrue,
		"joshyorko/actions#129": convergence.ConditionFalse,
		"joshyorko/rcc#120":     convergence.ConditionTrue,
	}
	for _, dep := range obs.Dependencies {
		if dep.Status != want[dep.ID] {
			t.Errorf("dependency %q status = %q, want %q", dep.ID, dep.Status, want[dep.ID])
		}
	}
}

func TestObserveDependencies_CommonMarkdownDependencyFields(t *testing.T) {
	for _, body := range []string{
		"**Depends on**: #2",
		"- Depends on: #2",
		"- [ ] Depends on: #2",
		"1. Depends on: #2",
	} {
		obs := worksource.ObserveDependencies(worksource.DependencySnapshot{
			Authority:        []string{"acme/actions"},
			EnrollmentLabels: []string{"hive-managed"},
			Issues: []worksource.Issue{
				{Repo: "acme/actions", Number: 1, State: "open", Labels: []string{"hive-managed"}, Body: body},
				{Repo: "acme/actions", Number: 2, State: "open"},
			},
		}, worksource.Ref{Repo: "acme/actions", Number: 1})
		if !obs.Found || len(obs.Dependencies) != 1 || obs.Dependencies[0].Status != convergence.ConditionFalse {
			t.Errorf("dependency field %q = %+v, want one open blocker", body, obs)
		}
	}
}

func TestObserveDependencies_FencedExamplesAreNonAuthoritative(t *testing.T) {
	obs := worksource.ObserveDependencies(worksource.DependencySnapshot{
		Authority:        []string{"acme/actions"},
		EnrollmentLabels: []string{"hive-managed"},
		Issues: []worksource.Issue{
			{Repo: "acme/actions", Number: 1, State: "open", Labels: []string{"hive-managed"}, Body: "```text\nDepends on: #2\n```"},
			{Repo: "acme/actions", Number: 2, State: "open"},
		},
	}, worksource.Ref{Repo: "acme/actions", Number: 1})
	if obs.Found || len(obs.Dependencies) != 0 {
		t.Fatalf("fenced dependency example created a blocker: %+v", obs)
	}
}

func TestObserveDependencies_ClosedHistoricalCycleIsSatisfiedAndSelfRemainsBlocked(t *testing.T) {
	cycle := worksource.ObserveDependencies(worksource.DependencySnapshot{
		Authority:        []string{"acme/actions"},
		EnrollmentLabels: []string{"hive-managed"},
		Issues: []worksource.Issue{
			{Repo: "acme/actions", Number: 1, State: "open", Labels: []string{"hive-managed"}, Body: "Depends on: #2"},
			{Repo: "acme/actions", Number: 2, State: "closed", Labels: []string{"hive-managed"}, Body: "Depends on: #1"},
		},
	}, worksource.Ref{Repo: "acme/actions", Number: 1})
	if len(cycle.Dependencies) != 1 || cycle.Dependencies[0].Status != convergence.ConditionTrue {
		t.Fatalf("closed historical cycle dependency = %+v, want authoritatively satisfied", cycle.Dependencies)
	}
	self := worksource.ObserveDependencies(worksource.DependencySnapshot{
		Authority:        []string{"acme/actions"},
		EnrollmentLabels: []string{"hive-managed"},
		Issues: []worksource.Issue{
			{Repo: "acme/actions", Number: 3, State: "closed", Labels: []string{"hive-managed"}, Body: "Depends on: #3"},
		},
	}, worksource.Ref{Repo: "acme/actions", Number: 3})
	if len(self.Dependencies) != 1 || self.Dependencies[0].Status != convergence.ConditionFalse || !strings.Contains(self.Dependencies[0].Detail, "self") {
		t.Fatalf("closed self dependency = %+v, want diagnosed False", self.Dependencies)
	}
}

func TestObserveDependencies_MetadataFieldsRemainNonBlocking(t *testing.T) {
	for _, body := range []string{
		"Parent: #2",
		"Related: #2",
		"Informs: #2",
		"Grounded by: #2",
		"Progresses: #2",
		"See https://github.com/acme/actions/issues/2 for context",
	} {
		obs := worksource.ObserveDependencies(worksource.DependencySnapshot{
			Authority:        []string{"acme/actions"},
			EnrollmentLabels: []string{"hive-managed"},
			Issues: []worksource.Issue{
				{Repo: "acme/actions", Number: 1, State: "open", Labels: []string{"hive-managed"}, Body: body},
				{Repo: "acme/actions", Number: 2, State: "open"},
			},
		}, worksource.Ref{Repo: "acme/actions", Number: 1})
		if obs.Found || len(obs.Dependencies) != 0 {
			t.Fatalf("metadata %q created a blocker: %+v", body, obs)
		}
	}
}

func TestToGitHubIssuesPreservesSourceObservationFields(t *testing.T) {
	out := worksource.ToGitHubIssues([]worksource.Issue{{Repo: "acme/actions", Number: 1, State: "open", Body: "Depends on: #2"}})
	if len(out) != 1 || out[0].Body != "Depends on: #2" || out[0].State != "open" {
		t.Fatalf("projected source fields = %+v, want body and state preserved", out)
	}
}

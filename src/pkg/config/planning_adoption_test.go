package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadPlanningAdoptionFiniteRoots(t *testing.T) {
	raw := []byte("planning:\n  adoption:\n    enabled: true\n    project: actions-rcc\n    repositories: [joshyorko/actions, joshyorko/rcc]\n    roots: [joshyorko/actions#101, joshyorko/actions#82, joshyorko/rcc#118]\n    outcome_repo: joshyorko/actions\n    outcome: existing-backlog-adoption\n")
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	got := cfg.Planning.Adoption
	if !got.Enabled || got.Project != "actions-rcc" || len(got.Repositories) != 2 || len(got.Roots) != 3 || got.OutcomeRepo != "joshyorko/actions" || got.Outcome != "existing-backlog-adoption" {
		t.Fatalf("planning adoption = %+v", got)
	}
}

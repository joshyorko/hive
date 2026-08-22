package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kubestellar/hive/pkg/config"
	"github.com/kubestellar/hive/pkg/convergence"
	"github.com/kubestellar/hive/pkg/convergence/outcome"
	"github.com/kubestellar/hive/pkg/dashboard"
	"github.com/kubestellar/hive/pkg/planning/adoption"
	"github.com/kubestellar/hive/pkg/worksource"
)

type result struct {
	Action     string         `json:"action"`
	Key        string         `json:"key"`
	Generation int            `json:"generation"`
	State      outcome.State  `json:"state"`
	WorkRefs   []string       `json:"work_refs,omitempty"`
	Spec       *adoption.Spec `json:"spec,omitempty"`
}

type evaluation struct {
	Work     string   `json:"work"`
	Ready    bool     `json:"ready"`
	Reason   string   `json:"reason"`
	Blockers []string `json:"blockers,omitempty"`
}

type evaluationResult struct {
	Action   string       `json:"action"`
	Total    int          `json:"total"`
	Admitted int          `json:"admitted"`
	Blocked  int          `json:"blocked"`
	Unknown  int          `json:"unknown"`
	Items    []evaluation `json:"items"`
}

func main() {
	var (
		action       = flag.String("action", "inspect", "prompt, authorize, propose, promote, validate, evaluate, or inspect")
		ledgerPath   = flag.String("ledger", "/data/outcomes.json", "outcome ledger path")
		specPath     = flag.String("spec", "", "adoption spec JSON path")
		configPath   = flag.String("config", "", "Hive config path used as the adoption seed")
		snapshotPath = flag.String("snapshot", "", "bounded dependency snapshot JSON path")
		outPath      = flag.String("out", "", "output path for prompt or authorized spec")
		actor        = flag.String("actor", "", "authorized operator principal")
		project      = flag.String("project", "actions-rcc", "outcome project")
		repo         = flag.String("repo", "joshyorko/actions", "outcome repository scope")
		name         = flag.String("name", "existing-backlog-adoption", "outcome slug")
	)
	flag.Parse()

	if (*action == "propose" || *action == "promote") && *actor == "" {
		fatalf("-actor is required")
	}
	ledger, err := outcome.Open(*ledgerPath, outcome.Options{Principals: []string{*actor}})
	if err != nil {
		fatalf("open ledger: %v", err)
	}
	oref := outcome.Ref{Project: *project, Repo: *repo, Outcome: *name}

	switch *action {
	case "prompt":
		prompt, err := adoption.BuildArchitectPrompt(readSeed(*specPath, *configPath), readSnapshot(*snapshotPath), 48)
		if err != nil {
			fatalf("build architect prompt: %v", err)
		}
		if *outPath == "" {
			fmt.Print(prompt)
		} else if err := writeFile(*outPath, []byte(prompt)); err != nil {
			fatalf("write prompt: %v", err)
		}
	case "authorize":
		raw, err := os.ReadFile(*specPath)
		if err != nil {
			fatalf("read architect proposal: %v", err)
		}
		spec, err := adoption.ParseArchitectOutput(string(raw))
		if err != nil {
			fatalf("parse architect proposal: %v", err)
		}
		authorized, report, err := adoption.AuthorizeExplicitEdges(spec, readSnapshot(*snapshotPath))
		if err != nil {
			fatalf("authorize architect proposal: %v", err)
		}
		if *outPath == "" {
			fatalf("-out is required for authorized spec")
		}
		writeJSON(*outPath, authorized)
		emit(report)
	case "inspect":
		for _, rec := range ledger.List() {
			var spec adoption.Spec
			if err := json.Unmarshal([]byte(rec.Spec), &spec); err != nil {
				fatalf("decode %s: %v", rec.Ref.Key(), err)
			}
			emit(result{Action: "inspect", Key: rec.Ref.Key(), Generation: rec.Generation, State: rec.State, WorkRefs: rec.WorkRefs, Spec: &spec})
		}
	case "validate":
		spec := readSpec(*specPath)
		snapshot := readSnapshot(*snapshotPath)
		if err := adoption.ValidatePromotion(spec, snapshot); err != nil {
			fatalf("validate: %v", err)
		}
		emit(result{Action: "validate", Key: oref.Key()})
	case "evaluate":
		snapshot := readSnapshot(*snapshotPath)
		report := evaluationResult{Action: "evaluate"}
		for _, issue := range snapshot.Issues {
			if !isEnrolled(issue, snapshot.EnrollmentLabels) || !isOpen(issue.State) {
				continue
			}
			candidate := worksource.RefFromIssue(issue)
			sourceObs := worksource.ObserveDependencies(snapshot, candidate)
			planningObs := adoption.Observe(ledger.List(), snapshot, candidate)
			decision := convergence.Evaluate(dashboard.ComposeAdmissionObservations(sourceObs, planningObs))
			item := evaluation{Work: candidate.Key(), Ready: decision.Admitted, Reason: decision.Reason, Blockers: decision.Blockers}
			report.Items = append(report.Items, item)
			report.Total++
			if decision.Admitted {
				report.Admitted++
			} else if decision.Reason == convergence.ReasonDependencyUnknown {
				report.Unknown++
			} else {
				report.Blocked++
			}
		}
		sort.Slice(report.Items, func(i, j int) bool { return report.Items[i].Work < report.Items[j].Work })
		emit(report)
	case "propose":
		spec := readSpec(*specPath)
		var rec outcome.Record
		if current, ok := ledger.Get(oref); ok {
			rec, err = adoption.Supersede(ledger, oref, current.Generation, spec, *actor)
			if err != nil {
				fatalf("supersede: %v", err)
			}
		} else {
			rec, err = adoption.Propose(ledger, oref, spec, *actor)
			if err != nil {
				fatalf("propose: %v", err)
			}
		}
		emit(result{Action: "propose", Key: rec.Ref.Key(), Generation: rec.Generation, State: rec.State, WorkRefs: rec.WorkRefs})
	case "promote":
		spec := readSpec(*specPath)
		snapshot := readSnapshot(*snapshotPath)
		rec, ok := ledger.Get(oref)
		if !ok {
			fatalf("outcome %s is not proposed", oref.Key())
		}
		rec, err = adoption.Promote(ledger, oref, rec.Generation, spec, snapshot, *actor)
		if err != nil {
			fatalf("promote: %v", err)
		}
		emit(result{Action: "promote", Key: rec.Ref.Key(), Generation: rec.Generation, State: rec.State, WorkRefs: rec.WorkRefs})
	default:
		fatalf("unknown -action %q", *action)
	}
}

func isEnrolled(issue worksource.Issue, required []string) bool {
	if len(required) == 0 {
		return true
	}
	for _, label := range issue.Labels {
		for _, want := range required {
			if strings.EqualFold(strings.TrimSpace(label), strings.TrimSpace(want)) {
				return true
			}
		}
	}
	return false
}

func isOpen(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "closed", "done", "complete", "completed", "resolved", "cancelled", "canceled":
		return false
	default:
		return true
	}
}

func readSpec(path string) adoption.Spec {
	var spec adoption.Spec
	readJSON(path, &spec)
	return spec
}

func readSeed(specPath, configPath string) adoption.Spec {
	if specPath != "" {
		return readSpec(specPath)
	}
	if configPath == "" {
		fatalf("prompt requires -spec or -config")
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		fatalf("load Hive config: %v", err)
	}
	a := cfg.Planning.Adoption
	if !a.Enabled {
		fatalf("planning.adoption is not enabled")
	}
	if a.Project == "" || len(a.Repositories) == 0 || len(a.Roots) == 0 {
		fatalf("planning.adoption requires project, repositories, and roots")
	}
	return adoption.Spec{Version: 1, Project: a.Project, Repositories: append([]string(nil), a.Repositories...), Roots: append([]string(nil), a.Roots...), Provenance: adoption.Provenance{Planner: "architect-existing-backlog-adoption"}}
}

func readSnapshot(path string) worksource.DependencySnapshot {
	var snapshot worksource.DependencySnapshot
	readJSON(path, &snapshot)
	return snapshot
}

func readJSON(path string, dst any) {
	if path == "" {
		fatalf("required JSON path is empty")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		fatalf("decode %s: %v", path, err)
	}
}

func writeJSON(path string, value any) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fatalf("encode %s: %v", path, err)
	}
	raw = append(raw, '\n')
	if err := writeFile(path, raw); err != nil {
		fatalf("write %s: %v", path, err)
	}
}

func writeFile(path string, raw []byte) error {
	dir := filepath.Dir(path)
	_, statErr := os.Stat(dir)
	created := os.IsNotExist(statErr)
	if err := os.MkdirAll(dir, 0o770); err != nil {
		return err
	}
	// Agent processes share the node group but not the operator UID. Preserve
	// group inheritance so the architect can complete the file handoff.
	if created {
		if err := os.Chmod(dir, 0o2770); err != nil {
			return err
		}
	}
	if statErr != nil && !created {
		return statErr
	}
	if err := os.WriteFile(path, raw, 0o660); err != nil {
		return err
	}
	return os.Chmod(path, 0o660)
}

func emit(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fatalf("encode result: %v", err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

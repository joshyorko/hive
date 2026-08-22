// Package adoption projects an approved Planning Intelligence graph over
// existing work into convergence dependency observations.
package adoption

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/kubestellar/hive/pkg/convergence"
	"github.com/kubestellar/hive/pkg/convergence/outcome"
	"github.com/kubestellar/hive/pkg/worksource"
)

type EdgeState string

const (
	EdgeProposed EdgeState = "proposed"
	EdgePromoted EdgeState = "promoted"
	EdgeRejected EdgeState = "rejected"
)

type Evidence struct {
	Source  string `json:"source"`
	Excerpt string `json:"excerpt"`
	Hash    string `json:"hash"`
}

type Edge struct {
	From           string    `json:"from"`
	DependsOn      string    `json:"depends_on"`
	State          EdgeState `json:"state"`
	Classification string    `json:"classification,omitempty"`
	Evidence       Evidence  `json:"evidence"`
}

type Provenance struct {
	Planner   string `json:"planner,omitempty"`
	RunID     string `json:"run_id,omitempty"`
	Generated string `json:"generated_at,omitempty"`
}

type Spec struct {
	Version      int        `json:"version"`
	Project      string     `json:"project"`
	Repositories []string   `json:"repositories"`
	Roots        []string   `json:"roots"`
	Edges        []Edge     `json:"edges"`
	Provenance   Provenance `json:"provenance,omitempty"`
}

// Propose records one inert adoption generation. It references existing work;
// it never creates issues or beads.
func Propose(ledger *outcome.Ledger, ref outcome.Ref, spec Spec, actor string) (outcome.Record, error) {
	if ledger == nil {
		return outcome.Record{}, fmt.Errorf("adoption proposal requires an outcome ledger")
	}
	normalized, err := normalize(spec)
	if err != nil {
		return outcome.Record{}, err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return outcome.Record{}, fmt.Errorf("marshal adoption spec: %w", err)
	}
	return ledger.Create(ref, string(raw), workRefs(normalized), actor)
}

// Supersede records a revised inert generation using the same canonical form
// as Propose, preserving exact-generation promotion semantics.
func Supersede(ledger *outcome.Ledger, ref outcome.Ref, generation int, spec Spec, actor string) (outcome.Record, error) {
	if ledger == nil {
		return outcome.Record{}, fmt.Errorf("adoption supersede requires an outcome ledger")
	}
	normalized, err := normalize(spec)
	if err != nil {
		return outcome.Record{}, err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return outcome.Record{}, fmt.Errorf("marshal adoption spec: %w", err)
	}
	return ledger.Supersede(ref, generation, string(raw), workRefs(normalized), actor)
}

// Promote validates the exact proposed generation against one bounded source
// snapshot, then accepts it through the outcome ledger's generation CAS.
func Promote(ledger *outcome.Ledger, ref outcome.Ref, generation int, spec Spec, snapshot worksource.DependencySnapshot, actor string) (outcome.Record, error) {
	if ledger == nil {
		return outcome.Record{}, fmt.Errorf("adoption promotion requires an outcome ledger")
	}
	normalized, err := normalize(spec)
	if err != nil {
		return outcome.Record{}, err
	}
	if err := ValidatePromotion(normalized, snapshot); err != nil {
		return outcome.Record{}, err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return outcome.Record{}, fmt.Errorf("marshal adoption spec: %w", err)
	}
	current, ok := ledger.Get(ref)
	if !ok {
		return outcome.Record{}, outcome.ErrOutcomeNotFound
	}
	if current.Generation != generation || current.Spec != string(raw) {
		return outcome.Record{}, fmt.Errorf("adoption promotion does not match proposed generation %d", generation)
	}
	return ledger.Accept(ref, generation, actor)
}

// ValidatePromotion fails the promoted portion of a graph closed without
// allowing an invalid lane to serialize unrelated work.
func ValidatePromotion(spec Spec, snapshot worksource.DependencySnapshot) error {
	normalized, err := normalize(spec)
	if err != nil {
		return err
	}
	authority := stringSet(normalized.Repositories)
	existing := make(map[string]bool, len(snapshot.Issues))
	for _, issue := range snapshot.Issues {
		key := worksource.RefFromIssue(issue).Key()
		if key != "" {
			existing[strings.ToLower(key)] = true
		}
	}
	graph := make(map[string][]string)
	for _, edge := range normalized.Edges {
		if edge.State != EdgePromoted {
			continue
		}
		from, fromOK := canonicalRef(edge.From)
		dep, depOK := canonicalRef(edge.DependsOn)
		if !fromOK || !depOK {
			return fmt.Errorf("promoted edge requires canonical existing refs: %q -> %q", edge.DependsOn, edge.From)
		}
		if !authority[strings.ToLower(from.Repo)] || !authority[strings.ToLower(dep.Repo)] {
			return fmt.Errorf("promoted edge escapes project repository scope: %s depends on %s", from.Key(), dep.Key())
		}
		if from.Key() == dep.Key() {
			return fmt.Errorf("promoted edge is a self-dependency: %s", from.Key())
		}
		if !existing[strings.ToLower(from.Key())] || !existing[strings.ToLower(dep.Key())] {
			return fmt.Errorf("promoted edge references missing work: %s depends on %s", from.Key(), dep.Key())
		}
		evidenceRef, evidenceOK := canonicalRef(edge.Evidence.Source)
		if !evidenceOK || !authority[strings.ToLower(evidenceRef.Repo)] || !existing[strings.ToLower(evidenceRef.Key())] {
			return fmt.Errorf("promoted edge lacks an existing in-scope evidence source: %s depends on %s", from.Key(), dep.Key())
		}
		if edge.Evidence.Excerpt == "" || edge.Evidence.Hash == "" {
			return fmt.Errorf("promoted edge lacks exact evidence: %s depends on %s", from.Key(), dep.Key())
		}
		graph[strings.ToLower(from.Key())] = append(graph[strings.ToLower(from.Key())], strings.ToLower(dep.Key()))
	}
	if cycle := findCycle(graph); len(cycle) > 0 {
		return fmt.Errorf("promoted adoption graph contains cycle: %s", strings.Join(cycle, " -> "))
	}
	return nil
}

// Observe reads only promoted edges from accepted outcome generations.
func Observe(records []outcome.Record, snapshot worksource.DependencySnapshot, subject worksource.Ref) convergence.Observation {
	obs := convergence.Observation{Subject: convergence.Subject{Repo: subject.Repo, Number: subject.Number}}
	subjectKey := strings.ToLower(subject.Key())
	if subjectKey == "" {
		return obs
	}
	states := make(map[string]string, len(snapshot.Issues))
	for _, issue := range snapshot.Issues {
		if key := worksource.RefFromIssue(issue).Key(); key != "" {
			states[strings.ToLower(key)] = issue.State
		}
	}
	byID := map[string]convergence.Dependency{}
	var recordIDs []string
	var generations []string
	for _, rec := range records {
		if rec.State != outcome.StateAccepted {
			continue
		}
		var spec Spec
		if err := json.Unmarshal([]byte(rec.Spec), &spec); err != nil || spec.Version != 1 {
			continue
		}
		for _, edge := range spec.Edges {
			if edge.State != EdgePromoted || strings.ToLower(strings.TrimSpace(edge.From)) != subjectKey {
				continue
			}
			depRef, ok := canonicalRef(edge.DependsOn)
			if !ok {
				continue
			}
			depKey := strings.ToLower(depRef.Key())
			status, detail := planningDependencyState(states, depKey)
			candidate := convergence.Dependency{ID: depKey, Status: status, Detail: detail}
			if prior, found := byID[depKey]; !found || severity(candidate.Status) > severity(prior.Status) {
				byID[depKey] = candidate
			}
			recordIDs = append(recordIDs, rec.Ref.Key())
			generations = append(generations, fmt.Sprintf("%s=%d", rec.Ref.Key(), rec.Generation))
		}
	}
	if len(byID) == 0 {
		return obs
	}
	obs.Found = true
	recordIDs = uniqueSorted(recordIDs)
	generations = uniqueSorted(generations)
	obs.RecordID = "planning:" + strings.Join(recordIDs, ",")
	obs.Generation = strings.Join(generations, ",")
	for _, dep := range byID {
		obs.Dependencies = append(obs.Dependencies, dep)
	}
	sort.Slice(obs.Dependencies, func(i, j int) bool { return obs.Dependencies[i].ID < obs.Dependencies[j].ID })
	return obs
}

func normalize(spec Spec) (Spec, error) {
	if spec.Version == 0 {
		spec.Version = 1
	}
	if spec.Version != 1 || strings.TrimSpace(spec.Project) == "" || len(spec.Repositories) == 0 {
		return Spec{}, fmt.Errorf("adoption spec requires version 1, project, and finite repository scope")
	}
	spec.Project = strings.TrimSpace(spec.Project)
	for i := range spec.Repositories {
		spec.Repositories[i] = strings.ToLower(strings.TrimSpace(spec.Repositories[i]))
	}
	spec.Repositories = uniqueSorted(spec.Repositories)
	for i := range spec.Roots {
		r, ok := canonicalRef(spec.Roots[i])
		if !ok {
			return Spec{}, fmt.Errorf("invalid roadmap root %q", spec.Roots[i])
		}
		spec.Roots[i] = strings.ToLower(r.Key())
	}
	spec.Roots = uniqueSorted(spec.Roots)
	for i := range spec.Edges {
		e := &spec.Edges[i]
		from, fromOK := canonicalRef(e.From)
		dep, depOK := canonicalRef(e.DependsOn)
		if !fromOK || !depOK {
			return Spec{}, fmt.Errorf("invalid adoption edge %q depends on %q", e.From, e.DependsOn)
		}
		e.From, e.DependsOn = strings.ToLower(from.Key()), strings.ToLower(dep.Key())
		if e.State == "" {
			e.State = EdgeProposed
		}
		if e.State != EdgeProposed && e.State != EdgePromoted && e.State != EdgeRejected {
			return Spec{}, fmt.Errorf("invalid adoption edge state %q", e.State)
		}
		e.Evidence.Source = strings.ToLower(strings.TrimSpace(e.Evidence.Source))
		e.Evidence.Excerpt = strings.TrimSpace(e.Evidence.Excerpt)
		if e.Evidence.Excerpt != "" {
			sum := sha256.Sum256([]byte(e.Evidence.Source + "\n" + e.Evidence.Excerpt))
			e.Evidence.Hash = "sha256:" + hex.EncodeToString(sum[:])
		}
	}
	sort.Slice(spec.Edges, func(i, j int) bool {
		if spec.Edges[i].From != spec.Edges[j].From {
			return spec.Edges[i].From < spec.Edges[j].From
		}
		return spec.Edges[i].DependsOn < spec.Edges[j].DependsOn
	})
	return spec, nil
}

func workRefs(spec Spec) []string {
	refs := append([]string(nil), spec.Roots...)
	for _, edge := range spec.Edges {
		refs = append(refs, edge.From, edge.DependsOn)
	}
	return uniqueSorted(refs)
}

func canonicalRef(raw string) (worksource.Ref, bool) {
	ref, ok := worksource.ParseKey(strings.ToLower(strings.TrimSpace(raw)))
	return ref, ok && ref.IsGitHubIssue() && strings.Contains(ref.Repo, "/")
}

func stringSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, v := range values {
		out[strings.ToLower(strings.TrimSpace(v))] = true
	}
	return out
}

func uniqueSorted(values []string) []string {
	set := stringSet(values)
	out := make([]string, 0, len(set))
	for v := range set {
		if v != "" {
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

func findCycle(graph map[string][]string) []string {
	state := map[string]uint8{}
	stack := []string{}
	positions := map[string]int{}
	var visit func(string) []string
	visit = func(node string) []string {
		if state[node] == 1 {
			return append(append([]string(nil), stack[positions[node]:]...), node)
		}
		if state[node] == 2 {
			return nil
		}
		state[node] = 1
		positions[node] = len(stack)
		stack = append(stack, node)
		for _, next := range graph[node] {
			if cycle := visit(next); len(cycle) > 0 {
				return cycle
			}
		}
		stack = stack[:len(stack)-1]
		delete(positions, node)
		state[node] = 2
		return nil
	}
	keys := make([]string, 0, len(graph))
	for key := range graph {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if cycle := visit(key); len(cycle) > 0 {
			return cycle
		}
	}
	return nil
}

func planningDependencyState(states map[string]string, key string) (convergence.ConditionStatus, string) {
	state, ok := states[key]
	if !ok {
		return convergence.ConditionUnknown, "approved planning dependency is missing or inaccessible in the source snapshot"
	}
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "closed", "done", "complete", "completed", "resolved", "cancelled", "canceled":
		return convergence.ConditionTrue, "approved planning dependency is closed"
	case "open", "todo", "backlog", "in progress", "in_progress":
		return convergence.ConditionFalse, "approved planning dependency is open"
	default:
		return convergence.ConditionUnknown, "approved planning dependency state is unknown"
	}
}

func severity(status convergence.ConditionStatus) int {
	if status == convergence.ConditionFalse {
		return 2
	}
	if status == convergence.ConditionUnknown {
		return 1
	}
	return 0
}

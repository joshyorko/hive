package adoption

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kubestellar/hive/pkg/worksource"
)

const defaultInventoryLimit = 128
const nonRootBodyLimit = 6000

var discoveredRefPattern = regexp.MustCompile(`(?i)https://github\.com/([a-z0-9_.-]+/[a-z0-9_.-]+)/issues/([1-9][0-9]*)|([a-z0-9_.-]+/[a-z0-9_.-]+|[a-z0-9_.-]+)#([1-9][0-9]*)|#([1-9][0-9]*)`)

type AuthorizationReport struct {
	Promoted     int      `json:"promoted"`
	LeftProposed int      `json:"left_proposed"`
	Reasons      []string `json:"reasons,omitempty"`
}

// Inventory performs bounded discovery from configured roadmap roots. It may
// discover loose references, but discovery confers no authority: only the
// architect output and later promotion validation can create an edge.
func Inventory(snapshot worksource.DependencySnapshot, roots []string, limit int) []worksource.Issue {
	if limit <= 0 {
		limit = defaultInventoryLimit
	}
	authority := stringSet(snapshot.Authority)
	byKey := make(map[string]worksource.Issue, len(snapshot.Issues))
	for _, issue := range snapshot.Issues {
		key := strings.ToLower(worksource.RefFromIssue(issue).Key())
		if key == "" || !authority[strings.ToLower(issue.Repo)] {
			continue
		}
		if prior, ok := byKey[key]; !ok || (prior.Body == "" && issue.Body != "") {
			byKey[key] = issue
		}
	}
	queue := make([]string, 0, len(roots))
	for _, raw := range roots {
		if ref, ok := canonicalRef(raw); ok && authority[strings.ToLower(ref.Repo)] {
			queue = append(queue, strings.ToLower(ref.Key()))
		}
	}
	seen := map[string]bool{}
	out := make([]worksource.Issue, 0)
	for len(queue) > 0 && len(out) < limit {
		key := queue[0]
		queue = queue[1:]
		if seen[key] {
			continue
		}
		seen[key] = true
		issue, ok := byKey[key]
		if !ok {
			continue
		}
		out = append(out, issue)
		for _, next := range discoverRefs(issue, authority) {
			if !seen[next] {
				queue = append(queue, next)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return worksource.RefFromIssue(out[i]).Key() < worksource.RefFromIssue(out[j]).Key()
	})
	return out
}

// BuildArchitectPrompt asks the existing architect role to reconcile existing
// work. The output is proposal-only JSON and cannot directly gate admission.
func BuildArchitectPrompt(seed Spec, snapshot worksource.DependencySnapshot, limit int) (string, error) {
	seed, err := normalize(seed)
	if err != nil {
		return "", err
	}
	seed.Edges = nil
	items := prioritizedInventory(snapshot, seed.Roots, limit)
	if len(items) == 0 {
		return "", fmt.Errorf("adoption architect inventory found no configured roadmap roots")
	}
	var b strings.Builder
	b.WriteString("[agent:architect]\n")
	b.WriteString("Reconcile this EXISTING multi-repository backlog into a proposed dependency graph over existing worksource.Ref identities.\n")
	b.WriteString("You must not create issues, beads, branches, or replacement tasks; output only JSON.\n")
	b.WriteString("Every edge state must be \"proposed\". Planning output is discovery, not authority, and cannot gate work until deterministic validation and operator-authorized promotion.\n")
	b.WriteString("Use only canonical refs present below and only repositories in the finite project scope. Preserve parallel lanes.\n")
	b.WriteString("Create hard edges only for explicit Depends on/Blocked by declarations, exact ordered roadmap statements, numbered sequencing constraints, explicit phase boundaries, or unambiguous producer-before-consumer statements.\n")
	b.WriteString("Use one classification exactly: explicit-hard-dependency, explicit-roadmap-order, numbered-roadmap-sequence, explicit-phase-boundary, explicit-producer-consumer-order, or ambiguous-prose.\n")
	b.WriteString("Related, Parent, loose mentions, importance, and ambiguous prose are not hard blockers. Keep them out or classify them ambiguous so they remain proposed.\n")
	b.WriteString("Every edge requires an exact evidence excerpt copied byte-for-byte from one listed issue body and its canonical evidence source ref.\n\n")
	b.WriteString("PROJECT SEED:\n")
	rawSeed, _ := json.MarshalIndent(seed, "", "  ")
	b.Write(rawSeed)
	b.WriteString("\n\n")
	b.WriteString("OUTPUT SHAPE: one JSON object matching the seed, with edges [{from,depends_on,state,classification,evidence:{source,excerpt}}]. No Markdown fence.\n\n")
	b.WriteString("BOUNDED EXISTING WORK INVENTORY:\n")
	rootSet := stringSet(seed.Roots)
	for _, issue := range items {
		key := worksource.RefFromIssue(issue).Key()
		b.WriteString("\n--- ")
		b.WriteString(key)
		b.WriteString(" | ")
		b.WriteString(issue.State)
		b.WriteString(" | ")
		b.WriteString(issue.Title)
		b.WriteString(" ---\n")
		body := issue.Body
		if !rootSet[strings.ToLower(key)] && len(body) > nonRootBodyLimit {
			body = body[:nonRootBodyLimit] + "\n[body truncated by bounded adoption inventory]"
		}
		b.WriteString(body)
		b.WriteString("\n")
	}
	return b.String(), nil
}

// prioritizedInventory guarantees the configured roots and enrolled factory
// population fit before spending the remaining bound on referenced context.
func prioritizedInventory(snapshot worksource.DependencySnapshot, roots []string, limit int) []worksource.Issue {
	if limit <= 0 {
		limit = defaultInventoryLimit
	}
	byKey := make(map[string]worksource.Issue, len(snapshot.Issues))
	for _, issue := range snapshot.Issues {
		if key := strings.ToLower(worksource.RefFromIssue(issue).Key()); key != "" {
			byKey[key] = issue
		}
	}
	selected := map[string]bool{}
	keys := []string{}
	add := func(key string) {
		key = strings.ToLower(key)
		if !selected[key] && byKey[key].Number > 0 && len(keys) < limit {
			selected[key] = true
			keys = append(keys, key)
		}
	}
	for _, raw := range roots {
		if ref, ok := canonicalRef(raw); ok {
			add(ref.Key())
		}
	}
	for _, issue := range snapshot.Issues {
		if enrolledForPlanning(issue.Labels, snapshot.EnrollmentLabels) {
			add(worksource.RefFromIssue(issue).Key())
		}
	}
	for cursor := 0; cursor < len(keys) && len(keys) < limit; cursor++ {
		issue := byKey[keys[cursor]]
		for _, ref := range discoverRefs(issue, stringSet(snapshot.Authority)) {
			add(ref)
		}
	}
	out := make([]worksource.Issue, 0, len(keys))
	for _, key := range keys {
		out = append(out, byKey[key])
	}
	return out
}

func enrolledForPlanning(labels, required []string) bool {
	if len(required) == 0 {
		return false
	}
	for _, label := range labels {
		for _, want := range required {
			if strings.EqualFold(strings.TrimSpace(label), strings.TrimSpace(want)) {
				return true
			}
		}
	}
	return false
}

// ParseArchitectOutput accepts only a proposal. An architect cannot mark its
// own inference promoted or accepted.
func ParseArchitectOutput(raw string) (Spec, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		lines := strings.Split(raw, "\n")
		if len(lines) < 3 || !strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
			return Spec{}, fmt.Errorf("unterminated architect JSON fence")
		}
		raw = strings.Join(lines[1:len(lines)-1], "\n")
	}
	var spec Spec
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&spec); err != nil {
		return Spec{}, fmt.Errorf("decode architect adoption proposal: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Spec{}, fmt.Errorf("decode architect adoption proposal: trailing JSON value")
	}
	normalized, err := normalize(spec)
	if err != nil {
		return Spec{}, err
	}
	for _, edge := range normalized.Edges {
		if edge.State != EdgeProposed && edge.State != EdgeRejected {
			return Spec{}, fmt.Errorf("architect output attempted authoritative edge state %q", edge.State)
		}
	}
	return normalized, nil
}

var promotableClassifications = map[string]bool{
	"explicit-hard-dependency":  true,
	"explicit-roadmap-order":    true,
	"numbered-roadmap-sequence": true,
	"explicit-phase-boundary":   true,
}

// AuthorizeExplicitEdges promotes only proposal edges whose evidence is an
// exact excerpt from an existing in-scope source and whose classification is
// explicitly eligible. The resulting promoted subgraph then passes the same
// missing-ref/self/cycle validator used at ledger acceptance.
func AuthorizeExplicitEdges(spec Spec, snapshot worksource.DependencySnapshot) (Spec, AuthorizationReport, error) {
	normalized, err := normalize(spec)
	if err != nil {
		return Spec{}, AuthorizationReport{}, err
	}
	bodyByKey := make(map[string]string, len(snapshot.Issues))
	existing := make(map[string]bool, len(snapshot.Issues))
	for _, issue := range snapshot.Issues {
		if key := strings.ToLower(worksource.RefFromIssue(issue).Key()); key != "" {
			bodyByKey[key] = issue.Body
			existing[key] = true
		}
	}
	authority := stringSet(normalized.Repositories)
	report := AuthorizationReport{}
	for i := range normalized.Edges {
		edge := &normalized.Edges[i]
		if edge.State == EdgeRejected {
			continue
		}
		if !promotableClassifications[strings.ToLower(strings.TrimSpace(edge.Classification))] {
			edge.State = EdgeProposed
			report.LeftProposed++
			report.Reasons = append(report.Reasons, edge.From+"<-"+edge.DependsOn+": classification is not promotion-eligible")
			continue
		}
		body, ok := bodyByKey[strings.ToLower(edge.Evidence.Source)]
		if !ok || edge.Evidence.Excerpt == "" || !strings.Contains(body, edge.Evidence.Excerpt) {
			edge.State = EdgeProposed
			report.LeftProposed++
			report.Reasons = append(report.Reasons, edge.From+"<-"+edge.DependsOn+": evidence is not an exact source excerpt")
			continue
		}
		edge.State = EdgePromoted
		report.Promoted++
	}
	leaveProposed := func(edge *Edge, reason string) {
		if edge.State != EdgePromoted {
			return
		}
		edge.State = EdgeProposed
		report.Promoted--
		report.LeftProposed++
		report.Reasons = append(report.Reasons, edge.From+"<-"+edge.DependsOn+": "+reason)
	}
	for i := range normalized.Edges {
		edge := &normalized.Edges[i]
		if edge.State != EdgePromoted {
			continue
		}
		from, _ := canonicalRef(edge.From)
		dep, _ := canonicalRef(edge.DependsOn)
		switch {
		case !authority[strings.ToLower(from.Repo)] || !authority[strings.ToLower(dep.Repo)]:
			leaveProposed(edge, "edge escapes project repository scope")
		case from.Key() == dep.Key():
			leaveProposed(edge, "self-dependency is not promotable")
		case !existing[strings.ToLower(from.Key())] || !existing[strings.ToLower(dep.Key())]:
			leaveProposed(edge, "referenced work is missing from the bounded source snapshot")
		}
	}
	for {
		graph := make(map[string][]string)
		for _, edge := range normalized.Edges {
			if edge.State == EdgePromoted {
				graph[edge.From] = append(graph[edge.From], edge.DependsOn)
			}
		}
		cycle := findCycle(graph)
		if len(cycle) == 0 {
			break
		}
		cycleNodes := stringSet(cycle)
		for i := range normalized.Edges {
			edge := &normalized.Edges[i]
			if edge.State == EdgePromoted && cycleNodes[edge.From] && cycleNodes[edge.DependsOn] {
				leaveProposed(edge, "edge participates in an unresolved cycle")
			}
		}
	}
	if err := ValidatePromotion(normalized, snapshot); err != nil {
		return Spec{}, report, err
	}
	return normalized, report, nil
}

func discoverRefs(issue worksource.Issue, authority map[string]bool) []string {
	refs := map[string]bool{}
	for _, match := range discoveredRefPattern.FindAllStringSubmatch(issue.Body, -1) {
		repo := ""
		number := 0
		switch {
		case match[1] != "":
			repo = strings.ToLower(match[1])
			number, _ = strconv.Atoi(match[2])
		case match[3] != "":
			repo = strings.ToLower(match[3])
			number, _ = strconv.Atoi(match[4])
		default:
			repo = strings.ToLower(issue.Repo)
			number, _ = strconv.Atoi(match[5])
		}
		if !strings.Contains(repo, "/") {
			for configured := range authority {
				if strings.HasSuffix(configured, "/"+repo) {
					repo = configured
					break
				}
			}
		}
		key := strings.ToLower((worksource.Ref{Repo: repo, Number: number}).Key())
		if authority[repo] && key != "" {
			refs[key] = true
		}
	}
	out := make([]string, 0, len(refs))
	for ref := range refs {
		out = append(out, ref)
	}
	sort.Strings(out)
	return out
}

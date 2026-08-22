package worksource

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kubestellar/hive/pkg/convergence"
)

var (
	dependencyLinePattern = regexp.MustCompile(`(?i)^\s*(?:[-*+]\s*)?(?:\*\*|__)?(depends on|blocked by)\s*:(?:\*\*|__)?\s*(.*?)\s*$`)
	shortIssueRefPattern  = regexp.MustCompile(`^#([1-9][0-9]*)$`)
	fullIssueRefPattern   = regexp.MustCompile(`^([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)#([1-9][0-9]*)$`)
	issueURLPattern       = regexp.MustCompile(`(?i)^https://github\.com/([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)/issues/([1-9][0-9]*)(?:[?#].*)?$`)
	markdownLinkPattern   = regexp.MustCompile(`^\[[^]]+\]\(([^)]+)\)$`)
)

type parsedDependency struct {
	repo          string
	number        int
	id            string
	valid         bool
	malformed     bool
	malformedText string
}

type parsedSourceRecord struct {
	issue        Issue
	key          string
	dependencies []parsedDependency
	found        bool
}

// ObserveDependencies normalises explicitly enrolled source declarations into
// the source-neutral convergence observation. Only line-anchored "Depends
// on:" and "Blocked by:" declarations are interpreted. Parent, Related,
// Progresses, prose, and arbitrary links are deliberately ignored.
//
// A source record is opt-in through the configured enrollment label. An explicit but
// malformed declaration is still observed and produces Unknown dependencies;
// it can never become silently satisfied. Missing, unreadable, and
// out-of-authority targets are likewise Unknown. The snapshot is read-only and
// the function holds no process-local state, so callers can rebuild it on every
// level-triggered sweep and after restart.
func ObserveDependencies(snapshot DependencySnapshot, subject Ref) convergence.Observation {
	authority := dependencyAuthority(snapshot, subject.Repo)
	allRecords := make(map[string]parsedSourceRecord, len(snapshot.Issues))
	records := make(map[string]parsedSourceRecord, len(snapshot.Issues))
	for _, issue := range snapshot.Issues {
		key := sourceIssueKey(issue, authority)
		if key == "" {
			continue
		}
		if _, exists := allRecords[key]; !exists {
			allRecords[key] = parsedSourceRecord{issue: issue, key: key}
		}
		if _, exists := records[key]; exists {
			continue
		}
		if !enrolled(issue.Labels, snapshot.EnrollmentLabels) {
			continue
		}
		deps, found := parseDependencies(issue.Body)
		for i := range deps {
			if !deps[i].valid {
				continue
			}
			repo := deps[i].repo
			if repo == "" {
				repo = strings.SplitN(key, "#", 2)[0]
			}
			deps[i].repo = canonicalRepoFromSet(repo, authority)
			deps[i].id = canonicalIssueKey(deps[i].repo, deps[i].number, nil)
		}
		records[key] = parsedSourceRecord{issue: issue, key: key, dependencies: deps, found: found}
	}

	subjectKey := canonicalIssueKey(subject.Repo, subject.Number, authority)
	record, found := records[subjectKey]
	obs := convergence.Observation{Subject: convergence.Subject{Repo: subject.Repo, Number: subject.Number}}
	if !found || !record.found {
		return obs
	}
	obs.Found = true
	obs.RecordID = "source:" + subjectKey
	obs.Generation = generation(record.issue.UpdatedAt)
	graph := dependencyGraph(records, authority)
	seen := make(map[string]struct{}, len(record.dependencies))
	for _, dep := range record.dependencies {
		if _, duplicate := seen[dep.id]; duplicate {
			continue
		}
		seen[dep.id] = struct{}{}
		if dep.malformed {
			obs.Dependencies = append(obs.Dependencies, convergence.Dependency{
				ID: dep.id, Status: convergence.ConditionUnknown,
				Detail: "malformed explicit dependency target",
			})
			continue
		}
		if !dep.valid {
			continue
		}
		if !authority[dep.repo] {
			obs.Dependencies = append(obs.Dependencies, convergence.Dependency{
				ID: dep.id, Status: convergence.ConditionUnknown,
				Detail: "dependency target is outside configured repository authority",
			})
			continue
		}
		target, exists := allRecords[dep.id]
		if !exists {
			obs.Dependencies = append(obs.Dependencies, convergence.Dependency{
				ID: dep.id, Status: convergence.ConditionUnknown,
				Detail: "dependency target is missing or inaccessible in the source snapshot",
			})
			continue
		}
		status, detail := sourceState(target.issue.State)
		if dep.id == subjectKey {
			status = convergence.ConditionFalse
			detail = "self-dependency detected"
		} else if reaches(graph, dep.id, subjectKey) {
			status = convergence.ConditionFalse
			detail = "dependency cycle detected"
		}
		obs.Dependencies = append(obs.Dependencies, convergence.Dependency{ID: dep.id, Status: status, Detail: detail})
	}
	sort.Slice(obs.Dependencies, func(i, j int) bool { return obs.Dependencies[i].ID < obs.Dependencies[j].ID })
	return obs
}

func dependencyAuthority(snapshot DependencySnapshot, subjectRepo string) map[string]bool {
	authority := make(map[string]bool, len(snapshot.Authority))
	for _, repo := range snapshot.Authority {
		if canonical := canonicalRepo(repo, snapshot.Authority); canonical != "" {
			authority[canonical] = true
		}
	}
	if len(authority) == 0 {
		if canonical := canonicalRepo(subjectRepo, nil); canonical != "" {
			authority[canonical] = true
		}
	}
	return authority
}

func sourceIssueKey(issue Issue, authority map[string]bool) string {
	if issue.Number <= 0 {
		return ""
	}
	repo := canonicalRepoFromSet(issue.Repo, authority)
	if repo == "" {
		return ""
	}
	return canonicalIssueKey(repo, issue.Number, nil)
}

func canonicalIssueKey(repo string, number int, authority map[string]bool) string {
	if number <= 0 {
		return ""
	}
	if authority != nil {
		repo = canonicalRepoFromSet(repo, authority)
	} else {
		repo = strings.ToLower(strings.TrimSpace(repo))
	}
	if repo == "" {
		return ""
	}
	return fmt.Sprintf("%s#%d", repo, number)
}

func canonicalRepoFromSet(raw string, authority map[string]bool) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	if authority == nil || strings.Contains(raw, "/") {
		return raw
	}
	for repo := range authority {
		if repo == raw || strings.TrimPrefix(repo, strings.SplitN(repo, "/", 2)[0]+"/") == raw {
			return repo
		}
	}
	return raw
}

func canonicalRepo(raw string, configured []string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "/") {
		return raw
	}
	for _, configuredRepo := range configured {
		configuredRepo = strings.ToLower(strings.TrimSpace(configuredRepo))
		if strings.TrimPrefix(configuredRepo, strings.SplitN(configuredRepo, "/", 2)[0]+"/") == raw {
			return configuredRepo
		}
	}
	return raw
}

func dependencyGraph(records map[string]parsedSourceRecord, authority map[string]bool) map[string][]string {
	graph := make(map[string][]string, len(records))
	for key, record := range records {
		for _, dep := range record.dependencies {
			if dep.valid && authority[dep.repo] {
				if _, exists := records[dep.id]; exists {
					graph[key] = append(graph[key], dep.id)
				}
			}
		}
	}
	return graph
}

func reaches(graph map[string][]string, start, goal string) bool {
	stack := []string{start}
	visited := map[string]bool{}
	for len(stack) > 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]
		if node == goal {
			return true
		}
		if visited[node] {
			continue
		}
		visited[node] = true
		stack = append(stack, graph[node]...)
	}
	return false
}

func sourceState(state string) (convergence.ConditionStatus, string) {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "closed", "done", "complete", "completed", "resolved", "cancelled", "canceled":
		return convergence.ConditionTrue, "dependency target is closed"
	case "open", "todo", "backlog", "in progress", "in_progress":
		return convergence.ConditionFalse, "dependency target is open"
	default:
		return convergence.ConditionUnknown, "dependency target state is missing or unrecognised"
	}
}

func generation(updatedAt time.Time) string {
	if updatedAt.IsZero() {
		return ""
	}
	return updatedAt.UTC().Format(time.RFC3339Nano)
}

func enrolled(labels, required []string) bool {
	if len(required) == 0 {
		required = []string{"hive-ready"}
	}
	for _, label := range labels {
		for _, enrollment := range required {
			if strings.EqualFold(strings.TrimSpace(label), strings.TrimSpace(enrollment)) {
				return true
			}
		}
	}
	return false
}

func parseDependencies(body string) ([]parsedDependency, bool) {
	var deps []parsedDependency
	found := false
	for _, line := range strings.Split(body, "\n") {
		match := dependencyLinePattern.FindStringSubmatch(strings.TrimSuffix(line, "\r"))
		if match == nil {
			continue
		}
		found = true
		value := strings.TrimSpace(match[2])
		if value == "" {
			deps = append(deps, malformedDependency("empty target"))
			continue
		}
		for _, raw := range strings.Split(value, ",") {
			raw = strings.Trim(strings.TrimSpace(raw), "`*_ ")
			if link := markdownLinkPattern.FindStringSubmatch(raw); link != nil {
				raw = strings.TrimSpace(link[1])
			}
			if short := shortIssueRefPattern.FindStringSubmatch(raw); short != nil {
				n, _ := strconv.Atoi(short[1])
				deps = append(deps, parsedDependency{number: n, valid: true})
				continue
			}
			if full := fullIssueRefPattern.FindStringSubmatch(raw); full != nil {
				n, _ := strconv.Atoi(full[2])
				deps = append(deps, parsedDependency{repo: strings.ToLower(full[1]), number: n, valid: true})
				continue
			}
			if issueURL := issueURLPattern.FindStringSubmatch(raw); issueURL != nil {
				n, _ := strconv.Atoi(issueURL[2])
				deps = append(deps, parsedDependency{repo: strings.ToLower(issueURL[1]), number: n, valid: true})
				continue
			}
			deps = append(deps, malformedDependency(raw))
		}
	}
	return deps, found
}

func malformedDependency(raw string) parsedDependency {
	if raw == "" {
		raw = "empty target"
	}
	if len(raw) > 128 {
		raw = raw[:128]
	}
	return parsedDependency{id: "malformed:" + raw, malformed: true}
}

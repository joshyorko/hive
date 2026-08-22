package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	ghpkg "github.com/kubestellar/hive/pkg/github"
)

type acmmIssueFixture struct {
	Number      int
	Title       string
	Body        string
	State       string
	StateReason string
}

type acmmMutationFixture struct {
	mu               sync.Mutex
	issues           []acmmIssueFixture
	created          int
	edits            []map[string]any
	comments         []map[string]any
	paths            map[string]bool
	freshEvaluations int
}

func (f *acmmMutationFixture) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/myorg/repo1", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, map[string]any{"default_branch": "main"})
	})
	mux.HandleFunc("/repos/myorg/repo1/commits/HEAD", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.freshEvaluations++
		f.mu.Unlock()
		jsonResponse(w, map[string]any{"sha": "fresh-sha"})
	})
	mux.HandleFunc("/repos/myorg/repo1/labels", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, map[string]any{"name": "acmm"})
	})
	mux.HandleFunc("/repos/myorg/repo1/contents/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/repos/myorg/repo1/contents/")
		var entries []map[string]any
		for candidate := range f.paths {
			parent := ""
			name := candidate
			if idx := strings.LastIndex(candidate, "/"); idx >= 0 {
				parent, name = candidate[:idx], candidate[idx+1:]
			}
			if parent == path {
				entries = append(entries, map[string]any{"name": name, "path": candidate, "type": "file", "sha": "blob"})
			}
		}
		if entries != nil {
			jsonResponse(w, entries)
			return
		}
		if f.paths[path] {
			jsonResponse(w, map[string]any{"name": path, "path": path, "type": "file", "sha": "blob"})
			return
		}
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	mux.HandleFunc("/repos/myorg/repo1/issues", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			var out []map[string]any
			for _, issue := range f.issues {
				out = append(out, map[string]any{
					"id": issue.Number, "number": issue.Number, "title": issue.Title, "body": issue.Body,
					"state": issue.State, "state_reason": issue.StateReason,
					"html_url": fmt.Sprintf("https://github.com/myorg/repo1/issues/%d", issue.Number),
				})
			}
			jsonResponse(w, out)
		case http.MethodPost:
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			f.created++
			n := 100 + f.created
			f.issues = append(f.issues, acmmIssueFixture{Number: n, Title: req["title"].(string), Body: req["body"].(string), State: "open"})
			jsonResponse(w, map[string]any{"number": n, "html_url": fmt.Sprintf("https://github.com/myorg/repo1/issues/%d", n)})
		}
	})
	mux.HandleFunc("/repos/myorg/repo1/issues/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if strings.HasSuffix(r.URL.Path, "/comments") {
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			f.comments = append(f.comments, req)
			jsonResponse(w, map[string]any{"id": len(f.comments)})
			return
		}
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.edits = append(f.edits, req)
		var number int
		_, _ = fmt.Sscanf(strings.TrimPrefix(r.URL.Path, "/repos/myorg/repo1/issues/"), "%d", &number)
		for i := range f.issues {
			if f.issues[i].Number == number {
				f.issues[i].State = fmt.Sprint(req["state"])
				f.issues[i].StateReason = fmt.Sprint(req["state_reason"])
			}
		}
		jsonResponse(w, map[string]any{"number": number, "state": req["state"], "state_reason": req["state_reason"]})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func TestAutomaticACMMReconciliationIsLevelTriggeredAndCadenceBounded(t *testing.T) {
	f := &acmmMutationFixture{
		paths:  map[string]bool{"CLAUDE.md": true},
		issues: []acmmIssueFixture{{Number: 40, Title: "satisfied", Body: legacyACMMBody("acmm:claude-md"), State: "open"}},
	}
	s := acmmTestServer(t, f)
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)

	ran, result := s.ReconcileACMMAutomatically(context.Background(), now)
	if !ran || len(result.Repos) != 1 || len(f.edits) != 1 {
		t.Fatalf("first run=(ran=%v repos=%d edits=%d), want one applied reconciliation", ran, len(result.Repos), len(f.edits))
	}
	f.mu.Lock()
	firstEvaluations := f.freshEvaluations
	f.mu.Unlock()

	ran, _ = s.ReconcileACMMAutomatically(context.Background(), now.Add(acmmAutomaticReconcileInterval-time.Second))
	if ran {
		t.Fatal("reconciliation ran again before its cadence elapsed")
	}
	f.mu.Lock()
	if f.freshEvaluations != firstEvaluations {
		t.Fatalf("cadence skip performed fresh evaluation: got %d want %d", f.freshEvaluations, firstEvaluations)
	}
	f.mu.Unlock()

	ran, _ = s.ReconcileACMMAutomatically(context.Background(), now.Add(acmmAutomaticReconcileInterval))
	if !ran {
		t.Fatal("reconciliation did not run when cadence elapsed")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.freshEvaluations <= firstEvaluations {
		t.Fatalf("due run did not reacquire fresh evidence: got %d, first run %d", f.freshEvaluations, firstEvaluations)
	}
	if len(f.edits) != 1 {
		t.Fatalf("idempotent due run added mutations: edits=%d want 1", len(f.edits))
	}
}

func acmmTestServer(t *testing.T, f *acmmMutationFixture) *Server {
	t.Helper()
	ts := f.server(t)
	s := NewServer(0, acmmEvalTestLogger())
	deps := testDeps(t)
	deps.GHClient = ghpkg.NewClientForTest(ts.URL, "myorg", []string{"repo1"}, acmmEvalTestLogger())
	s.RegisterAPI(deps)
	return s
}

func legacyACMMBody(id string) string {
	return "## ACMM Gap\n\n**Criterion ID:** `" + id + "`\n\n*Opened by Hive ACMM Evaluation*"
}

func TestACMMCreateIssueIsIdempotentByCriterionIdentity(t *testing.T) {
	f := &acmmMutationFixture{paths: map[string]bool{}}
	s := acmmTestServer(t, f)
	for range 2 {
		rec := doPost(s, "/api/acmm/issue", map[string]any{"repo": "repo1", "criterion_id": "acmm:claude-md"})
		if rec.Code != http.StatusOK {
			t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
		}
	}
	if f.created != 1 {
		t.Fatalf("created %d issues, want exactly 1", f.created)
	}
}

func TestACMMCreateIssueRejectsStaleCachedFailure(t *testing.T) {
	f := &acmmMutationFixture{paths: map[string]bool{"CLAUDE.md": true}}
	s := acmmTestServer(t, f)
	s.acmmEvalCache = &ACMMEvaluation{RepoResults: []RepoEvaluation{{Repo: "repo1", CriteriaResults: []CriterionResult{{ID: "acmm:claude-md", Passed: false}}}}}
	rec := doPost(s, "/api/acmm/issue", map[string]any{"repo": "repo1", "criterion_id": "acmm:claude-md"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d want 409; body=%s", rec.Code, rec.Body.String())
	}
	if f.created != 0 {
		t.Fatalf("stale cache created %d issues", f.created)
	}
}

func TestACMMCreateIssueRespectsHumanNotPlannedDisposition(t *testing.T) {
	f := &acmmMutationFixture{paths: map[string]bool{}, issues: []acmmIssueFixture{
		{Number: 9, Title: "declined", Body: legacyACMMBody("acmm:claude-md"), State: "closed", StateReason: "not_planned"},
	}}
	s := acmmTestServer(t, f)
	rec := doPost(s, "/api/acmm/issue", map[string]any{"repo": "repo1", "criterion_id": "acmm:claude-md"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d want 409; body=%s", rec.Code, rec.Body.String())
	}
	if f.created != 0 {
		t.Fatalf("human disposition was recreated %d times", f.created)
	}
}

func TestACMMReconcileDuplicateAndSatisfiedIsIdempotent(t *testing.T) {
	f := &acmmMutationFixture{
		paths: map[string]bool{"CLAUDE.md": true},
		issues: []acmmIssueFixture{
			{Number: 10, Title: "[ACMM L2] Add CLAUDE.md instruction file", Body: legacyACMMBody("acmm:claude-md"), State: "open"},
			{Number: 11, Title: "same wording", Body: legacyACMMBody("acmm:claude-md"), State: "open"},
			{Number: 12, Title: "similar but unrelated", Body: "no machine identity", State: "open"},
		},
	}
	s := acmmTestServer(t, f)
	for i := 0; i < 2; i++ {
		rec := doPost(s, "/api/acmm/reconcile", map[string]any{"repos": []string{"repo1"}, "dry_run": false})
		if rec.Code != http.StatusOK {
			t.Fatalf("reconcile status=%d body=%s", rec.Code, rec.Body.String())
		}
		if i == 0 && (!strings.Contains(rec.Body.String(), `"disposition":"satisfied"`) || !strings.Contains(rec.Body.String(), `"disposition":"duplicate"`)) {
			t.Fatalf("missing satisfied/duplicate dispositions: %s", rec.Body.String())
		}
	}
	if len(f.edits) != 2 {
		t.Fatalf("edits=%d want 2 after idempotent double run", len(f.edits))
	}
	if len(f.comments) != 2 {
		t.Fatalf("comments=%d want 2", len(f.comments))
	}
	var duplicateLinked bool
	for _, edit := range f.edits {
		if edit["state_reason"] == "duplicate" && int(edit["duplicate_issue_id"].(float64)) == 10 {
			duplicateLinked = true
		}
	}
	if !duplicateLinked {
		t.Fatalf("duplicate close did not link canonical issue database ID: %+v", f.edits)
	}
}

func TestACMMReconcileStillFailingAndHumanNotPlannedRemainUntouched(t *testing.T) {
	f := &acmmMutationFixture{
		paths: map[string]bool{},
		issues: []acmmIssueFixture{
			{Number: 20, Title: "active", Body: legacyACMMBody("acmm:claude-md"), State: "open"},
			{Number: 21, Title: "declined", Body: legacyACMMBody("acmm:cursor-rules"), State: "closed", StateReason: "not_planned"},
		},
	}
	s := acmmTestServer(t, f)
	rec := doPost(s, "/api/acmm/reconcile", map[string]any{"repos": []string{"repo1"}, "dry_run": false})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"disposition":"evaluator_gap"`, `"disposition":"human_dispositioned"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("missing %s in %s", want, rec.Body.String())
		}
	}
	if len(f.edits) != 0 || len(f.comments) != 0 {
		t.Fatalf("failing/human-dispositioned issues mutated: edits=%d comments=%d", len(f.edits), len(f.comments))
	}
}

func TestACMMReconcileStillFailingGenericCriterionAndDryRunDoNotMutate(t *testing.T) {
	f := &acmmMutationFixture{paths: map[string]bool{}, issues: []acmmIssueFixture{
		{Number: 30, Title: "e2e gap", Body: legacyACMMBody("acmm:prereq-e2e"), State: "open"},
		{Number: 31, Title: "e2e duplicate", Body: legacyACMMBody("acmm:prereq-e2e"), State: "open"},
	}}
	s := acmmTestServer(t, f)
	rec := doPost(s, "/api/acmm/reconcile", map[string]any{"repos": []string{"repo1"}, "dry_run": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"disposition":"still_failing"`, `"disposition":"duplicate"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("missing %s in %s", want, rec.Body.String())
		}
	}
	if len(f.edits) != 0 || len(f.comments) != 0 {
		t.Fatalf("dry-run mutated issues: edits=%d comments=%d", len(f.edits), len(f.comments))
	}
}

func TestACMMIssueMarkerRoundTrip(t *testing.T) {
	body := buildACMMIssueBody("myorg/repo1", universalCriteria[0])
	repo, criterion, ok := parseACMMIssueIdentity(body)
	if !ok || repo != "myorg/repo1" || criterion != universalCriteria[0].ID {
		t.Fatalf("identity=(%q,%q,%v)", repo, criterion, ok)
	}
	if strings.Contains(body, "content can follow") || strings.Contains(body, "Create one of the files") {
		t.Fatalf("body still encourages placeholder scaffolding: %s", body)
	}
}

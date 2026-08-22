package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kubestellar/hive/pkg/config"
	"github.com/kubestellar/hive/pkg/continuity"
	ghpkg "github.com/kubestellar/hive/pkg/github"
)

func continuationRecord() continuity.Record {
	return continuity.Record{
		Ref: continuity.PRRef{Repo: "acme/widgets", Number: 17}, Active: true,
		OriginalAuthor: "alice", HeadRepo: "acme/widgets", HeadBranch: "alice/existing",
		BaseBranch: "main", ObservedHeadSHA: "head-1", CurrentHeadSHA: "head-1",
		State: continuity.StateContinue, WriteCapability: continuity.CapabilityWritable,
	}
}

func TestVerifiedContinuityCompletionPersistsDeliveryReceipt(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/widgets/pulls/17", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"number":17,"user":{"login":"alice"},"head":{"ref":"alice/existing","sha":"head-2","repo":{"full_name":"acme/widgets"}},"base":{"ref":"main","repo":{"full_name":"acme/widgets"}}}`)
	})
	mux.HandleFunc("/repos/acme/widgets/compare/head-1...head-2", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"status":"ahead","ahead_by":1,"commits":[{"sha":"head-2","author":{"login":"hive-bot"}}]}`)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ledger, _ := continuity.OpenLedger(filepath.Join(t.TempDir(), "continuity.json"))
	before := continuity.Observation{
		Ref: continuity.PRRef{Repo: "acme/widgets", Number: 17}, OriginalAuthor: "alice",
		HeadRepo: "acme/widgets", HeadBranch: "alice/existing", BaseBranch: "main", HeadSHA: "head-1",
		State: continuity.StateContinue, WriteCapability: continuity.CapabilityWritable,
		LinkedWork: []continuity.WorkRelationship{{WorkRef: "acme/widgets#9", Relationship: continuity.RelationshipCloses, OwnedSlice: "runtime"}},
		Provenance: "github:head-1",
	}
	_, _ = ledger.Adopt(before, "owner", "session", time.Now())
	after := before
	after.HeadSHA = "head-2"
	after.State = continuity.StateReady
	after.Provenance = "github:head-2"
	deps := &Dependencies{
		Ctx: context.Background(), Logger: logger, ContinuityLedger: ledger,
		GHClient:            ghpkg.NewClientForTest(ts.URL, "acme", []string{"widgets"}, logger),
		ObserveContinuityPR: func(context.Context, continuity.PRRef) (continuity.Observation, error) { return after, nil },
	}
	s := NewServer(0, logger)
	s.RegisterAPI(deps)
	hub := NewContributeWSHub(logger, s)
	task := &WSTaskAssign{Repo: "acme/widgets", ContinuityPR: 17, OriginalPRAuthor: "alice", ExistingHeadRepo: "acme/widgets", ExistingHeadBranch: "alice/existing", BaseBranch: "main", ExpectedHeadSHA: "head-1"}

	verified := hub.verifyTaskDelivery(task, "https://github.com/acme/widgets/pull/17", "hive-bot")
	if !verified.Verified {
		t.Fatalf("delivery verification = %+v", verified)
	}
	rec, _ := ledger.Get(before.Ref)
	if rec.ObservedHeadSHA != "head-2" || rec.State != continuity.StateReady || rec.History[len(rec.History)-1].Verb != "delivery" {
		t.Fatalf("durable delivery receipt missing: %+v", rec)
	}
}

func TestBuildReposProjectsAdoptedPRIntoExistingContributorQueue(t *testing.T) {
	cfg := &config.Config{}
	cfg.Project.Org = "acme"
	cfg.Project.Repos = []string{"widgets"}
	actionable := &ghpkg.ActionableResult{Continuations: ghpkg.BuildContinuityResult([]continuity.Record{continuationRecord()})}
	repos := buildRepos(cfg, actionable)
	if len(repos) != 1 || len(repos[0].ActionableIssues) != 1 {
		t.Fatalf("repos = %+v", repos)
	}
	raw, _ := json.Marshal(repos[0].ActionableIssues[0])
	var item map[string]any
	_ = json.Unmarshal(raw, &item)
	if item["source_type"] != "github_pull_request" || item["external_id"] != "pr-17" || item["existing_head_branch"] != "alice/existing" || item["expected_head_sha"] != "head-1" {
		t.Fatalf("continuity envelope = %s", raw)
	}
}

func TestContinuityPromptRequiresExistingBranchWithoutHistoryRewrite(t *testing.T) {
	prompt := buildContinuityTaskPrompt(continuationRecord())
	for _, required := range []string{"existing PR #17", "alice/existing", "head-1", "git fetch", "checkout", "same existing branch", "do not force push", "do not create a replacement PR"} {
		if !strings.Contains(strings.ToLower(prompt), strings.ToLower(required)) {
			t.Errorf("prompt missing %q: %s", required, prompt)
		}
	}
	if strings.Contains(prompt, "gh repo fork") || strings.Contains(prompt, "mark it ready for review") {
		t.Fatalf("continuity prompt reused greenfield/draft-changing semantics: %s", prompt)
	}
}

func TestContinuityAssignmentCarriesExactHeadBinding(t *testing.T) {
	task := WSTaskAssign{TaskID: "ct-acme/widgets-pr-17", Kind: "pull_request_continuation",
		Repo: "acme/widgets", Key: "acme/widgets!pr-17", SourceType: "github_pull_request", ExternalID: "pr-17",
		ContinuityPR: 17, ExistingHeadRepo: "acme/widgets", ExistingHeadBranch: "alice/existing", ExpectedHeadSHA: "head-1", BaseBranch: "main", OriginalPRAuthor: "alice"}
	if task.identityKey() != "acme/widgets!pr-17" || task.ExistingHeadBranch != "alice/existing" || task.ExpectedHeadSHA != "head-1" {
		t.Fatalf("task = %+v", task)
	}
}

func TestContinuityReadyQueueAndSelectionUseSameExistingBranchEnvelope(t *testing.T) {
	hub, s := covK2Hub(t)
	s.deps.ValidateContinuityHead = func(context.Context, continuity.PRRef, string, string) error { return nil }
	issue := ghpkg.ContinuityIssue(continuationRecord())
	s.statusMu.Lock()
	s.status = &StatusPayload{Repos: []FrontendRepo{{Name: "widgets", Full: "acme/widgets", ActionableIssues: []any{issue}}}}
	s.statusMu.Unlock()

	queue := hub.ReadyQueue(10)
	if len(queue) != 1 || queue[0].SourceType != "github_pull_request" || queue[0].ExternalID != "pr-17" {
		t.Fatalf("ready queue = %+v", queue)
	}
	conn := &ContributorConnection{profile: &ContributorProfile{GitHubUsername: "worker", ContributorID: "c-worker", TrustTier: "contributor"}, lastPong: time.Now()}
	msg := hub.selectTask(conn)
	if msg == nil || msg.Type != "task_assign" || msg.Kind != "pull_request_continuation" {
		t.Fatalf("assignment = %+v", msg)
	}
	if msg.TaskKey != "acme/widgets!pr-17" || msg.ContinuityPR != 17 || msg.ExistingHeadBranch != "alice/existing" || msg.ExpectedHeadSHA != "head-1" {
		t.Fatalf("assignment lost continuity binding: %+v", msg)
	}
	if !strings.Contains(msg.Prompt, "Do not create a replacement PR") {
		t.Fatalf("assignment prompt = %s", msg.Prompt)
	}
}

func TestContinuityUnexpectedHeadMovementWithholdsAssignment(t *testing.T) {
	hub, s := covK2Hub(t)
	issue := ghpkg.ContinuityIssue(continuationRecord())
	s.statusMu.Lock()
	s.status = &StatusPayload{Repos: []FrontendRepo{{Name: "widgets", Full: "acme/widgets", ActionableIssues: []any{issue}}}}
	s.statusMu.Unlock()
	s.deps.ValidateContinuityHead = func(_ context.Context, ref continuity.PRRef, expectedHead, expectedBranch string) error {
		return fmt.Errorf("head moved from %s", expectedHead)
	}
	conn := &ContributorConnection{profile: &ContributorProfile{GitHubUsername: "worker", ContributorID: "c-worker", TrustTier: "contributor"}, lastPong: time.Now()}
	msg := hub.selectTask(conn)
	if msg == nil || msg.Type != "task_unavailable" || msg.Reason != taskUnavailableContinuityChanged {
		t.Fatalf("moved head must fail closed: %+v", msg)
	}
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if conn.currentTask != nil || conn.pendingToken != "" {
		t.Fatalf("moved head received assignment/credential: %+v", conn.currentTask)
	}
}

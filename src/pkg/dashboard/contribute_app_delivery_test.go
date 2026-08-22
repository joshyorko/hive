package dashboard

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ghpkg "github.com/kubestellar/hive/pkg/github"
	"github.com/kubestellar/hive/pkg/worksource"
)

func TestAppManagedContributorPromptUsesSameRepositoryDelivery(t *testing.T) {
	ref := worksource.Ref{Repo: "acme/widget", Number: 42}
	prompt := buildTaskPromptForContributor(ref, "repair the widget", true)

	for _, want := range []string{
		"clone acme/widget",
		"push the signed branch to acme/widget",
		"hold",
	} {
		if !strings.Contains(strings.ToLower(prompt), want) {
			t.Errorf("managed prompt missing %q: %s", want, prompt)
		}
	}
	for _, forbidden := range []string{"fork", "do not have push access"} {
		if strings.Contains(strings.ToLower(prompt), forbidden) {
			t.Errorf("managed prompt contains personal-contributor instruction %q: %s", forbidden, prompt)
		}
	}
}

func TestPersonalContributorPromptRetainsForkWorkflow(t *testing.T) {
	ref := worksource.Ref{Repo: "acme/widget", Number: 42}
	prompt := buildTaskPromptForContributor(ref, "repair the widget", false)
	if !strings.Contains(strings.ToLower(prompt), "fork") {
		t.Fatalf("personal contributor prompt lost fork workflow: %s", prompt)
	}
}

func TestContributorCompletionFailsClosedWhenRequiredHoldCannotBeApplied(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/widget/pulls/7", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"user": map[string]any{"login": "hive-app[bot]"},
			"base": map[string]any{"repo": map[string]any{"full_name": "acme/widget"}},
		})
	})
	mux.HandleFunc("/repos/acme/widget/issues/7/labels", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"labels unavailable"}`, http.StatusInternalServerError)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := ghpkg.NewClientForTest(ts.URL, "acme", []string{"widget"}, logger)
	ctx, cancel := context.WithCancel(context.Background())
	client.StartPRRequestWatcher(ctx, nil, func() bool { return true }, time.Now)
	cancel()

	server := NewServer(0, logger)
	server.RegisterAPI(&Dependencies{GHClient: client, Logger: logger, Ctx: context.Background()})
	hub := NewContributeWSHub(logger, server)
	result := hub.verifyReportedPRDetail("acme/widget", "https://github.com/acme/widget/pull/7", "hive-app[bot]")
	if result.Verified {
		t.Fatalf("delivery verified without its required hold label: %+v", result)
	}
	if !strings.Contains(strings.ToLower(result.Reason), "hold") {
		t.Fatalf("reason = %q, want hold failure", result.Reason)
	}
}

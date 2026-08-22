package github

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnsureReportedPRHoldAppliesConfiguredHold(t *testing.T) {
	var labels []string
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/widget/issues/7/labels", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&labels); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := NewClientForTest(ts.URL, "acme", []string{"widget"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	client.prHoldLabel = func() bool { return true }
	required, err := client.EnsureReportedPRHold(context.Background(), "acme/widget", "https://github.com/acme/widget/pull/7")
	if err != nil {
		t.Fatal(err)
	}
	if !required {
		t.Fatal("hold should be required")
	}
	if len(labels) != 1 || labels[0] != "hold" {
		t.Fatalf("labels = %v, want [hold]", labels)
	}
}

func TestEnsureReportedPRHoldIsInertOutsideHoldMode(t *testing.T) {
	client := &Client{prHoldLabel: func() bool { return false }}
	required, err := client.EnsureReportedPRHold(context.Background(), "acme/widget", "https://github.com/acme/widget/pull/7")
	if err != nil || required {
		t.Fatalf("required=%v err=%v, want false nil", required, err)
	}
}

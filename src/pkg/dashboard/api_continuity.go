package dashboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kubestellar/hive/pkg/continuity"
)

type continuityPRRequest struct {
	Action             string `json:"action"`
	Repo               string `json:"repo"`
	PRNumber           int    `json:"pr_number"`
	WorkRef            string `json:"work_ref,omitempty"`
	ExpectedGeneration uint64 `json:"expected_generation,omitempty"`
	Reason             string `json:"reason,omitempty"`
}

type continuityPRResponse struct {
	Action      string                  `json:"action"`
	DryRun      bool                    `json:"dry_run"`
	Observation *continuity.Observation `json:"observation,omitempty"`
	Record      *continuity.Record      `json:"record,omitempty"`
}

func (s *Server) handleContinuityPRAdoptions(w http.ResponseWriter, r *http.Request) {
	if !requireOwnerRole(w, r) {
		return
	}
	if s.deps == nil || s.deps.Config == nil || s.deps.ContinuityLedger == nil {
		jsonError(w, "continuity authority unavailable", http.StatusServiceUnavailable)
		return
	}
	if r.Method == http.MethodGet {
		jsonResponse(w, map[string]any{"records": s.deps.ContinuityLedger.List()})
		return
	}
	var req continuityPRRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	ref, err := s.continuityPRRef(req.Repo, req.PRNumber)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	principal := requestUser(r)
	now := time.Now().UTC()
	response := continuityPRResponse{Action: action}

	s.continuityMu.Lock()
	defer s.continuityMu.Unlock()

	switch action {
	case "dry_run", "adopt", "refresh", "reacquire", "promote_suppression":
		observe := s.deps.ObserveContinuityPR
		if observe == nil && s.deps.GHClient != nil {
			observe = s.deps.GHClient.ObserveContinuityPR
		}
		if observe == nil {
			jsonError(w, "GitHub continuity observer unavailable", http.StatusServiceUnavailable)
			return
		}
		obs, observeErr := observe(r.Context(), ref)
		if observeErr != nil {
			jsonError(w, observeErr.Error(), http.StatusBadGateway)
			return
		}
		response.Observation = &obs
		if action == "dry_run" {
			response.DryRun = true
			jsonResponse(w, response)
			return
		}
		var rec continuity.Record
		switch action {
		case "adopt":
			rec, err = s.deps.ContinuityLedger.Adopt(obs, principal, "verified-owner-dashboard", now)
		case "refresh":
			rec, err = s.deps.ContinuityLedger.Refresh(obs, now)
		case "reacquire":
			rec, err = s.deps.ContinuityLedger.Reacquire(obs, req.ExpectedGeneration, principal, "verified-owner-dashboard", now)
		case "promote_suppression":
			rec, err = s.deps.ContinuityLedger.PromoteSuppression(ref, req.WorkRef, req.ExpectedGeneration, principal, "verified-owner-dashboard", now)
		}
		response.Record = &rec
	case "revoke":
		rec, revokeErr := s.deps.ContinuityLedger.Revoke(ref, req.ExpectedGeneration, principal, strings.TrimSpace(req.Reason), now)
		err = revokeErr
		response.Record = &rec
	default:
		jsonError(w, "action must be dry_run, adopt, refresh, reacquire, promote_suppression, or revoke", http.StatusBadRequest)
		return
	}
	if err != nil {
		status := http.StatusConflict
		if errors.Is(err, continuity.ErrUnauthorized) {
			status = http.StatusForbidden
		} else if errors.Is(err, continuity.ErrNotFound) {
			status = http.StatusNotFound
		}
		jsonError(w, err.Error(), status)
		return
	}
	if s.audit != nil {
		s.auditFromRequest(r, "continuity_pr_"+action,
			auditDetail("repo", ref.Repo, "pr", fmt.Sprintf("%d", ref.Number), "generation", fmt.Sprintf("%d", response.Record.Generation)), "")
	}
	jsonResponse(w, response)
}

func (s *Server) continuityPRRef(repo string, number int) (continuity.PRRef, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" || number <= 0 {
		return continuity.PRRef{}, fmt.Errorf("repository and positive pr_number are required")
	}
	org := strings.TrimSpace(s.deps.Config.Project.Org)
	full := repo
	if !strings.Contains(repo, "/") {
		full = org + "/" + repo
	}
	allowed := false
	for _, configured := range s.deps.Config.Project.Repos {
		configuredFull := configured
		if !strings.Contains(configured, "/") {
			configuredFull = org + "/" + configured
		}
		if strings.EqualFold(full, configuredFull) {
			allowed = true
			break
		}
	}
	if !allowed {
		return continuity.PRRef{}, fmt.Errorf("repository %s is outside configured project boundary", full)
	}
	ref := continuity.PRRef{Repo: full, Number: number}
	if err := ref.Validate(); err != nil {
		return continuity.PRRef{}, err
	}
	return ref, nil
}

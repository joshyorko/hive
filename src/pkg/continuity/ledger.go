package continuity

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"
)

var (
	ErrUnauthorized       = errors.New("verified owner authority is required")
	ErrNotFound           = errors.New("pull request adoption not found")
	ErrGenerationConflict = errors.New("adoption generation conflict")
	ErrHeadMoved          = errors.New("adopted pull request head moved unexpectedly")
)

const ledgerVersion = 1
const ledgerFileMode = 0o660

const DefaultLedgerPath = "/data/continuity-pr-adoptions.json"

type persistedLedger struct {
	Version int      `json:"version"`
	Records []Record `json:"records"`
}

type Ledger struct {
	path    string
	mu      sync.RWMutex
	records map[string]*Record
}

func OpenLedger(path string) (*Ledger, error) {
	l := &Ledger{path: path, records: map[string]*Record{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading continuity ledger: %w", err)
	}
	var disk persistedLedger
	if err := json.Unmarshal(data, &disk); err != nil {
		return nil, fmt.Errorf("continuity ledger is unparseable and left untouched: %w", err)
	}
	if disk.Version != ledgerVersion {
		return nil, fmt.Errorf("unsupported continuity ledger version %d", disk.Version)
	}
	for i := range disk.Records {
		rec := disk.Records[i]
		if rec.Ref.Key() == "" || rec.Generation < 1 {
			return nil, fmt.Errorf("continuity ledger contains invalid record %+v", rec.Ref)
		}
		if _, exists := l.records[rec.Ref.Key()]; exists {
			return nil, fmt.Errorf("continuity ledger contains duplicate %s", rec.Ref.Key())
		}
		copy := rec.clone()
		l.records[rec.Ref.Key()] = &copy
	}
	return l, nil
}

func (l *Ledger) Adopt(obs Observation, principal, authorityProvenance string, now time.Time) (Record, error) {
	if strings.TrimSpace(principal) == "" || strings.TrimSpace(authorityProvenance) == "" || strings.HasPrefix(authorityProvenance, "github-label:") {
		return Record{}, ErrUnauthorized
	}
	if err := obs.Validate(); err != nil {
		return Record{}, err
	}
	if obs.ObservedAt.IsZero() {
		obs.ObservedAt = now
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	key := obs.Ref.Key()
	if prior, ok := l.records[key]; ok {
		if prior.Active && prior.ObservedHeadSHA == obs.HeadSHA && prior.HeadBranch == obs.HeadBranch && prior.HeadRepo == obs.HeadRepo {
			return prior.clone(), nil
		}
		return Record{}, fmt.Errorf("%s already exists at generation %d; refresh/reacquire explicitly: %w", key, prior.Generation, ErrGenerationConflict)
	}
	rec := recordFromObservation(obs)
	rec.Active = true
	rec.AdoptionPrincipal = principal
	rec.AdoptionGeneration = 1
	rec.Generation = 1
	rec.AdoptedAt = now
	rec.History = []Transition{{Verb: "adopt", Generation: 1, Principal: principal, Provenance: authorityProvenance, At: now}}
	if err := l.commitLocked(key, &rec); err != nil {
		return Record{}, err
	}
	return rec.clone(), nil
}

func recordFromObservation(obs Observation) Record {
	return Record{
		Ref: obs.Ref, OriginalAuthor: obs.OriginalAuthor, HeadRepo: obs.HeadRepo,
		HeadBranch: obs.HeadBranch, BaseBranch: obs.BaseBranch,
		ObservedHeadSHA: obs.HeadSHA, CurrentHeadSHA: obs.HeadSHA,
		BaseSHA: obs.BaseSHA, MergeBaseSHA: obs.MergeBaseSHA,
		LinkedWork:      append([]WorkRelationship(nil), obs.LinkedWork...),
		Acceptance:      append([]AcceptanceDelta(nil), obs.Acceptance...),
		Stack:           append([]StackRelation(nil), obs.Stack...),
		OverlappingPRs:  append([]PRRef(nil), obs.OverlappingPRs...),
		ChangedFiles:    append([]string(nil), obs.ChangedFiles...),
		WriteCapability: obs.WriteCapability, Draft: obs.Draft, Hold: obs.Hold,
		Mergeable: obs.Mergeable, CIStatus: obs.CIStatus, State: obs.State,
		StateReason: obs.StateReason, ObservedAt: obs.ObservedAt, Provenance: obs.Provenance,
	}
}

func (l *Ledger) Refresh(obs Observation, now time.Time) (Record, error) {
	if err := obs.Validate(); err != nil {
		return Record{}, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	key := obs.Ref.Key()
	prior, ok := l.records[key]
	if !ok {
		return Record{}, ErrNotFound
	}
	if observationMatchesRecord(obs, *prior) {
		return prior.clone(), nil
	}
	work := prior.clone()
	work.Generation++
	work.CurrentHeadSHA = obs.HeadSHA
	work.ObservedAt = now
	work.BaseSHA = obs.BaseSHA
	work.MergeBaseSHA = obs.MergeBaseSHA
	work.Mergeable = obs.Mergeable
	work.CIStatus = obs.CIStatus
	work.Hold = obs.Hold
	work.Draft = obs.Draft
	work.WriteCapability = obs.WriteCapability
	if obs.HeadSHA != prior.ObservedHeadSHA || obs.HeadBranch != prior.HeadBranch || obs.HeadRepo != prior.HeadRepo {
		work.State = StateUnknown
		work.StateReason = "observed head identity changed after adoption; owner reacquisition required"
		work.History = append(work.History, Transition{Verb: "head_moved", Generation: work.Generation, Principal: "observer", Provenance: obs.Provenance, At: now, Reason: work.StateReason})
		if err := l.commitLocked(key, &work); err != nil {
			return Record{}, err
		}
		return work.clone(), ErrHeadMoved
	}
	work.State, work.StateReason = obs.State, obs.StateReason
	work.LinkedWork = append([]WorkRelationship(nil), obs.LinkedWork...)
	work.SuppressionClaims = append([]SuppressionClaim(nil), prior.SuppressionClaims...)
	work.Acceptance = append([]AcceptanceDelta(nil), obs.Acceptance...)
	work.Stack = append([]StackRelation(nil), obs.Stack...)
	work.OverlappingPRs = append([]PRRef(nil), obs.OverlappingPRs...)
	work.ChangedFiles = append([]string(nil), obs.ChangedFiles...)
	work.History = append(work.History, Transition{Verb: "refresh", Generation: work.Generation, Principal: "observer", Provenance: obs.Provenance, At: now})
	if err := l.commitLocked(key, &work); err != nil {
		return Record{}, err
	}
	return work.clone(), nil
}

func observationMatchesRecord(obs Observation, rec Record) bool {
	return obs.OriginalAuthor == rec.OriginalAuthor && obs.HeadRepo == rec.HeadRepo &&
		obs.HeadBranch == rec.HeadBranch && obs.BaseBranch == rec.BaseBranch &&
		obs.HeadSHA == rec.ObservedHeadSHA && obs.HeadSHA == rec.CurrentHeadSHA &&
		obs.BaseSHA == rec.BaseSHA && obs.MergeBaseSHA == rec.MergeBaseSHA &&
		obs.WriteCapability == rec.WriteCapability && obs.Draft == rec.Draft && obs.Hold == rec.Hold &&
		obs.Mergeable == rec.Mergeable && obs.CIStatus == rec.CIStatus && obs.State == rec.State &&
		obs.StateReason == rec.StateReason && reflect.DeepEqual(obs.LinkedWork, rec.LinkedWork) &&
		reflect.DeepEqual(obs.Acceptance, rec.Acceptance) && reflect.DeepEqual(obs.Stack, rec.Stack) &&
		reflect.DeepEqual(obs.OverlappingPRs, rec.OverlappingPRs) && reflect.DeepEqual(obs.ChangedFiles, rec.ChangedFiles)
}

// Degrade records a source-local UNKNOWN without releasing ownership. It is
// used when the observer cannot reacquire an adopted PR; unrelated records are
// untouched and replacement work for this record stays suppressed.
func (l *Ledger) Degrade(ref PRRef, reason, provenance string, now time.Time) (Record, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	prior, ok := l.records[ref.Key()]
	if !ok {
		return Record{}, ErrNotFound
	}
	if prior.State == StateUnknown && prior.StateReason == reason {
		return prior.clone(), nil
	}
	work := prior.clone()
	work.Generation++
	work.State = StateUnknown
	work.StateReason = reason
	work.ObservedAt = now
	work.History = append(work.History, Transition{Verb: "degrade", Generation: work.Generation, Principal: "observer", Provenance: provenance, At: now, Reason: reason})
	if err := l.commitLocked(ref.Key(), &work); err != nil {
		return Record{}, err
	}
	return work.clone(), nil
}

// AcceptDelivery advances an active adoption after the contributor delivery
// verifier proves that expectedHead is an ancestor of obs.HeadSHA and that all
// new commits belong to the assigned contributor. This is continuation
// authority, not a new owner adoption or release/merge authorization.
func (l *Ledger) AcceptDelivery(obs Observation, expectedHead, principal, provenance string, now time.Time) (Record, error) {
	if strings.TrimSpace(principal) == "" || strings.TrimSpace(provenance) == "" {
		return Record{}, ErrUnauthorized
	}
	if err := obs.Validate(); err != nil {
		return Record{}, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	prior, ok := l.records[obs.Ref.Key()]
	if !ok {
		return Record{}, ErrNotFound
	}
	if !prior.Active || prior.ObservedHeadSHA != expectedHead || obs.HeadSHA == expectedHead ||
		(prior.CurrentHeadSHA != expectedHead && prior.CurrentHeadSHA != obs.HeadSHA) {
		return Record{}, ErrHeadMoved
	}
	work := recordFromObservation(obs)
	work.Active = true
	work.AdoptionPrincipal = prior.AdoptionPrincipal
	work.AdoptionGeneration = prior.AdoptionGeneration
	work.Generation = prior.Generation + 1
	work.AdoptedAt = prior.AdoptedAt
	work.SuppressionClaims = append([]SuppressionClaim(nil), prior.SuppressionClaims...)
	work.History = append(append([]Transition(nil), prior.History...), Transition{
		Verb: "delivery", Generation: work.Generation, Principal: principal,
		Provenance: provenance, At: now, Reason: "verified fast-forward continuation delivery",
	})
	if err := l.commitLocked(obs.Ref.Key(), &work); err != nil {
		return Record{}, err
	}
	return work.clone(), nil
}

func (l *Ledger) Reacquire(obs Observation, expected uint64, principal, authorityProvenance string, now time.Time) (Record, error) {
	if strings.TrimSpace(principal) == "" || strings.TrimSpace(authorityProvenance) == "" || strings.HasPrefix(authorityProvenance, "github-label:") {
		return Record{}, ErrUnauthorized
	}
	if err := obs.Validate(); err != nil {
		return Record{}, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	key := obs.Ref.Key()
	prior, ok := l.records[key]
	if !ok {
		return Record{}, ErrNotFound
	}
	if prior.Generation != expected {
		return Record{}, ErrGenerationConflict
	}
	work := recordFromObservation(obs)
	work.Active = true
	work.AdoptionPrincipal = principal
	work.AdoptionGeneration = prior.AdoptionGeneration + 1
	work.Generation = prior.Generation + 1
	work.AdoptedAt = now
	work.SuppressionClaims = append([]SuppressionClaim(nil), prior.SuppressionClaims...)
	work.History = append(append([]Transition(nil), prior.History...), Transition{Verb: "reacquire", Generation: work.Generation, Principal: principal, Provenance: authorityProvenance, At: now})
	if err := l.commitLocked(key, &work); err != nil {
		return Record{}, err
	}
	return work.clone(), nil
}

func (l *Ledger) Revoke(ref PRRef, expected uint64, principal, reason string, now time.Time) (Record, error) {
	if strings.TrimSpace(principal) == "" {
		return Record{}, ErrUnauthorized
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	prior, ok := l.records[ref.Key()]
	if !ok {
		return Record{}, ErrNotFound
	}
	if prior.Generation != expected {
		return Record{}, ErrGenerationConflict
	}
	work := prior.clone()
	work.Generation++
	work.Active = false
	work.StateReason = reason
	work.History = append(work.History, Transition{Verb: "revoke", Generation: work.Generation, Principal: principal, Provenance: "owner-gated-dashboard", At: now, Reason: reason})
	if err := l.commitLocked(ref.Key(), &work); err != nil {
		return Record{}, err
	}
	return work.clone(), nil
}

func (l *Ledger) Get(ref PRRef) (Record, bool) {
	if l == nil {
		return Record{}, false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	rec, ok := l.records[ref.Key()]
	if !ok {
		return Record{}, false
	}
	return rec.clone(), true
}

func (l *Ledger) List() []Record {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Record, 0, len(l.records))
	for _, rec := range l.records {
		out = append(out, rec.clone())
	}
	sortRecords(out)
	return out
}

func (l *Ledger) LookupWork(workKey string) []Record {
	if l == nil || workKey == "" {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	var out []Record
	for _, rec := range l.records {
		if !rec.Active || rec.State == StateSuperseded {
			continue
		}
		if activeSuppressionClaim(rec.SuppressionClaims, workKey) != nil {
			out = append(out, rec.clone())
			continue
		}
		for _, rel := range rec.LinkedWork {
			if rel.WorkRef == workKey && !rel.Ambiguous && strings.TrimSpace(rel.OwnedSlice) != "" {
				out = append(out, rec.clone())
				break
			}
		}
	}
	sortRecords(out)
	return out
}

func (l *Ledger) PromoteSuppression(ref PRRef, workRef string, expected uint64, principal, authorityProvenance string, now time.Time) (Record, error) {
	if strings.TrimSpace(principal) == "" || strings.TrimSpace(authorityProvenance) == "" || strings.HasPrefix(authorityProvenance, "github-label:") {
		return Record{}, ErrUnauthorized
	}
	if !validWorkKey(workRef) {
		return Record{}, fmt.Errorf("invalid work ref %q", workRef)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	prior, ok := l.records[ref.Key()]
	if !ok {
		return Record{}, ErrNotFound
	}
	if !prior.Active {
		return Record{}, ErrNotFound
	}
	if !suppressionClaimDiscovered(prior.LinkedWork, workRef) {
		return Record{}, ErrNotFound
	}
	if claim := activeSuppressionClaim(prior.SuppressionClaims, workRef); claim != nil {
		return prior.clone(), nil
	}
	if prior.Generation != expected {
		return Record{}, ErrGenerationConflict
	}
	work := prior.clone()
	work.Generation++
	work.SuppressionClaims = append(work.SuppressionClaims, SuppressionClaim{
		WorkRef: workRef, Principal: principal, Provenance: authorityProvenance,
		Generation: work.Generation, Active: true, ClaimedAt: now,
	})
	work.History = append(work.History, Transition{Verb: "suppress", Generation: work.Generation, Principal: principal, Provenance: authorityProvenance, At: now, Reason: workRef})
	if err := l.commitLocked(ref.Key(), &work); err != nil {
		return Record{}, err
	}
	return work.clone(), nil
}

func suppressionClaimDiscovered(rels []WorkRelationship, workRef string) bool {
	for _, rel := range rels {
		if rel.WorkRef == workRef && rel.Ambiguous && rel.Relationship != RelationshipCloses {
			return true
		}
	}
	return false
}

func activeSuppressionClaim(claims []SuppressionClaim, workRef string) *SuppressionClaim {
	for i := range claims {
		if claims[i].Active && claims[i].WorkRef == workRef {
			return &claims[i]
		}
	}
	return nil
}

func (l *Ledger) commitLocked(key string, rec *Record) error {
	prior, had := l.records[key]
	copy := rec.clone()
	l.records[key] = &copy
	if err := l.persistLocked(); err != nil {
		if had {
			l.records[key] = prior
		} else {
			delete(l.records, key)
		}
		return err
	}
	return nil
}

func (l *Ledger) persistLocked() error {
	records := make([]Record, 0, len(l.records))
	for _, rec := range l.records {
		records = append(records, rec.clone())
	}
	sortRecords(records)
	data, err := json.MarshalIndent(persistedLedger{Version: ledgerVersion, Records: records}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepathDir(l.path), 0o770); err != nil {
		return err
	}
	tmp := l.path + ".tmp"
	if err := os.WriteFile(tmp, data, ledgerFileMode); err != nil {
		return err
	}
	if err := os.Rename(tmp, l.path); err != nil {
		return err
	}
	return nil
}

func filepathDir(path string) string {
	if i := strings.LastIndex(path, string(os.PathSeparator)); i >= 0 {
		if i == 0 {
			return string(os.PathSeparator)
		}
		return path[:i]
	}
	return "."
}

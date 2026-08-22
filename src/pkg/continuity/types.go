// Package continuity owns explicit operator adoption of pre-existing work.
// Discovery remains source-specific and non-authoritative; only records
// durably promoted through Ledger are allowed to affect admission or mutation.
package continuity

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type State string

const (
	StateContinue   State = "CONTINUE"
	StateReady      State = "READY"
	StateBlocked    State = "BLOCKED"
	StateSuperseded State = "SUPERSEDED"
	StateUnknown    State = "UNKNOWN"
)

type WriteCapability string

const (
	CapabilityWritable   WriteCapability = "writable"
	CapabilityUnwritable WriteCapability = "unwritable"
	CapabilityUnknown    WriteCapability = "unknown"
)

const (
	RelationshipCloses     = "closes"
	RelationshipReferences = "references"
	RelationshipBranch     = "branch"
)

type PRRef struct {
	Repo   string `json:"repo"`
	Number int    `json:"number"`
}

func (r PRRef) Validate() error {
	if strings.Count(r.Repo, "/") != 1 || strings.ContainsAny(r.Repo, "#!@ \t") || r.Number <= 0 {
		return fmt.Errorf("invalid pull request ref %+v", r)
	}
	return nil
}

func (r PRRef) Key() string {
	if r.Validate() != nil {
		return ""
	}
	return fmt.Sprintf("%s!pr-%d", r.Repo, r.Number)
}

func (r PRRef) WorkKey() string {
	return r.Key()
}

type WorkRelationship struct {
	WorkRef      string `json:"work_ref"`
	Relationship string `json:"relationship"`
	OwnedSlice   string `json:"owned_slice,omitempty"`
	Evidence     string `json:"evidence,omitempty"`
	Ambiguous    bool   `json:"ambiguous,omitempty"`
}

type AcceptanceDelta struct {
	WorkRef            string   `json:"work_ref"`
	Owned              []string `json:"owned,omitempty"`
	Satisfied          []string `json:"satisfied,omitempty"`
	Missing            []string `json:"missing,omitempty"`
	OwnedByOther       []string `json:"owned_by_other,omitempty"`
	Ambiguous          []string `json:"ambiguous,omitempty"`
	ClosingKeywordRisk bool     `json:"closing_keyword_risk,omitempty"`
}

type StackRelation struct {
	PRRef    PRRef  `json:"pr_ref"`
	Kind     string `json:"kind"`
	Evidence string `json:"evidence,omitempty"`
}

type Observation struct {
	Ref             PRRef              `json:"ref"`
	OriginalAuthor  string             `json:"original_author"`
	HeadRepo        string             `json:"head_repository"`
	HeadBranch      string             `json:"head_branch"`
	BaseBranch      string             `json:"base_branch"`
	HeadSHA         string             `json:"head_sha"`
	BaseSHA         string             `json:"base_sha"`
	MergeBaseSHA    string             `json:"merge_base_sha"`
	Draft           bool               `json:"draft"`
	Hold            bool               `json:"hold"`
	Mergeable       string             `json:"mergeable"`
	CIStatus        string             `json:"ci_status"`
	WriteCapability WriteCapability    `json:"write_capability"`
	LinkedWork      []WorkRelationship `json:"linked_work,omitempty"`
	Acceptance      []AcceptanceDelta  `json:"acceptance_delta,omitempty"`
	Stack           []StackRelation    `json:"stack,omitempty"`
	OverlappingPRs  []PRRef            `json:"overlapping_prs,omitempty"`
	ChangedFiles    []string           `json:"changed_files,omitempty"`
	State           State              `json:"state"`
	StateReason     string             `json:"state_reason,omitempty"`
	Provenance      string             `json:"provenance"`
	ObservedAt      time.Time          `json:"observed_at"`
}

func (o Observation) Validate() error {
	if err := o.Ref.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(o.OriginalAuthor) == "" || strings.TrimSpace(o.HeadRepo) == "" ||
		strings.TrimSpace(o.HeadBranch) == "" || strings.TrimSpace(o.BaseBranch) == "" ||
		strings.TrimSpace(o.HeadSHA) == "" || strings.TrimSpace(o.Provenance) == "" {
		return fmt.Errorf("pull request observation lacks authoritative identity: %+v", o.Ref)
	}
	switch o.State {
	case StateContinue, StateReady, StateBlocked, StateSuperseded, StateUnknown:
	default:
		return fmt.Errorf("invalid continuation state %q", o.State)
	}
	for _, rel := range o.LinkedWork {
		if !validWorkKey(rel.WorkRef) {
			return fmt.Errorf("linked work %q is not a canonical worksource ref", rel.WorkRef)
		}
	}
	return nil
}

func validWorkKey(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	if repo, number, ok := strings.Cut(key, "#"); ok {
		if strings.Count(repo, "/") != 1 || number == "" {
			return false
		}
		for _, r := range number {
			if r < '0' || r > '9' {
				return false
			}
		}
		return number != "0"
	}
	repo, external, ok := strings.Cut(key, "!")
	return ok && strings.Count(repo, "/") == 1 && strings.TrimSpace(external) != ""
}

type Transition struct {
	Verb       string    `json:"verb"`
	Generation uint64    `json:"generation"`
	Principal  string    `json:"principal"`
	Provenance string    `json:"provenance"`
	At         time.Time `json:"at"`
	Reason     string    `json:"reason,omitempty"`
}

type Record struct {
	Ref                PRRef              `json:"ref"`
	OriginalAuthor     string             `json:"original_author"`
	HeadRepo           string             `json:"head_repository"`
	HeadBranch         string             `json:"head_branch"`
	BaseBranch         string             `json:"base_branch"`
	ObservedHeadSHA    string             `json:"observed_head_sha"`
	CurrentHeadSHA     string             `json:"current_head_sha"`
	BaseSHA            string             `json:"base_sha"`
	MergeBaseSHA       string             `json:"merge_base_sha"`
	LinkedWork         []WorkRelationship `json:"linked_work,omitempty"`
	Acceptance         []AcceptanceDelta  `json:"acceptance_delta,omitempty"`
	Stack              []StackRelation    `json:"stack,omitempty"`
	OverlappingPRs     []PRRef            `json:"overlapping_prs,omitempty"`
	ChangedFiles       []string           `json:"changed_files,omitempty"`
	WriteCapability    WriteCapability    `json:"write_capability"`
	Draft              bool               `json:"draft"`
	Hold               bool               `json:"hold"`
	Mergeable          string             `json:"mergeable"`
	CIStatus           string             `json:"ci_status"`
	State              State              `json:"state"`
	StateReason        string             `json:"state_reason,omitempty"`
	Active             bool               `json:"active"`
	AdoptionPrincipal  string             `json:"adoption_principal"`
	AdoptionGeneration uint64             `json:"adoption_generation"`
	Generation         uint64             `json:"generation"`
	AdoptedAt          time.Time          `json:"adopted_at"`
	ObservedAt         time.Time          `json:"observed_at"`
	Provenance         string             `json:"provenance"`
	History            []Transition       `json:"history"`
}

func (r Record) Continuable() bool {
	return r.Active && r.State == StateContinue && r.WriteCapability == CapabilityWritable && r.CurrentHeadSHA == r.ObservedHeadSHA
}

func (r Record) clone() Record {
	out := r
	out.LinkedWork = append([]WorkRelationship(nil), r.LinkedWork...)
	out.Acceptance = append([]AcceptanceDelta(nil), r.Acceptance...)
	out.Stack = append([]StackRelation(nil), r.Stack...)
	out.OverlappingPRs = append([]PRRef(nil), r.OverlappingPRs...)
	out.ChangedFiles = append([]string(nil), r.ChangedFiles...)
	out.History = append([]Transition(nil), r.History...)
	return out
}

func sortRecords(records []Record) {
	sort.Slice(records, func(i, j int) bool { return records[i].Ref.Key() < records[j].Ref.Key() })
}

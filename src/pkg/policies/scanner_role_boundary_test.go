package policies

import (
	"strings"
	"testing"
)

func TestDefaultScannerPoliciesKeepSecurityPrivate(t *testing.T) {
	paths := []string{
		"defaults/scanner.md",
		"defaults/scanner-advisory.md",
		"defaults/scanner-issues.md",
		"defaults/scanner-holdgated.md",
		"defaults/scanner-full.md",
		"defaults/scanner-automerge.md",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			data, err := DefaultPolicies.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			policy := strings.ToLower(string(data))
			for _, required := range []string{"security", "private", "operator"} {
				if !strings.Contains(policy, required) {
					t.Errorf("scanner policy lacks %q boundary", required)
				}
			}
		})
	}
}

func TestDefaultL5ScannerPolicyAllowsAdmittedImplementation(t *testing.T) {
	data, err := DefaultPolicies.ReadFile("defaults/scanner-holdgated.md")
	if err != nil {
		t.Fatal(err)
	}
	policy := strings.ToLower(string(data))
	for _, required := range []string{
		"work list is your implementation queue",
		"create a worktree",
		"implement",
		"git push",
		"hive-open-pr",
		"hold",
		"never merge",
	} {
		if !strings.Contains(policy, required) {
			t.Errorf("L5 scanner policy does not authorize %q", required)
		}
	}
	for _, forbidden := range []string{
		"not the production implementation worker",
		"implementation work is claimed through hive's contributor path",
		"do not implement backlog issues",
	} {
		if strings.Contains(policy, forbidden) {
			t.Errorf("L5 scanner policy contradicts ISSUES_AND_PRS with %q", forbidden)
		}
	}
}

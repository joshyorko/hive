package policies

import (
	"strings"
	"testing"
)

func TestDefaultScannerPoliciesKeepSecurityPrivateAndImplementationSeparate(t *testing.T) {
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
			for _, required := range []string{"security", "private", "operator", "contributor path"} {
				if !strings.Contains(policy, required) {
					t.Errorf("scanner policy lacks %q boundary", required)
				}
			}
			for _, forbidden := range []string{
				"git push",
				"gh pr create",
				"hive-open-pr",
				"implement the fix",
				"create github issues for findings",
			} {
				if strings.Contains(policy, forbidden) {
					t.Errorf("scanner policy still authorizes production/public action %q", forbidden)
				}
			}
		})
	}
}

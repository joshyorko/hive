package policies

import (
	"strings"
	"testing"
)

const scannerAutomergePolicyPath = "defaults/scanner-automerge.md"

func readScannerAutomergePolicy(t *testing.T) string {
	t.Helper()
	data, err := DefaultPolicies.ReadFile(scannerAutomergePolicyPath)
	if err != nil {
		t.Fatalf("read embedded %s: %v", scannerAutomergePolicyPath, err)
	}
	return string(data)
}

func TestScannerAutomergeLegacyNameDoesNotGrantMergeAuthority(t *testing.T) {
	policy := strings.ToLower(readScannerAutomergePolicy(t))
	for _, forbidden := range []string{"merge sweep", "ci repair", "dispatching fixes", "hive-merge"} {
		if strings.Contains(policy, forbidden) {
			t.Errorf("legacy scanner policy still contains %q authority", forbidden)
		}
	}
	for _, required := range []string{"legacy template name does not grant", "never merge", "contributor path"} {
		if !strings.Contains(policy, required) {
			t.Errorf("legacy scanner policy lacks boundary %q", required)
		}
	}
}

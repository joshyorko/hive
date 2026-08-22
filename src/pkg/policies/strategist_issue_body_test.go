package policies

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStrategistIssueExamplesUseBodyFileForMultilineMarkdown(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	for _, name := range []string{"strategist-holdgated.md", "strategist-full.md"} {
		body, err := os.ReadFile(filepath.Join(root, "policies", name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		if !strings.Contains(text, "--body-file \"$issue_body\"") || !strings.Contains(text, "<<'HIVE_ISSUE_BODY'") {
			t.Fatalf("%s must deliver multiline issue Markdown through a quoted body file", name)
		}
		if strings.Contains(text, `--body "## Strategic Finding`) {
			t.Fatalf("%s still teaches fragile multiline shell arguments", name)
		}
	}
}

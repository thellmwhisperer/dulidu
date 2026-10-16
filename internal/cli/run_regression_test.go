//go:build unix

package cli

import (
	"os"
	"strings"
	"testing"
)

func TestRunRejectsPositionalAttachmentBeforeWritesOrCalls(t *testing.T) {
	calls := runRepo(t, map[string]string{"notes.md": "Some notes.\n"})
	code, _, errOut := dulidu(t, "run", "brief.md", "notes.md")
	if code == 0 || !strings.Contains(errOut, "accepts 1 arg") {
		t.Fatalf("want extra positional argument rejected, got exit %d and %q", code, errOut)
	}
	if dirs := runs(t); len(dirs) != 0 {
		t.Errorf("a run folder was written: %v", dirs)
	}
	if _, err := os.Stat(calls); err == nil {
		t.Error("the agent was called")
	}
}

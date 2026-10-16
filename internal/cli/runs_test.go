package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunsListsRunFoldersNewestFirst(t *testing.T) {
	dir := repo(t)
	if code, out, errOut := dulidu(t, "runs"); code != 0 || out != "" {
		t.Fatalf("dulidu runs with no runs: exit %d, %q\n%s", code, out, errOut)
	}
	for label, report := range map[string]string{
		"b-newer": `{"status": "running", "started": "2026-10-16T12:00:00Z", "steps": [
			{"name": "triage", "status": "done", "model": "opus", "harness": "claude",
			 "started": "2026-10-16T12:00:00Z", "finished": "2026-10-16T12:01:00Z"},
			{"name": "research", "status": "running", "model": "opus", "harness": "claude",
			 "started": "2026-10-16T12:01:00Z"}]}`,
		"a-older": `{"status": "done", "started": "2026-10-15T09:30:00Z", "steps": [
			{"name": "triage", "status": "done", "model": "", "harness": "codex",
			 "started": "2026-10-15T09:30:00Z", "finished": "2026-10-15T09:31:00Z"}]}`,
	} {
		run := filepath.Join(dir, ".dulidu", "runs", label)
		if err := os.MkdirAll(run, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(run, "report.json"), []byte(report), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	code, out, errOut := dulidu(t, "runs")
	if code != 0 {
		t.Fatalf("dulidu runs: exit %d\n%s%s", code, out, errOut)
	}
	want := []string{"b-newer running research 2026-10-16T12:00:00Z", "a-older done triage 2026-10-15T09:30:00Z"}
	if got := strings.Split(strings.TrimSpace(out), "\n"); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("dulidu runs:\n%s\nwant:\n%s", out, strings.Join(want, "\n"))
	}
}

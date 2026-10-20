package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
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

func TestRunsListsOldRunFolders(t *testing.T) {
	dir := repo(t)
	old := filepath.Join(dir, ".dulidu", "runs", "a-older")
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"report.json": `{"status":"done","started":"2026-10-15T09:30:00Z","steps":[
			{"name":"analysis","status":"done","harness":"codex","started":"2026-10-15T09:30:00Z"}]}`,
		"analysis.json": `{"personas":["Reader"],"functionalities":["Read stories"],
			"flows":["Open a story"],"constraints":[],"questions":[]}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(old, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	started := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	current := pipeline.Run{
		Report: pipeline.Report{Status: "running", Started: started, Steps: []pipeline.Step{
			{Name: "epics", Status: "running", Harness: "codex", Started: started},
		}},
		Analysis: &pipeline.Analysis{
			Personas:        []pipeline.Persona{{Name: "Reader", Role: "User"}},
			Functionalities: []pipeline.Functionality{{ID: "F1", Name: "Read stories"}},
			Flows:           []pipeline.Flow{{ID: "FL1", Name: "Open a story", Steps: []string{"Open", "Read"}}},
		},
	}
	if err := pipeline.Write(filepath.Join(dir, ".dulidu", "runs", "b-newer"), current); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := dulidu(t, "runs")
	if code != 0 {
		t.Fatalf("dulidu runs: exit %d\n%s%s", code, out, errOut)
	}
	want := "b-newer running epics 2026-10-16T12:00:00Z\na-older done analysis 2026-10-15T09:30:00Z\n"
	if out != want {
		t.Fatalf("dulidu runs:\n%s\nwant:\n%s", out, want)
	}
	for name, content := range files {
		if got := read(t, filepath.Join(old, name)); got != content {
			t.Errorf("dulidu runs changed old %s: %q", name, got)
		}
	}
}

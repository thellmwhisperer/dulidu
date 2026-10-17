//go:build unix

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGoesThroughTheWholeGraph(t *testing.T) {
	log := runRepo(t, map[string]string{"rules.md": unique})
	triageOn(t, `["market"]`, `[]`)
	code, out, errOut := dulidu(t, "run", "brief.md", "--attach", "rules.md")
	if code != 0 {
		t.Fatalf("dulidu run: exit %d\n%s%s", code, out, errOut)
	}
	dir := runs(t)[0]
	if !strings.HasSuffix(strings.TrimSpace(out), filepath.Join(filepath.Base(dir), "contexto.md")) {
		t.Errorf("stdout %q is not contexto.md's path", out)
	}
	for _, name := range []string{"report.json", "contexto.md", "triage.json", "research.json", "analysis.json",
		"epics.json", "spec.md", "tickets/1.md", "plan.json", "brief-diseno.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	var report reportFile
	decodeFile(t, filepath.Join(dir, "report.json"), &report)
	names := []string{}
	for _, s := range report.Steps {
		names = append(names, s.Name)
		if s.Name != "input" && s.Status != "passed" {
			t.Errorf("step %s: %s", s.Name, s.Status)
		}
		if (s.Name == "analysis" || s.Name == "epics") && (s.Harness != "claude" || s.Model != "sonnet" || s.Effort != "medium") {
			t.Errorf("step %s's agent: %+v", s.Name, s)
		}
	}
	if report.Status != "done" || strings.Join(names, " ") != "input triage research analysis epics" {
		t.Errorf("report.json: %+v", report)
	}
	graph := read(t, log+".graph")
	if strings.Count(graph, "\n") != 7 || strings.Count(graph, "--model sonnet --effort medium") != 7 ||
		!strings.Contains(graph, "stories ") {
		t.Errorf("analysis and epics calls, stories with the epics step's agent, want 7:\n%s", graph)
	}
}

func TestRunFailsAtAnalysisWritingNothingLater(t *testing.T) {
	log := runRepo(t, map[string]string{"rules.md": unique})
	triageOn(t, `["market"]`, `[]`)
	t.Setenv("FAKE_BAD", "flows")
	code, _, errOut := dulidu(t, "run", "brief.md", "--attach", "rules.md")
	if code != 1 || !strings.Contains(errOut, "failed at analysis: flows") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	dir := runs(t)[0]
	if got := read(t, log+".graph"); strings.Count(got, "\n") != 3 || strings.Contains(got, "constraints") {
		t.Errorf("calls after the failing list:\n%s", got)
	}
	for _, name := range []string{"analysis.json", "epics.json", "spec.md", "tickets", "plan.json", "brief-diseno.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("%s was written after a failed analysis", name)
		}
	}
	var report reportFile
	decodeFile(t, filepath.Join(dir, "report.json"), &report)
	last := report.Steps[len(report.Steps)-1]
	if report.Status != "failed" || last.Name != "analysis" || last.Status != "failed at analysis: flows" {
		t.Errorf("report.json: %+v", report)
	}
}

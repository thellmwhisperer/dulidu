//go:build unix

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A triage with no research_dimensions list failed, moderate; the lists it no longer keeps change nothing.
func TestRunRequiresTriageListsAndDeliversPersistedContext(t *testing.T) {
	const valid = `{"complexity":"simple","epics":2,"reasoning":"One check.",` +
		`"research_dimensions":[],"context_present":[],"missing_questions":[],"project_type":"other"}`
	for _, tc := range []struct {
		name, from, to string
		passed         bool // classified, not failed into moderate
	}{
		{"all empty arrays", "", "", true},
		{"research omitted", `"research_dimensions":[],`, "", false},
		{"research null", `"research_dimensions":[],`, `"research_dimensions":null,`, false},
		{"context omitted", `"context_present":[],`, "", true},
		{"context null", `"context_present":[],`, `"context_present":null,`, true},
		{"questions omitted", `"missing_questions":[],`, "", true},
		{"questions null", `"missing_questions":[],`, `"missing_questions":null,`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const brief = "The bike exchange needs a seller identity check.\n"
			const attachment = "Only bicycles tagged red-74 can be listed.\n"
			calls := runRepo(t, map[string]string{"rules.md": attachment})
			if err := os.WriteFile("brief.md", []byte(brief), 0o644); err != nil {
				t.Fatal(err)
			}
			answer := valid
			if tc.from != "" {
				answer = strings.Replace(valid, tc.from, tc.to, 1)
			}
			triageWith(t, answer)
			code, _, errOut := dulidu(t, "run", "brief.md", "--attach", "rules.md")
			if code != 0 {
				t.Fatalf("exit %d, stderr %q", code, errOut)
			}
			dirs := runs(t)
			if len(dirs) != 1 {
				t.Fatalf("run folders: %v", dirs)
			}
			dir := dirs[0]
			context := read(t, filepath.Join(dir, "contexto.md"))
			prompt := read(t, calls+".prompt")
			if !strings.Contains(context, brief) || !strings.Contains(context, attachment) || !strings.Contains(prompt, context) {
				t.Errorf("agent did not receive the persisted contexto.md verbatim")
			}
			if read(t, filepath.Join(dir, "input", "brief.md")) != brief || read(t, filepath.Join(dir, "input", "rules.md")) != attachment {
				t.Error("input files changed")
			}
			var report reportFile
			decodeFile(t, filepath.Join(dir, "report.json"), &report)
			// A triage, classified or failed, goes on to research, analysis and epics.
			if len(report.Steps) != 6 || report.Steps[0].Status != "done" {
				t.Fatalf("report steps: %+v", report)
			}
			var triage triageFile
			decodeFile(t, filepath.Join(dir, "triage.json"), &triage)
			if tc.passed {
				if report.Status != "done" || report.Steps[1].Status != "passed" || report.Steps[1].Note != "" ||
					triage.ResearchDimensions == nil || triage.ContextPresent != nil || triage.MissingQuestions != nil {
					t.Errorf("empty arrays did not pass: report=%+v triage=%+v", report, triage)
				}
				return
			}
			if report.Status != "done" || report.Steps[1].Status != "passed" ||
				report.Steps[1].Note != "classification failed, moderate: invalid output" {
				t.Errorf("invalid output report: %+v", report)
			}
			if triage.Complexity != "moderate" || triage.Epics != 5 || triage.ResearchDimensions == nil {
				t.Errorf("invalid output triage.json: %+v", triage)
			}
			for _, name := range []string{"triage.json", "research.json", "analysis.json", "epics.json", "spec.md"} {
				if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
					t.Errorf("%s was not written after a failed triage: %v", name, err)
				}
			}
		})
	}
}

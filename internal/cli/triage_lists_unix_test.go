//go:build unix

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRequiresTriageListsAndDeliversPersistedContext(t *testing.T) {
	const valid = `{"complexity":"simple","caps":{"epics":2,"stories_per_epic":3},` +
		`"research_dimensions":[],"context_present":[],"missing_questions":[],"project_type":"other"}`
	for _, tc := range []struct {
		name, from, to string
		passed         bool
	}{
		{"all empty arrays", "", "", true},
		{"research omitted", `"research_dimensions":[],`, "", false},
		{"research null", `"research_dimensions":[],`, `"research_dimensions":null,`, false},
		{"context omitted", `"context_present":[],`, "", false},
		{"context null", `"context_present":[],`, `"context_present":null,`, false},
		{"questions omitted", `"missing_questions":[],`, "", false},
		{"questions null", `"missing_questions":[],`, `"missing_questions":null,`, false},
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
			if tc.passed && code != 0 || !tc.passed && (code != 1 || !strings.Contains(errOut, "failed at triage: invalid output")) {
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
			if len(report.Steps) != 2 || report.Steps[0].Status != "done" {
				t.Fatalf("report steps: %+v", report)
			}
			if tc.passed {
				var triage triageFile
				decodeFile(t, filepath.Join(dir, "triage.json"), &triage)
				if report.Status != "done" || report.Steps[1].Status != "passed" ||
					triage.ResearchDimensions == nil || triage.ContextPresent == nil || triage.MissingQuestions == nil {
					t.Errorf("empty arrays did not pass: report=%+v triage=%+v", report, triage)
				}
				return
			}
			if report.Status != "failed" || report.Steps[1].Status != "failed at triage: invalid output" {
				t.Errorf("invalid output report: %+v", report)
			}
			for _, name := range []string{"triage.json", "research.json", "analysis.json", "epics.json", "spec.md"} {
				if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
					t.Errorf("%s exists or could not be inspected: %v", name, err)
				}
			}
		})
	}
}

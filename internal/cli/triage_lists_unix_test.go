//go:build unix

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Removed lists cannot change classification; research always asks all five dimensions. The answers are plain text,
// each removed list a line triage does not read.
func TestRunIgnoresRemovedTriageListsAndDeliversPersistedContext(t *testing.T) {
	const valid = "Complexity: simple\nEpics: 2\n" +
		"Research dimensions:\nContext present:\nMissing questions:\nProject type: other\n\nOne check.\n"
	for _, tc := range []struct {
		name, from, to string
		passed         bool // classified, not failed into moderate
	}{
		{"all empty arrays", "", "", true},
		{"research omitted", "Research dimensions:\n", "", true},
		{"invalid complexity", "Complexity: simple", "Complexity: huge", false},
		{"research invalid", "Research dimensions:\n", "Research dimensions: legal\n", true},
		{"research duplicate", "Research dimensions:\n", "Research dimensions: users, users\n", true},
		{"research null", "Research dimensions:\n", "Research dimensions: null\n", true},
		{"context omitted", "Context present:\n", "", true},
		{"context null", "Context present:\n", "Context present: null\n", true},
		{"questions omitted", "Missing questions:\n", "", true},
		{"questions null", "Missing questions:\n", "Missing questions: null\n", true},
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
			if len(report.Steps) != 6 || report.Steps[0].Status != "passed" {
				t.Fatalf("report steps: %+v", report)
			}
			var triage triageFile
			decodeFile(t, filepath.Join(dir, "triage.json"), &triage)
			if n := strings.Count(read(t, calls+".research"), "\n"); n != 5 {
				t.Errorf("research calls: %d, want five regardless of removed lists", n)
			}
			if strings.Contains(read(t, filepath.Join(dir, "triage.json")), "research_dimensions") {
				t.Error("triage.json still chooses research dimensions")
			}
			if tc.passed {
				if report.Status != "done" || report.Steps[1].Status != "passed" || report.Steps[1].Note != "" ||
					triage.ResearchDimensions != nil || triage.ContextPresent != nil || triage.MissingQuestions != nil {
					t.Errorf("empty arrays did not pass: report=%+v triage=%+v", report, triage)
				}
				return
			}
			if report.Status != "done" || report.Steps[1].Status != "passed" ||
				report.Steps[1].Note != "classification failed, moderate: unrecognized classification" {
				t.Errorf("unrecognized classification report: %+v", report)
			}
			if triage.Complexity != "moderate" || triage.Epics != 5 || triage.ResearchDimensions != nil {
				t.Errorf("unrecognized classification triage.json: %+v", triage)
			}
			for _, name := range []string{"triage.json", "research.json", "analysis.json", "epics.json", "spec.md"} {
				if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
					t.Errorf("%s was not written after a failed triage: %v", name, err)
				}
			}
		})
	}
}

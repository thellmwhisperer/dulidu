//go:build unix

package pipeline_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// TestBinaryRunNotesUncoveredMust runs the built dulidu with must functionalities F1, F2 and F5 where only F1 has a
// story: the run is done and the epics step's note names F2 and F5.
func TestBinaryRunNotesUncoveredMust(t *testing.T) {
	bin := binary(t)
	repo := realTempDir(t)
	t.Setenv("HOME", t.TempDir())
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "brief.md"), []byte(binaryRead(t, filepath.Join(binaryExample, "brief.md"))), 0o644); err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	answer := func(name, body string) string {
		path := filepath.Join(scratch, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	recorded := func(name string) string { return filepath.Join(binaryExample, "answers", name) }
	var answers []string
	for _, name := range []string{"02-triage.md", "03-research-business.md", "04-research-users.md", "05-research-market.md",
		"06-research-technical.md", "07-research-project.md", "08-analysis-personas.md"} {
		answers = append(answers, recorded(name))
	}
	functionality := func(id, title, priority string) string {
		return "## " + id + " " + title + "\nPriority: " + priority + "\n" + title + ".\n\n"
	}
	answers = append(answers, answer("functionalities.md", functionality("F1", "Book a wheel", "must")+
		functionality("F2", "Drop off for the kiln", "must")+functionality("F3", "Waiting list", "could")+
		functionality("F4", "Glaze shelf", "could")+functionality("F5", "Pay the month", "must")))
	for _, name := range []string{"10-analysis-flows.md", "11-analysis-constraints.md", "12-analysis-questions.md",
		"13-epics-outlines.md", "14-epics-E1.md"} {
		answers = append(answers, recorded(name))
	}
	answers = append(answers, answer("stories-E2.md", strings.Replace(binaryRead(t, recorded("15-epics-E2.md")),
		"Functionalities: F2", "Functionalities: F3", 1)))
	answers = append(answers, binaryDesign...)
	replay := fakeReplay(t, answers)
	for _, args := range [][]string{{"init"}, {"run", "brief.md"}} {
		cmd := exec.Command(bin, args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("dulidu %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	if n := binaryLines(filepath.Join(replay, "calls")); n != len(answers) {
		t.Errorf("claude was called %d times, want %d", n, len(answers))
	}
	dirs, _ := filepath.Glob(filepath.Join(repo, ".dulidu", "runs", "*"))
	if len(dirs) != 1 {
		t.Fatalf("run folders: %v", dirs)
	}
	r, err := pipeline.Read(dirs[0])
	if err != nil {
		t.Fatal(err)
	}
	if r.Report.Status != "done" || len(r.Epics) != 2 {
		t.Fatalf("run: %+v, epics: %+v", r.Report, r.Epics)
	}
	for _, s := range r.Report.Steps {
		if s.Name == "epics" && (s.Status != "passed" || s.Note != "uncovered must: F2, F5") {
			t.Errorf("epics step %+v, want passed with note %q", s, "uncovered must: F2, F5")
		}
	}
}

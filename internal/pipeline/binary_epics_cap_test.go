//go:build unix

package pipeline_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// TestBinaryRunTrimsOverCap runs the built dulidu with a simple triage, cap 3, max_stories_per_epic 2, four outlines
// where E2 depends on E4 and three stories for E1: the run keeps the first three epics and E1's first two stories,
// drops E2's dependency on the cut E4, and says what it cut in the epics step's note.
func TestBinaryRunTrimsOverCap(t *testing.T) {
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
	story := func(id, f string) string {
		return "## " + id + " Story " + id + "\nFunctionalities: " + f + "\nAs Ana I want " + id + " so that it works\n" +
			"### Acceptance\n- Given " + id + ", when it runs, then it works\n### First test\nWrite a test for " + id + ".\n\n"
	}
	answers := []string{answer("triage.md", "Complexity: simple\n\nOne studio, its members and its manager, no integrations.\n")}
	for _, name := range []string{"03-research-business.md", "04-research-users.md", "05-research-market.md",
		"06-research-technical.md", "07-research-project.md", "08-analysis-personas.md", "09-analysis-functionalities.md",
		"10-analysis-flows.md", "11-analysis-constraints.md", "12-analysis-questions.md"} {
		answers = append(answers, recorded(name))
	}
	answers = append(answers,
		answer("outlines.md", "## E1 Book a wheel\nDepends on: none\nBook.\n\n## E2 Fire\nDepends on: E1, E4\nFire.\n\n"+
			"## E3 Glaze\nDepends on: E1\nGlaze.\n\n## E4 Sell\nDepends on: none\nSell.\n"),
		answer("stories-E1.md", story("US1", "F1")+story("US2", "F1")+story("US3", "F1")),
		recorded("15-epics-E2.md"),
		recorded("15-epics-E2.md"))
	replay := fakeReplay(t, answers)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("dulidu %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init")
	yml := filepath.Join(repo, "dulidu.yaml")
	edited := regexp.MustCompile(`(?m)^max_stories_per_epic:.*$`).ReplaceAllString(binaryRead(t, yml), "max_stories_per_epic: 2")
	if err := os.WriteFile(yml, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	run("run", "brief.md")

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
	var ids []string
	for _, e := range r.Epics {
		ids = append(ids, e.ID)
	}
	if !slices.Equal(ids, []string{"E1", "E2", "E3"}) {
		t.Fatalf("epics %v, want E1 E2 E3", ids)
	}
	if len(r.Epics[0].Stories) != 2 || slices.Contains(r.Epics[1].DependsOn, "E4") {
		t.Errorf("E1 stories %d, want 2; E2 depends on %v, want no E4", len(r.Epics[0].Stories), r.Epics[1].DependsOn)
	}
	for _, s := range r.Report.Steps {
		if s.Name == "epics" && (!strings.Contains(s.Note, "kept 3 of 4 epics") || !strings.Contains(s.Note, "kept 2 of 3 stories in E1")) {
			t.Errorf("epics note %q", s.Note)
		}
	}
	prompt := r.Logs["epics"]
	if !strings.Contains(prompt, "EXACTLY 3") || strings.Contains(prompt, "Never drop a must functionality") {
		t.Errorf("outlines prompt must say EXACTLY 3 and not Never drop a must functionality:\n%s", prompt)
	}
}

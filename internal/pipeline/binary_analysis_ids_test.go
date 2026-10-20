//go:build unix

package pipeline_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// TestBinaryAnalysisKeepsModelIDs runs the built dulidu with functionalities F7 and F9, two personas with goals and
// pain points, a flow FL1 of three steps and a story naming F9: analysis.json holds the model's objects and ids, the
// outlines prompt lists "- [must] F7 <name>: <description>", and the story and its ticket's scope name F9.
func TestBinaryAnalysisKeepsModelIDs(t *testing.T) {
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
	answers = append(answers,
		answer("functionalities.md", "## F7 Book a wheel\nPriority: must\nOne hour, one member per wheel\n\n"+
			"## F9 Drop off for the kiln\nPriority: should\nPieces in before Friday 6 pm make that week's firing\n"),
		answer("flows.md", "## FL1 Booking\n1. Pick a free hour\n2. Pick a wheel\n3. Confirm\n"),
		recorded("11-analysis-constraints.md"), recorded("12-analysis-questions.md"), recorded("13-epics-outlines.md"),
		answer("stories-E1.md", strings.Replace(binaryRead(t, recorded("14-epics-E1.md")), "Functionalities: F1", "Functionalities: F9", 1)),
		answer("stories-E2.md", strings.Replace(binaryRead(t, recorded("15-epics-E2.md")), "Functionalities: F2", "Functionalities: F7", 1)))
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

	var stored any
	if err := json.Unmarshal([]byte(binaryRead(t, filepath.Join(dirs[0], "analysis.json"))), &stored); err != nil {
		t.Fatal(err)
	}
	var want any
	if err := json.Unmarshal([]byte(`{
		"personas": [
			{"name": "Ana", "role": "member", "goals": ["Throw on a wheel after work"],
				"pain_points": ["Every wheel taken when she arrives"]},
			{"name": "Luis", "role": "studio manager", "goals": ["Fill Friday's kiln"],
				"pain_points": ["Pieces dropped off after the firing is loaded"]}],
		"functionalities": [
			{"id": "F7", "name": "Book a wheel", "description": "One hour, one member per wheel", "priority": "must"},
			{"id": "F9", "name": "Drop off for the kiln", "description": "Pieces in before Friday 6 pm make that week's firing",
				"priority": "should"}],
		"flows": [{"id": "FL1", "name": "Booking", "steps": ["Pick a free hour", "Pick a wheel", "Confirm"]}],
		"constraints": ["The kiln fires once a week, on Friday at 6 pm", "Bookings are free: members pay a monthly fee"],
		"questions": ["How many wheels does the studio have?"]}`), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(stored, want) {
		t.Errorf("analysis.json:\n%v\nwant\n%v", stored, want)
	}

	r, err := pipeline.Read(dirs[0])
	if err != nil {
		t.Fatal(err)
	}
	if r.Report.Status != "done" || len(r.Epics) != 2 || len(r.Epics[0].Stories) == 0 {
		t.Fatalf("run: %+v, epics: %+v", r.Report, r.Epics)
	}
	if !strings.Contains(r.Logs["epics"], "\n- [must] F7 Book a wheel: One hour, one member per wheel\n") {
		t.Errorf("outlines prompt does not list F7 with the model's id:\n%s", r.Logs["epics"])
	}
	if got := r.Epics[0].Stories[0].Functionalities; !slices.Equal(got, []string{"F9"}) {
		t.Errorf("story %s functionalities %v, want [F9]", r.Epics[0].Stories[0].ID, got)
	}
	ticket := binaryRead(t, filepath.Join(dirs[0], "tickets", "1.md"))
	_, scope, _ := strings.Cut(ticket, "## Scope surface\n")
	scope, _, _ = strings.Cut(scope, "## ")
	if !strings.Contains(scope, "F9") || !strings.Contains(scope, "Drop off for the kiln") {
		t.Errorf("ticket 1's scope does not name F9:\n%s", ticket)
	}
}

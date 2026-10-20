//go:build unix

package pipeline_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
)

// TestBinaryAnalysisReadsPlainText runs the built dulidu on the example with a claude that replays the recorded
// answers, the five analysis ones plain text: analysis passes and analysis.json holds the items, with their ids, that
// read as the one-line items the JSON answers made. Each analysis prompt names its form, with an example, and never says JSON.
func TestBinaryAnalysisReadsPlainText(t *testing.T) {
	bin := binary(t)
	repo := realTempDir(t)
	t.Setenv("HOME", t.TempDir())
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "brief.md"), []byte(binaryRead(t, filepath.Join(binaryExample, "brief.md"))), 0o644); err != nil {
		t.Fatal(err)
	}
	recorded, _ := filepath.Glob(filepath.Join(binaryExample, "answers", "*-analysis-*"))
	if len(recorded) != 5 {
		t.Fatalf("recorded analysis answers: %v", recorded)
	}
	for _, path := range recorded {
		if answer := binaryRead(t, path); json.Valid([]byte(answer)) {
			t.Errorf("%s is still JSON:\n%s", path, answer)
		}
	}
	answers, _ := filepath.Glob(filepath.Join(binaryExample, "answers", "*"))
	answers = answers[1:] // the brief alone has no image, so the input step asks nothing
	replay := fakeReplay(t, answers)
	for _, args := range [][]string{{"init"}, {"run", "brief.md"}} {
		cmd := exec.Command(bin, args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("dulidu %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	dirs, _ := filepath.Glob(filepath.Join(repo, ".dulidu", "runs", "*"))
	if len(dirs) != 1 {
		t.Fatalf("run folders: %v", dirs)
	}
	var stored pipeline.Analysis
	if err := json.Unmarshal([]byte(binaryRead(t, filepath.Join(dirs[0], "analysis.json"))), &stored); err != nil {
		t.Fatalf("analysis.json must remain JSON: %v", err)
	}
	want := map[string][]string{
		"personas": {"Ana (member). Goals: Throw on a wheel after work. Pain points: Every wheel taken when she arrives",
			"Luis (studio manager). Goals: Fill Friday's kiln. Pain points: Pieces dropped off after the firing is loaded"},
		"functionalities": {"F1 [must] Book a wheel: One hour, one member per wheel",
			"F2 [must] Drop off for the kiln: Pieces in before Friday 6 pm make that week's firing",
			"F3 [could] Waiting list: Told when a wheel frees up"},
		"flows": {"FL1 Booking: Pick a free hour → Pick a wheel → Confirm",
			"FL2 Firing: Drop a piece off → Kiln fires on Friday → Collect"},
		"constraints": {"The kiln fires once a week, on Friday at 6 pm", "Bookings are free: members pay a monthly fee"},
		"questions":   {"How many wheels does the studio have?"},
	}
	got := map[string][]string{"personas": pipeline.Lines(stored.Personas), "constraints": stored.Constraints,
		"questions": stored.Questions}
	for _, f := range stored.Functionalities {
		got["functionalities"] = append(got["functionalities"], f.ID+" "+f.String())
	}
	for _, f := range stored.Flows {
		got["flows"] = append(got["flows"], f.ID+" "+f.String())
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("analysis.json: %+v, want %+v", got, want)
	}
	r, err := pipeline.Read(dirs[0])
	if err != nil {
		t.Fatal(err)
	}
	if r.Report.Status != "done" || r.Report.Steps[3].Name != "analysis" || r.Report.Steps[3].Status != "passed" ||
		binaryLines(filepath.Join(replay, "calls")) != len(answers) {
		t.Errorf("run did not finish after plain-text analysis: %+v", r.Report)
	}
	data := map[string]any{"Input": "A brief.", "Research": "{}", "Personas": "- P", "Functionalities": "- F",
		"Flows": "- FL", "Constraints": "- C"}
	for tmpl, form := range map[string][]string{
		"p09-analysis-personas":        {"## ", "Role: ", "### Goals", "### Pain points", "\n- "},
		"p10-analysis-functionalities": {"## F1 ", "## F2 ", "Priority: must | should | could", "prose"},
		"p11-analysis-flows":           {"## FL1 ", "FL2", "numbered list", "\n1. ", "\n2. "},
		"p12-analysis-constraints":     {"\n- ", "Example:"},
		"p13-analysis-questions":       {"\n- ", "Example:"},
	} {
		prompt, err := prompts.Render(tmpl, data)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToLower(prompt), "json") {
			t.Errorf("%s still says JSON:\n%s", tmpl, prompt)
		}
		for _, s := range append(form, "Example:") {
			if !strings.Contains(prompt, s) {
				t.Errorf("%s lacks %q in its form or example:\n%s", tmpl, s, prompt)
			}
		}
	}
}

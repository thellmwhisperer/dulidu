//go:build unix

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
)

// triageWith makes the fake claude answer the triage prompt with answer, the model's plain text.
func triageWith(t *testing.T, answer string) {
	t.Helper()
	line, err := json.Marshal(map[string]string{"type": "result", "result": answer})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "triage")
	if err := os.WriteFile(path, append(line, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_TRIAGE", path)
}

// triageFile is triage.json as the declared schema has it, and the fields it no longer has, absent.
type triageFile struct {
	Complexity string
	Epics      int
	Reasoning  string
	Caps       struct {
		Epics          int
		StoriesPerEpic int `json:"stories_per_epic"`
	}
	ResearchDimensions []string `json:"research_dimensions"`
	ContextPresent     []string `json:"context_present"`
	MissingQuestions   []string `json:"missing_questions"`
	ProjectType        string   `json:"project_type"`
}

// reportFile is report.json with each step's note.
type reportFile struct {
	Status string
	Steps  []struct{ Name, Status, Model, Harness, Effort, Note string }
}

func decodeFile(t *testing.T, path string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(read(t, path)), v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

// A plain-text triage of 12 complex epics is capped at 8 and clamped to max_epics 6.
func TestRunTriagesContextoOnceAndClampsItsCaps(t *testing.T) {
	png := read(t, filepath.Join("..", "pipeline", "input", "testdata", "pattern.png"))
	calls := runRepo(t, map[string]string{"notes.md": "Spain first.\n", "pattern.png": png})
	yml := strings.Replace(read(t, "dulidu.yaml"), "max_epics:\n", "max_epics: 6\n", 1)
	yml = strings.Replace(yml, "max_stories_per_epic:\n", "max_stories_per_epic: 4\n", 1)
	if err := os.WriteFile("dulidu.yaml", []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	triageWith(t, "Complexity: complex\nEpics: 12\n"+
		"Research dimensions: market, users\nContext present: brief, notes.md, pattern.png\n"+
		"Missing questions:\nProject type: new_product\n\nMany markets.\n")
	if code, out, errOut := dulidu(t, "run", "brief.md", "--attach", "notes.md", "--attach", "pattern.png"); code != 0 {
		t.Fatalf("dulidu run: exit %d\n%s%s", code, out, errOut)
	}
	dirs := runs(t)
	if len(dirs) != 1 {
		t.Fatalf("want 1 run folder, got %v", dirs)
	}
	dir := dirs[0]
	if got := read(t, calls); strings.Count(got, "\n") != 1 || !strings.Contains(got, "--model haiku") {
		t.Errorf("want one input description, got %q", got)
	}
	if got := read(t, calls+".triage"); strings.Count(got, "\n") != 1 || !strings.Contains(got, "--model opus") ||
		!strings.Contains(got, "--effort high") {
		t.Errorf("want one call to the triage step's model, got %q", got)
	}
	want, err := prompts.Render("triage", map[string]any{"Context": read(t, filepath.Join(dir, "contexto.md"))})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(want, "research_dimensions") {
		t.Error("triage prompt still asks for dimensions")
	}
	if got := read(t, calls+".prompt"); got != want {
		t.Errorf("triage prompt is not contexto.md's:\n%s", got)
	}

	var triage triageFile
	decodeFile(t, filepath.Join(dir, "triage.json"), &triage)
	// complex caps 12 at 8, max_epics at 6; the stories cap is max_stories_per_epic's, in the stories prompt.
	if triage.Epics != 6 || triage.Caps.Epics != 0 || triage.Caps.StoriesPerEpic != 0 || triage.Complexity != "complex" ||
		triage.ResearchDimensions != nil || triage.ProjectType != "" ||
		triage.ContextPresent != nil || triage.MissingQuestions != nil || triage.Reasoning != "Many markets." {
		t.Errorf("triage.json: %+v", triage)
	}
	if !strings.Contains(read(t, filepath.Join(dir, "agent", "epics.log")), "At most 4, but") {
		t.Error("the stories prompt lacks max_stories_per_epic's cap")
	}
	var report reportFile
	decodeFile(t, filepath.Join(dir, "report.json"), &report)
	if report.Status != "done" || len(report.Steps) != 6 || report.Steps[0].Name != "input" ||
		report.Steps[0].Status != "passed" || report.Steps[1].Name != "triage" || report.Steps[1].Status != "passed" ||
		report.Steps[1].Model != "opus" || report.Steps[1].Harness != "claude" ||
		report.Steps[1].Note != "clamped epics from 8 to max_epics 6" || report.Steps[2].Name != "research" {
		t.Errorf("report.json: %+v", report)
	}
	if read(t, filepath.Join(dir, "input", "pattern.png")) != png || read(t, filepath.Join(dir, "input", "notes.md")) != "Spain first.\n" {
		t.Error("the input files did not survive triage")
	}
	if !strings.Contains(read(t, filepath.Join(dir, "agent", "input.log")), "input/pattern.png") ||
		!strings.Contains(read(t, filepath.Join(dir, "agent", "triage.log")), "Epics: 12\n") {
		t.Error("agent logs lack the input description or the triage answer")
	}

	before := map[string]string{}
	for _, name := range []string{"triage.json", "report.json", "contexto.md", "research.json"} {
		before[name] = read(t, filepath.Join(dir, name))
	}
	r, err := pipeline.Read(dir)
	if err == nil {
		err = pipeline.Write(dir, r)
	}
	if err != nil {
		t.Fatal(err)
	}
	for name, b := range before {
		if read(t, filepath.Join(dir, name)) != b {
			t.Errorf("%s changed through Read and Write", name)
		}
	}
}

func TestRunGoesOnAfterTriageWithMissingQuestions(t *testing.T) {
	calls := runRepo(t, map[string]string{})
	triageWith(t, "Complexity: simple\nEpics: 2\nResearch dimensions:\n"+
		"Context present: brief\nMissing questions:\n- Who sells the bikes?\nProject type: other\n\nOne shop.\n")
	if code, out, errOut := dulidu(t, "run", "brief.md"); code != 0 {
		t.Fatalf("dulidu run: exit %d\n%s%s", code, out, errOut)
	}
	dir := runs(t)[0]
	var triage triageFile
	decodeFile(t, filepath.Join(dir, "triage.json"), &triage)
	if triage.MissingQuestions != nil || triage.ProjectType != "" || triage.Epics != 2 {
		t.Errorf("triage.json: %+v", triage)
	}
	var report reportFile
	decodeFile(t, filepath.Join(dir, "report.json"), &report)
	if report.Status != "done" || len(report.Steps) != 6 || report.Steps[1].Name != "triage" || report.Steps[1].Status != "passed" {
		t.Errorf("report.json: %+v", report)
	}
	if got := read(t, calls+".triage"); strings.Count(got, "\n") != 1 {
		t.Errorf("want one triage call, got %q", got)
	}
	if _, err := os.Stat(calls); err == nil {
		t.Error("a text brief called the agent for a description")
	}
	if _, err := os.Stat(filepath.Join(dir, "questions.json")); err == nil {
		t.Error("questions.json was written")
	}
	for _, name := range []string{"research.json", "analysis.json", "epics.json", "spec.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s was not written after triage: %v", name, err)
		}
	}
}

// An invalid triage answer is a failed triage: moderate, 5 epics, and the run goes on.
func TestRunGoesOnModerateAfterInvalidTriage(t *testing.T) {
	for name, answer := range map[string]string{"empty": "{}", "malformed": `{"complexity": "simple",`, "prose": "Simple."} {
		t.Run(name, func(t *testing.T) {
			calls := runRepo(t, map[string]string{"notes.md": "Spain first.\n"})
			triageWith(t, answer)
			code, _, errOut := dulidu(t, "run", "brief.md", "--attach", "notes.md")
			if code != 0 {
				t.Fatalf("exit %d, stderr %q", code, errOut)
			}
			dir := runs(t)[0]
			var report reportFile
			decodeFile(t, filepath.Join(dir, "report.json"), &report)
			if report.Status != "done" || len(report.Steps) != 6 || report.Steps[0].Status != "passed" ||
				report.Steps[1].Status != "passed" || report.Steps[1].Note != "classification failed, moderate: unrecognized classification" {
				t.Errorf("report.json: %+v", report)
			}
			var triage triageFile
			if decodeFile(t, filepath.Join(dir, "triage.json"), &triage); triage.Complexity != "moderate" || triage.Epics != 5 {
				t.Errorf("triage.json: %+v", triage)
			}
			if read(t, filepath.Join(dir, "input", "notes.md")) != "Spain first.\n" ||
				!strings.Contains(read(t, filepath.Join(dir, "contexto.md")), "Spain first.") {
				t.Error("the input did not survive a failed triage")
			}
			if got := read(t, calls+".triage"); strings.Count(got, "\n") != 1 {
				t.Errorf("want one triage call, got %q", got)
			}
		})
	}
}

func TestRunRejectsInputBeforeTriage(t *testing.T) {
	calls := runRepo(t, map[string]string{"a/notes.md": "one\n", "b/notes.md": "two\n"})
	if code, _, _ := dulidu(t, "run", "brief.md", "--attach", "a/notes.md", "--attach", "b/notes.md"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if _, err := os.Stat(calls + ".triage"); err == nil {
		t.Error("triage ran on a rejected input")
	}
}

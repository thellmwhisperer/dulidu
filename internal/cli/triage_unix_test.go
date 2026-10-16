//go:build unix

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
)

// triageWith makes the fake claude answer the triage prompt with answer.
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

// triageFile is triage.json as the declared schema has it.
type triageFile struct {
	Complexity string
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
	Steps  []struct{ Name, Status, Model, Harness, Note string }
}

func decodeFile(t *testing.T, path string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(read(t, path)), v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func TestRunTriagesContextoOnceAndClampsItsCaps(t *testing.T) {
	png := read(t, filepath.Join("..", "pipeline", "input", "testdata", "pattern.png"))
	calls := runRepo(t, map[string]string{"notes.md": "Spain first.\n", "pattern.png": png})
	yml := strings.Replace(read(t, "dulidu.yaml"), "max_epics:\n", "max_epics: 8\n", 1)
	if err := os.WriteFile("dulidu.yaml", []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	triageWith(t, `{"complexity":"complex","caps":{"epics":12,"stories_per_epic":4},`+
		`"research_dimensions":["market","users"],"context_present":["brief","notes.md","pattern.png"],`+
		`"missing_questions":[],"project_type":"new_product"}`)
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
	if got := read(t, calls+".prompt"); got != want {
		t.Errorf("triage prompt is not contexto.md's:\n%s", got)
	}

	var triage triageFile
	decodeFile(t, filepath.Join(dir, "triage.json"), &triage)
	if triage.Caps.Epics != 8 || triage.Caps.StoriesPerEpic != 4 || triage.Complexity != "complex" ||
		!slices.Equal(triage.ResearchDimensions, []string{"market", "users"}) || triage.ProjectType != "new_product" ||
		len(triage.ContextPresent) != 3 || len(triage.MissingQuestions) != 0 {
		t.Errorf("triage.json: %+v", triage)
	}
	var report reportFile
	decodeFile(t, filepath.Join(dir, "report.json"), &report)
	if report.Status != "done" || len(report.Steps) != 3 || report.Steps[0].Name != "input" ||
		report.Steps[0].Status != "done" || report.Steps[1].Name != "triage" || report.Steps[1].Status != "passed" ||
		report.Steps[1].Model != "opus" || report.Steps[1].Harness != "claude" ||
		!strings.Contains(report.Steps[1].Note, "clamped") || report.Steps[2].Name != "research" {
		t.Errorf("report.json: %+v", report)
	}
	if read(t, filepath.Join(dir, "input", "pattern.png")) != png || read(t, filepath.Join(dir, "input", "notes.md")) != "Spain first.\n" {
		t.Error("the input files did not survive triage")
	}
	if !strings.Contains(read(t, filepath.Join(dir, "agent", "input.log")), "input/pattern.png") ||
		!strings.Contains(read(t, filepath.Join(dir, "agent", "triage.log")), `"epics":12`) {
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

func TestRunStopsAfterTriageWithMissingQuestions(t *testing.T) {
	calls := runRepo(t, map[string]string{})
	triageWith(t, `{"complexity":"simple","caps":{"epics":2,"stories_per_epic":3},"research_dimensions":[],`+
		`"context_present":["brief"],"missing_questions":["Who sells the bikes?"],"project_type":"other"}`)
	if code, out, errOut := dulidu(t, "run", "brief.md"); code != 0 {
		t.Fatalf("dulidu run: exit %d\n%s%s", code, out, errOut)
	}
	dir := runs(t)[0]
	var triage triageFile
	decodeFile(t, filepath.Join(dir, "triage.json"), &triage)
	if !slices.Equal(triage.MissingQuestions, []string{"Who sells the bikes?"}) || triage.ProjectType != "other" {
		t.Errorf("triage.json: %+v", triage)
	}
	var report reportFile
	decodeFile(t, filepath.Join(dir, "report.json"), &report)
	if len(report.Steps) != 2 || report.Steps[1].Name != "triage" || report.Steps[1].Status != "passed" {
		t.Errorf("report.json: %+v", report)
	}
	if got := read(t, calls+".triage"); strings.Count(got, "\n") != 1 {
		t.Errorf("want one triage call, got %q", got)
	}
	if _, err := os.Stat(calls); err == nil {
		t.Error("a text brief called the agent for a description")
	}
	for _, name := range []string{"research.json", "analysis.json", "epics.json", "spec.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("%s was written after triage", name)
		}
	}
}

func TestRunFailsAtTriageOnInvalidOutput(t *testing.T) {
	for name, answer := range map[string]string{"empty": "{}", "malformed": `{"complexity": "simple",`, "prose": "Simple."} {
		t.Run(name, func(t *testing.T) {
			calls := runRepo(t, map[string]string{"notes.md": "Spain first.\n"})
			triageWith(t, answer)
			code, _, errOut := dulidu(t, "run", "brief.md", "--attach", "notes.md")
			if code != 1 || !strings.Contains(errOut, "failed at triage: invalid output") {
				t.Fatalf("exit %d, stderr %q", code, errOut)
			}
			dir := runs(t)[0]
			var report reportFile
			decodeFile(t, filepath.Join(dir, "report.json"), &report)
			if report.Status != "failed" || len(report.Steps) != 2 || report.Steps[0].Status != "done" ||
				report.Steps[1].Status != "failed at triage: invalid output" {
				t.Errorf("report.json: %+v", report)
			}
			if _, err := os.Stat(filepath.Join(dir, "triage.json")); err == nil {
				t.Error("triage.json was written")
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
	calls := runRepo(t, map[string]string{"foo.xyz": "?"})
	if code, _, _ := dulidu(t, "run", "brief.md", "--attach", "foo.xyz"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if _, err := os.Stat(calls + ".triage"); err == nil {
		t.Error("triage ran on a rejected input")
	}
}

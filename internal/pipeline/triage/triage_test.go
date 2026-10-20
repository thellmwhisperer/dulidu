package triage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
)

var step = pipeline.Step{Name: "triage", Harness: "claude", Model: "opus"}

// start is a run folder whose input step is done with context as its contexto.md.
func start(t *testing.T, context string) (string, *pipeline.Run) {
	t.Helper()
	at := time.Now()
	r := &pipeline.Run{Report: pipeline.Report{Status: "running", Started: at,
		Steps: []pipeline.Step{{Name: "input", Status: "done", Harness: "claude", Started: at, Finished: at}}},
		Input: &pipeline.InputContext{Brief: pipeline.Brief{Text: "A brief.\n"}}, Context: context}
	dir := t.TempDir()
	if err := pipeline.Write(dir, *r); err != nil {
		t.Fatal(err)
	}
	return dir, r
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The three recorded briefs, answered by their recorded plain-text answers with max_epics 8: existing asks 12, complex caps it.
func TestRecordedBriefs(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range []string{"thin", "attachments", "existing"} {
		context, answer := read(t, filepath.Join("testdata", name+".md")), read(t, filepath.Join("testdata", name+".txt"))
		dir, r := start(t, context)
		want, err := prompts.Render("triage", map[string]any{"Context": context})
		if err != nil {
			t.Fatal(err)
		}
		calls := 0
		if err := Run(dir, r, step, 8, 0, func(prompt string) (string, error) {
			calls++
			if prompt != want {
				t.Errorf("%s: the prompt is not contexto.md's", name)
			}
			return answer, nil
		}); err != nil || calls != 1 {
			t.Fatalf("%s: %v after %d calls", name, err, calls)
		}
		seen[read(t, filepath.Join(dir, "triage.json"))] = true
		got, err := pipeline.Read(dir)
		if err != nil {
			t.Fatal(err)
		}
		tr, s := got.Triage, got.Report.Steps[1]
		if tr == nil || s.Name != "triage" || s.Status != "passed" || got.Report.Status != "done" || got.Context != context {
			t.Fatalf("%s: %+v", name, got)
		}
		switch name {
		case "thin":
			if tr.Complexity != "simple" || tr.Epics != 2 {
				t.Errorf("thin: %+v", tr)
			}
		case "existing":
			if tr.Complexity != "complex" || tr.Reasoning == "" {
				t.Errorf("existing: %+v", tr)
			}
		}
		epics := map[string]int{"thin": 2, "attachments": 5, "existing": 8}[name]
		if tr.Epics != epics || s.Note != "" ||
			tr.Reasoning == "" {
			t.Errorf("%s: epics %d, want %d, note %q", name, tr.Epics, epics, s.Note)
		}
	}
	if len(seen) != 3 {
		t.Errorf("%d different triage.json, want 3", len(seen))
	}
}

// max_epics clamps complex's 8; max_stories_per_epic goes on to the epics step, not into triage.json.
func TestMaxEpicsClampsTheComplexityCap(t *testing.T) {
	dir, r := start(t, "# Contexto\n")
	answer := "Complexity: complex\nEpics: 12\nResearch dimensions:\n\n" +
		"A migration of three systems.\n"
	if err := Run(dir, r, step, 5, 10, func(string) (string, error) { return answer, nil }); err != nil {
		t.Fatal(err)
	}
	got, err := pipeline.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Triage.Epics != 5 || got.Triage.Complexity != "complex" || got.Report.Steps[1].Note !=
		"clamped epics from 8 to max_epics 5" || r.Triage.StoriesPerEpic != 10 || got.Triage.StoriesPerEpic != 0 {
		t.Errorf("triage %+v, note %q", got.Triage, got.Report.Steps[1].Note)
	}
}

// A failed triage, an agent error or an invalid answer, is a moderate one and the run goes on.
func TestFailures(t *testing.T) {
	for name, c := range map[string]struct {
		answer string
		err    error
		want   string
	}{
		"empty":     {"{}", nil, "classification failed, moderate: invalid output"},
		"malformed": {`{"complexity": "simple",`, nil, "classification failed, moderate: invalid output"},
		"null":      {"null", nil, "classification failed, moderate: invalid output"},
		"fractional epics": {"Complexity: simple\nEpics: 2.5", nil,
			"classification failed, moderate: invalid output"},
		"bad complexity": {`{"complexity":"huge","epics":1,"reasoning":"Small."}`,
			nil, "classification failed, moderate: invalid output"},
		"agent": {"", errors.New("timed out"), "classification failed, moderate: timed out"},
	} {
		dir, r := start(t, "# Contexto\n")
		err := Run(dir, r, step, 0, 0, func(string) (string, error) { return c.answer, c.err })
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
		got, rerr := pipeline.Read(dir)
		if rerr != nil {
			t.Fatal(rerr)
		}
		if got.Triage == nil || got.Triage.Complexity != "moderate" || got.Triage.Epics != 5 ||
			got.Triage.Reasoning != c.want || got.Report.Status != "done" || got.Report.Steps[1].Status != "passed" ||
			got.Report.Steps[1].Note != c.want || got.Context != "# Contexto\n" || got.Logs["triage"] == "" {
			t.Errorf("%s: %+v", name, got)
		}
	}
}

// The removed dimension selector, a line triage no longer reads, cannot invalidate a classification or survive in triage.json.
func TestRemovedDimensionsDoNotChangeClassification(t *testing.T) {
	for _, value := range []string{`["legal"]`, `["users","users"]`, `null`, `[]`, `"ignored"`} {
		dir, r := start(t, "# Contexto\n")
		answer := "Complexity: simple\nEpics: 1\nResearch dimensions: " + value + "\n\nSmall.\n"
		if err := Run(dir, r, step, 0, 0, func(prompt string) (string, error) {
			if strings.Contains(prompt, "research_dimensions") {
				t.Error("triage prompt still asks for dimensions")
			}
			return answer, nil
		}); err != nil {
			t.Fatal(err)
		}
		if r.Triage.Complexity != "simple" || r.Triage.Epics != 1 || r.Report.Steps[1].Note != "" {
			t.Errorf("legacy field %s changed classification: %+v", value, r.Triage)
		}
		if strings.Contains(read(t, filepath.Join(dir, "triage.json")), "research_dimensions") {
			t.Error("triage.json still contains research_dimensions")
		}
	}
}

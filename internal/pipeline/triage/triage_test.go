package triage

import (
	"errors"
	"os"
	"path/filepath"
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

// The three recorded briefs, answered by their recorded answers with max_epics 8 and no story limit.
func TestRecordedBriefs(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range []string{"thin", "attachments", "existing"} {
		context, answer := read(t, filepath.Join("testdata", name+".md")), read(t, filepath.Join("testdata", name+".json"))
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
			if len(tr.MissingQuestions) == 0 {
				t.Error("thin: no missing questions")
			}
		case "existing":
			if tr.ProjectType != "existing_codebase" {
				t.Errorf("existing: project_type %s", tr.ProjectType)
			}
		}
		clamped := name == "existing"
		if clamped && (tr.Caps.Epics != 8 || tr.Caps.StoriesPerEpic != 7 || s.Note != "clamped caps.epics from 12 to max_epics 8") ||
			!clamped && s.Note != "" {
			t.Errorf("%s: caps %+v, note %q", name, tr.Caps, s.Note)
		}
	}
	if len(seen) != 3 {
		t.Errorf("%d different triage.json, want 3", len(seen))
	}
}

func TestBothCapsClamp(t *testing.T) {
	dir, r := start(t, "# Contexto\n")
	answer := `{"complexity":"complex","caps":{"epics":12,"stories_per_epic":20},"research_dimensions":[],` +
		`"context_present":["brief"],"missing_questions":[],"project_type":"migration"}`
	if err := Run(dir, r, step, 8, 10, func(string) (string, error) { return answer, nil }); err != nil {
		t.Fatal(err)
	}
	got, err := pipeline.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Triage.Caps != (pipeline.Caps{Epics: 8, StoriesPerEpic: 10}) || got.Report.Steps[1].Note !=
		"clamped caps.epics from 12 to max_epics 8; clamped caps.stories_per_epic from 20 to max_stories_per_epic 10" {
		t.Errorf("caps %+v, note %q", got.Triage.Caps, got.Report.Steps[1].Note)
	}
}

func TestFailures(t *testing.T) {
	for name, c := range map[string]struct {
		answer string
		err    error
		want   string
	}{
		"empty":     {"{}", nil, "failed at triage: invalid output"},
		"malformed": {`{"complexity": "simple",`, nil, "failed at triage: invalid output"},
		"null":      {"null", nil, "failed at triage: invalid output"},
		"bad dimension": {`{"complexity":"simple","caps":{"epics":1,"stories_per_epic":1},"research_dimensions":["legal"],` +
			`"context_present":[],"missing_questions":[],"project_type":"other"}`, nil, "failed at triage: invalid output"},
		"agent": {"", errors.New("timed out"), "failed at triage: timed out"},
	} {
		dir, r := start(t, "# Contexto\n")
		err := Run(dir, r, step, 0, 0, func(string) (string, error) { return c.answer, c.err })
		if err == nil || err.Error() != c.want {
			t.Errorf("%s: %v", name, err)
		}
		got, rerr := pipeline.Read(dir)
		if rerr != nil {
			t.Fatal(rerr)
		}
		if got.Triage != nil || got.Report.Status != "failed" || got.Report.Steps[1].Status != c.want ||
			got.Context != "# Contexto\n" || got.Logs["triage"] == "" {
			t.Errorf("%s: %+v", name, got)
		}
	}
}

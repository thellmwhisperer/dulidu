package research

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
)

var step = pipeline.Step{Name: "research", Harness: "claude", Model: "sonnet"}

const context = "# Contexto\n\nA bike exchange; only bicycles tagged red-74 can be listed.\n"

// start is a run folder whose triage passed, switching dimensions on.
func start(t *testing.T, dimensions ...string) (string, *pipeline.Run) {
	t.Helper()
	at := time.Now()
	r := &pipeline.Run{Report: pipeline.Report{Status: "done", Started: at, Steps: []pipeline.Step{
		{Name: "input", Status: "done", Harness: "claude", Started: at, Finished: at},
		{Name: "triage", Status: "passed", Harness: "claude", Started: at, Finished: at}}},
		Input: &pipeline.InputContext{Brief: pipeline.Brief{Text: "A brief.\n"}}, Context: context,
		Triage: &pipeline.Triage{Complexity: "simple", Epics: 2,
			ResearchDimensions: append([]string{}, dimensions...),
			Reasoning:          "One kind of user."}}
	dir := t.TempDir()
	if err := pipeline.Write(dir, *r); err != nil {
		t.Fatal(err)
	}
	return dir, r
}

// answer is a valid answer for dimension.
func answer(dimension string) string {
	return `{"text":"About ` + dimension + `.","sources":[{"url":"https://example.com/` + dimension +
		`","title":"On ` + dimension + `","supports":"About ` + dimension + `."}]}`
}

func TestThreeDimensionsMakeThreeCallsAndThreeEntries(t *testing.T) {
	dir, r := start(t, "market", "users", "technical")
	var asked []string
	if err := Run(dir, r, step, func(prompt string) (string, error) {
		asked = append(asked, prompt)
		return answer([]string{"market", "users", "technical"}[len(asked)-1]), nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(asked) != 3 {
		t.Fatalf("%d calls, want 3", len(asked))
	}
	for i, tmpl := range []string{"p05-research-market", "p04-research-users", "p06-research-technical"} {
		want, err := prompts.Render(tmpl, map[string]any{"Input": context})
		if err != nil {
			t.Fatal(err)
		}
		if asked[i] != want || !strings.Contains(asked[i], "red-74") {
			t.Errorf("call %d is not %s on contexto.md:\n%s", i+1, tmpl, asked[i])
		}
	}
	got, err := pipeline.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	var want []pipeline.Dimension
	for _, d := range []string{"market", "users", "technical"} {
		want = append(want, pipeline.Dimension{Name: d, Text: "About " + d + ".",
			Sources: []pipeline.Source{{URL: "https://example.com/" + d, Title: "On " + d, Supports: "About " + d + "."}}})
	}
	if got.Research == nil || !reflect.DeepEqual(got.Research.Dimensions, want) {
		t.Fatalf("research.json: %+v", got.Research)
	}
	s := got.Report.Steps[2]
	if got.Report.Status != "done" || len(got.Report.Steps) != 3 || s.Name != "research" || s.Status != "passed" ||
		s.Model != "sonnet" || s.Note != "dimensions: market, users, technical" || got.Triage == nil || got.Context != context {
		t.Errorf("report: %+v", got.Report)
	}
	if !strings.Contains(got.Logs["research"], answer("technical")) {
		t.Errorf("agent/research.log: %q", got.Logs["research"])
	}
}

func TestNoDimensionsMakeNoCall(t *testing.T) {
	dir, r := start(t)
	if err := Run(dir, r, step, func(string) (string, error) {
		t.Error("a call with no dimension switched on")
		return "", nil
	}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "research.json"))
	if err != nil || strings.Join(strings.Fields(string(b)), "") != `{"dimensions":[]}` {
		t.Fatalf("research.json: %s %v", b, err)
	}
	if got, err := pipeline.Read(dir); err != nil || got.Report.Steps[2].Status != "passed" {
		t.Fatalf("%+v %v", got.Report, err)
	}
}

func TestFailures(t *testing.T) {
	for name, c := range map[string]struct {
		answer string
		err    error
		want   string
	}{
		"empty sources": {`{"text":"Crowded.","sources":[]}`, nil, "failed at research: market: no sources"},
		"no sources":    {`{"text":"Crowded."}`, nil, "failed at research: market: no sources"},
		"no text":       {`{"text":" ","sources":[{"url":"u","title":"t","supports":"s"}]}`, nil, "failed at research: market: no text"},
		"blank title":   {`{"text":"Crowded.","sources":[{"url":"u","title":"","supports":"s"}]}`, nil, "failed at research: market: a source lacks url, title or supports"},
		"prose":         {"Crowded.", nil, "failed at research: market: invalid output"},
		"agent":         {"", errors.New("timed out"), "failed at research: market: timed out"},
	} {
		dir, r := start(t, "business", "market", "users")
		calls := 0
		err := Run(dir, r, step, func(string) (string, error) {
			calls++
			if calls == 1 {
				return answer("business"), nil
			}
			return c.answer, c.err
		})
		if err == nil || err.Error() != c.want || calls != 2 {
			t.Errorf("%s: %v after %d calls", name, err, calls)
		}
		got, rerr := pipeline.Read(dir)
		if rerr != nil {
			t.Fatal(rerr)
		}
		if got.Research != nil || got.Report.Status != "failed" || got.Report.Steps[2].Status != c.want ||
			got.Triage == nil || got.Context != context || got.Logs["research"] == "" {
			t.Errorf("%s: %+v", name, got)
		}
	}
}

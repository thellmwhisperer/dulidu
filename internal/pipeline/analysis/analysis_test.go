package analysis

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
)

var step = pipeline.Step{Name: "analysis", Harness: "claude", Model: "sonnet"}

const context = "# Contexto\n\nA bike exchange; only bicycles tagged red-74 can be listed.\n"

var names = []string{"personas", "functionalities", "flows", "constraints", "questions"}

// start is a run folder whose research passed.
func start(t *testing.T) (string, *pipeline.Run) {
	t.Helper()
	at := time.Now()
	r := &pipeline.Run{Report: pipeline.Report{Status: "done", Started: at, Steps: []pipeline.Step{
		{Name: "triage", Status: "passed", Harness: "claude", Started: at, Finished: at},
		{Name: "research", Status: "passed", Harness: "claude", Started: at, Finished: at}}},
		Context: context, Research: &pipeline.Research{Dimensions: []pipeline.Dimension{{Name: "market", Text: "Bike swaps grow.",
			Sources: []pipeline.Source{{URL: "https://example.com/m", Title: "Swaps", Supports: "Bike swaps grow."}}}}}}
	dir := t.TempDir()
	if err := pipeline.Write(dir, *r); err != nil {
		t.Fatal(err)
	}
	return dir, r
}

// recorded is the recorded answer for list.
func recorded(t *testing.T, list string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", list+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestARecordedSetMakesFiveListsInOrder(t *testing.T) {
	dir, r := start(t)
	var asked []string
	if err := Run(dir, r, step, func(prompt string) (string, error) {
		asked = append(asked, prompt)
		return recorded(t, names[len(asked)-1]), nil
	}); err != nil {
		t.Fatal(err)
	}
	research, err := os.ReadFile(filepath.Join(dir, "research.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := pipeline.Analysis{
		Personas: []string{"Rosa (seller). Goals: List a red-74 bike. Pain points: Untagged bikes get rejected",
			"Tom (buyer). Goals: Find a bike"},
		Functionalities: []string{"[must] List a bike: Only red-74 bikes", "[could] Search: Find bikes by size"},
		Flows:           []string{"Listing: Scan the tag → Add photos → Publish"},
		Constraints:     []string{"Only bicycles tagged red-74 can be listed"},
		Questions:       []string{"Who prints the red-74 tags?"},
	}
	data := map[string]any{"Input": context, "Research": strings.TrimSuffix(string(research), "\n"),
		"Personas": "- " + strings.Join(want.Personas, "\n- "), "Functionalities": "- " + strings.Join(want.Functionalities, "\n- "),
		"Flows": "- " + strings.Join(want.Flows, "\n- "), "Constraints": "- " + strings.Join(want.Constraints, "\n- ")}
	if len(asked) != 5 {
		t.Fatalf("%d calls, want 5", len(asked))
	}
	for i, tmpl := range []string{"p09-analysis-personas", "p10-analysis-functionalities", "p11-analysis-flows",
		"p12-analysis-constraints", "p13-analysis-questions"} {
		prompt, err := prompts.Render(tmpl, data)
		if err != nil {
			t.Fatal(err)
		}
		if asked[i] != prompt || !strings.Contains(asked[i], "red-74") || !strings.Contains(asked[i], "Bike swaps grow.") {
			t.Errorf("call %d is not %s on contexto.md, research.json and the lists before it:\n%s", i+1, tmpl, asked[i])
		}
	}
	if !strings.Contains(asked[4], "[must] List a bike") || strings.Contains(asked[1], "[must]") {
		t.Errorf("calls do not carry exactly the lists before them:\n%s", strings.Join(asked, "\n----\n"))
	}
	got, err := pipeline.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Analysis == nil || !reflect.DeepEqual(*got.Analysis, want) {
		t.Fatalf("analysis.json: %+v", got.Analysis)
	}
	s := got.Report.Steps[2]
	if got.Report.Status != "done" || len(got.Report.Steps) != 3 || s.Name != "analysis" || s.Status != "passed" ||
		s.Model != "sonnet" || s.Finished.IsZero() || got.Research == nil || got.Context != context {
		t.Errorf("report: %+v", got.Report)
	}
	if !strings.Contains(got.Logs["analysis"], recorded(t, "questions")) {
		t.Errorf("agent/analysis.log: %q", got.Logs["analysis"])
	}
}

func TestPersonaEmptyNestedListsAllowed(t *testing.T) {
	dir, r := start(t)
	calls := 0
	if err := Run(dir, r, step, func(string) (string, error) {
		calls++
		if calls == 1 {
			return `[{"name":"Rosa","role":"seller","goals":[],"painPoints":[]}]`, nil
		}
		return recorded(t, names[calls-1]), nil
	}); err != nil {
		t.Fatal(err)
	}
	got, err := pipeline.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 5 || got.Analysis == nil || !reflect.DeepEqual(got.Analysis.Personas, []string{"Rosa (seller)"}) {
		t.Errorf("empty persona lists: %d calls, analysis %+v", calls, got.Analysis)
	}
}

func TestFailures(t *testing.T) {
	for name, c := range map[string]struct {
		list, answer string
		err          error
		want         string
	}{
		"flows object":         {"flows", `{}`, nil, "flows: invalid output"},
		"no priority":          {"functionalities", `[{"id":"f1","name":"List","description":"A bike"}]`, nil, "functionalities: functionality List: priority not must, should or could"},
		"bad priority":         {"functionalities", `[{"id":"f1","name":"List","description":"A bike","priority":"won't"}]`, nil, "functionalities: functionality List: priority not must, should or could"},
		"empty questions":      {"questions", `[]`, nil, "questions: empty"},
		"null constraints":     {"constraints", `null`, nil, "constraints: empty"},
		"blank constraint":     {"constraints", `["Budget", " "]`, nil, "constraints: a blank item"},
		"persona without role": {"personas", `[{"name":"Rosa"}]`, nil, "personas: a persona lacks name or role"},
		"null goal":            {"personas", `[{"name":"Rosa","role":"seller","goals":[null]}]`, nil, "personas: a persona has a blank goal"},
		"blank goal":           {"personas", `[{"name":"Rosa","role":"seller","goals":[" "]}]`, nil, "personas: a persona has a blank goal"},
		"null pain point":      {"personas", `[{"name":"Rosa","role":"seller","painPoints":[null]}]`, nil, "personas: a persona has a blank pain point"},
		"blank pain point":     {"personas", `[{"name":"Rosa","role":"seller","painPoints":[" "]}]`, nil, "personas: a persona has a blank pain point"},
		"flow without steps":   {"flows", `[{"id":"fl1","name":"Listing","steps":[]}]`, nil, "flows: a flow lacks name or steps"},
		"prose":                {"personas", "Rosa sells bikes.", nil, "personas: invalid output"},
		"agent":                {"flows", "", errors.New("timed out"), "flows: timed out"},
	} {
		dir, r := start(t)
		calls := 0
		err := Run(dir, r, step, func(string) (string, error) {
			calls++
			if names[calls-1] == c.list {
				return c.answer, c.err
			}
			return recorded(t, names[calls-1]), nil
		})
		if err == nil || err.Error() != "failed at analysis: "+c.want || calls != slices.Index(names, c.list)+1 {
			t.Errorf("%s: %v after %d calls", name, err, calls)
		}
		if _, err := os.Stat(filepath.Join(dir, "analysis.json")); err == nil {
			t.Errorf("%s: analysis.json written", name)
		}
		got, rerr := pipeline.Read(dir)
		if rerr != nil {
			t.Fatal(rerr)
		}
		if got.Analysis != nil || got.Report.Status != "failed" || got.Report.Steps[2].Status != "failed at analysis: "+c.list ||
			got.Research == nil || got.Context != context || got.Logs["analysis"] == "" {
			t.Errorf("%s: %+v", name, got)
		}
	}
}

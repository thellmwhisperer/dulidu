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
	b, err := os.ReadFile(filepath.Join("testdata", list+".md"))
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
	// research.json reaches every call as the prototype's named paragraphs, one per dimension.
	research := "Market: Bike swaps grow."
	if strings.Contains(asked[0], `"dimensions"`) {
		t.Errorf("call 1 still carries research.json as JSON:\n%s", asked[0])
	}
	want := pipeline.Analysis{
		Personas: []pipeline.Persona{{Name: "Rosa", Role: "seller", Goals: []string{"List a red-74 bike"},
			PainPoints: []string{"Untagged bikes get rejected"}},
			{Name: "Tom", Role: "buyer", Goals: []string{"Find a bike"}, PainPoints: []string{}}},
		Functionalities: []pipeline.Functionality{
			{ID: "F1", Name: "List a bike", Description: "Only red-74 bikes", Priority: "must"},
			{ID: "F2", Name: "Search", Description: "Find bikes by size", Priority: "could"}},
		Flows:       []pipeline.Flow{{ID: "FL1", Name: "Listing", Steps: []string{"Scan the tag", "Add photos", "Publish"}}},
		Constraints: []string{"Only bicycles tagged red-74 can be listed"},
		Questions:   []string{"Who prints the red-74 tags?"},
	}
	// No call reads a list asked before it: the five get the same context, the prototype's buildSectionPrompt.
	earlier := map[string][]string{
		"Personas":        {"Rosa (seller)", "Goals: List a red-74 bike", "Pain points: Untagged bikes get rejected"},
		"Functionalities": {"[must] List a bike: Only red-74 bikes", "[could] Search: Find bikes by size"},
		"Flows":           {"Listing: Scan the tag → Add photos → Publish"},
		"Constraints":     want.Constraints,
	}
	data := map[string]any{"Input": context, "Research": research}
	if len(asked) != 5 {
		t.Fatalf("%d calls, want 5", len(asked))
	}
	for i, tmpl := range []string{"p09-analysis-personas", "p10-analysis-functionalities", "p11-analysis-flows",
		"p12-analysis-constraints", "p13-analysis-questions"} {
		prompt, err := prompts.Render(tmpl, data)
		if err != nil {
			t.Fatal(err)
		}
		if asked[i] != prompt || !strings.Contains(asked[i], "red-74") ||
			!strings.Contains(asked[i], "## Research Context\n"+research+"\n") {
			t.Errorf("call %d is not %s on contexto.md and the named research alone:\n%s", i+1, tmpl, asked[i])
		}
		for key, items := range earlier {
			for _, item := range items {
				if strings.Contains(asked[i], item) {
					t.Errorf("call %d carries %s %q from an earlier call:\n%s", i+1, key, item, asked[i])
				}
			}
		}
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
			return "## Rosa\nRole: seller\n### Goals\n### Pain points\n", nil
		}
		return recorded(t, names[calls-1]), nil
	}); err != nil {
		t.Fatal(err)
	}
	got, err := pipeline.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 5 || got.Analysis == nil || !reflect.DeepEqual(got.Analysis.Personas,
		[]pipeline.Persona{{Name: "Rosa", Role: "seller", Goals: []string{}, PainPoints: []string{}}}) ||
		!slices.Equal(pipeline.Lines(got.Analysis.Personas), []string{"Rosa (seller)"}) {
		t.Errorf("empty persona lists: %d calls, analysis %+v", calls, got.Analysis)
	}
}

func TestFailures(t *testing.T) {
	for name, c := range map[string]struct {
		list, answer string
		err          error
		want         string
	}{
		"no priority":          {"functionalities", "## F1 List\nA bike", nil, "functionalities: functionality List: priority not must, should or could"},
		"bad priority":         {"functionalities", "## F1 List\nPriority: won't\nA bike", nil, "functionalities: functionality List: priority not must, should or could"},
		"no description":       {"functionalities", "## F1 List\nPriority: must", nil, "functionalities: a functionality lacks name or description"},
		"persona without role": {"personas", "## Rosa", nil, "personas: a persona lacks name or role"},
		"role under goals":     {"personas", "## Rosa\n### Goals\nRole: seller\n- Sell", nil, "personas: a persona lacks name or role"},
		"blank role":           {"personas", "## Rosa\nRole: \n### Goals\n- Sell", nil, "personas: a persona lacks name or role"},
		"second without role":  {"personas", "## Rosa\nRole: seller\n\n## Tom\n### Pain points\n- Prices", nil, "personas: a persona lacks name or role"},
		"flow without steps":   {"flows", "## FL1 Listing\nScan, photo, publish.", nil, "flows: a flow lacks name or steps"},
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

// Answers without items in the requested list now produce an empty list, rather than failing the run.
func TestEmptyListAnswers(t *testing.T) {
	for name, c := range map[string]struct{ list, answer string }{
		"flows object":          {"flows", `{}`},
		"flows list":            {"flows", `[]`},
		"empty questions":       {"questions", ""},
		"null constraints":      {"constraints", "Nothing limits it."},
		"constraints in blocks": {"constraints", "## Budget\n- Under $50k"},
		"lead persona":          {"personas", "Role: seller\n### Goals\n- Sell"},
		"prose":                 {"personas", "Rosa sells bikes."},
		"json":                  {"personas", `[{"name":"Rosa","role":"seller"}]`},
	} {
		t.Run(name, func(t *testing.T) {
			dir, r := start(t)
			calls := 0
			if err := Run(dir, r, step, func(string) (string, error) {
				calls++
				if names[calls-1] == c.list {
					return c.answer, nil
				}
				return recorded(t, names[calls-1]), nil
			}); err != nil {
				t.Fatal(err)
			}
			got, err := pipeline.Read(dir)
			if err != nil {
				t.Fatal(err)
			}
			if calls != 5 || got.Analysis == nil || got.Report.Status != "done" || got.Report.Steps[2].Status != "passed" ||
				got.Research == nil || got.Context != context || got.Logs["analysis"] == "" {
				t.Fatalf("%d calls, run %+v", calls, got)
			}
			lists := map[string][]string{"personas": pipeline.Lines(got.Analysis.Personas),
				"functionalities": pipeline.Lines(got.Analysis.Functionalities), "flows": pipeline.Lines(got.Analysis.Flows),
				"constraints": got.Analysis.Constraints, "questions": got.Analysis.Questions}
			stored := map[string]bool{"personas": got.Analysis.Personas != nil,
				"functionalities": got.Analysis.Functionalities != nil, "flows": got.Analysis.Flows != nil,
				"constraints": got.Analysis.Constraints != nil, "questions": got.Analysis.Questions != nil}
			for list, items := range lists {
				if list == c.list {
					if !stored[list] || len(items) != 0 {
						t.Errorf("%s = %v, want an empty list", list, items)
					}
				} else if len(items) == 0 {
					t.Errorf("%s lost its recorded items", list)
				}
			}
		})
	}
}

func TestFunctionalityDescriptionsPreserveBlockText(t *testing.T) {
	for name, c := range map[string]struct {
		body, want string
	}{
		"colon in prose": {"Priority: must\nExport account records.\nAccess: administrators only\nKeep the audit trail.",
			"Export account records. Access: administrators only Keep the audit trail."},
		"only colon prose": {"Priority: must\nAccess: administrators only", "Access: administrators only"},
		"lists and sections": {"Priority: must\nExport account records.\n- Include IDs\n### Access\n1. Administrators only",
			"Export account records. - Include IDs ### Access 1. Administrators only"},
	} {
		t.Run(name, func(t *testing.T) {
			dir, r := start(t)
			calls := 0
			want := "[must] Export: " + c.want
			if err := Run(dir, r, step, func(prompt string) (string, error) {
				calls++
				if calls == 2 {
					return "## F1 Export\n" + c.body + "\n\n## F2 Search\nPriority: could\nFind records", nil
				}
				// Later calls no longer read the functionalities: the description lives in analysis.json alone.
				if calls > 2 && strings.Contains(prompt, c.want) {
					t.Errorf("call %d carries the functionality's description: %s", calls, prompt)
				}
				return recorded(t, names[calls-1]), nil
			}); err != nil {
				t.Fatal(err)
			}
			got, err := pipeline.Read(dir)
			if err != nil {
				t.Fatal(err)
			}
			if got.Analysis == nil || !reflect.DeepEqual(pipeline.Lines(got.Analysis.Functionalities),
				[]string{want, "[could] Search: Find records"}) || got.Analysis.Functionalities[0].Description != c.want {
				t.Fatalf("analysis.json lost description: %+v", got.Analysis)
			}
		})
	}
}

// A functionality the model wrote without an id gets F<n> by its position, and the analysis step's note says so;
// the ids the model wrote stay.
func TestFunctionalityWithoutIDGetsItsPosition(t *testing.T) {
	dir, r := start(t)
	calls := 0
	if err := Run(dir, r, step, func(string) (string, error) {
		calls++
		if calls == 2 {
			return "## F7 List a bike\nPriority: must\nOnly red-74 bikes\n\n## Search\nPriority: could\nFind bikes by size", nil
		}
		return recorded(t, names[calls-1]), nil
	}); err != nil {
		t.Fatal(err)
	}
	got, err := pipeline.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []pipeline.Functionality{{ID: "F7", Name: "List a bike", Description: "Only red-74 bikes", Priority: "must"},
		{ID: "F2", Name: "Search", Description: "Find bikes by size", Priority: "could"}}
	if got.Analysis == nil || !reflect.DeepEqual(got.Analysis.Functionalities, want) {
		t.Fatalf("functionalities: %+v", got.Analysis)
	}
	if s := got.Report.Steps[2]; s.Status != "passed" || !strings.Contains(s.Note, "Search") || !strings.Contains(s.Note, "F2") {
		t.Errorf("analysis step %+v does not note the id it gave Search", s)
	}
}

func TestFunctionalityFallbackAvoidsModelAndAssignedIDs(t *testing.T) {
	dir, r := start(t)
	calls := 0
	if err := Run(dir, r, step, func(string) (string, error) {
		calls++
		if calls == 2 {
			return "## F2 Pay\nPriority: must\nPay for a bike\n\n" +
				"## Search\nPriority: could\nFind bikes by size\n\n" +
				"## Browse\nPriority: should\nBrowse bikes\n\n" +
				"## F3 Save\nPriority: could\nSave a bike", nil
		}
		return recorded(t, names[calls-1]), nil
	}); err != nil {
		t.Fatal(err)
	}
	got, err := pipeline.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []pipeline.Functionality{
		{ID: "F2", Name: "Pay", Description: "Pay for a bike", Priority: "must"},
		{ID: "F4", Name: "Search", Description: "Find bikes by size", Priority: "could"},
		{ID: "F5", Name: "Browse", Description: "Browse bikes", Priority: "should"},
		{ID: "F3", Name: "Save", Description: "Save a bike", Priority: "could"},
	}
	if got.Analysis == nil || !reflect.DeepEqual(got.Analysis.Functionalities, want) {
		t.Fatalf("functionalities: %+v, want %+v", got.Analysis, want)
	}
	if s := got.Report.Steps[2]; s.Status != "passed" ||
		s.Note != "functionality Search had no id: F4; functionality Browse had no id: F5" {
		t.Errorf("analysis step does not record the assigned ids: %+v", s)
	}
}

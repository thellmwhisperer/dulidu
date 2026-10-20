package epics

import (
	"cmp"
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

var step = pipeline.Step{Name: "epics", Harness: "claude", Model: "opus"}

const context = "# Contexto\n\nA bike exchange; only bicycles tagged red-74 can be listed.\n"

// start is a run folder whose analysis passed, with caps of two epics and two stories each.
func start(t *testing.T) (string, *pipeline.Run) {
	t.Helper()
	at := time.Now()
	r := &pipeline.Run{Report: pipeline.Report{Status: "done", Started: at, Steps: []pipeline.Step{
		{Name: "analysis", Status: "passed", Harness: "claude", Started: at, Finished: at}}},
		Context: context,
		Triage: &pipeline.Triage{Complexity: "simple", Epics: 2, // Triage no longer chooses research dimensions.
			Reasoning: "One kind of user.", StoriesPerEpic: 2},
		Analysis: &pipeline.Analysis{Personas: []string{"Rosa (seller)", "Tom (buyer)"},
			Functionalities: []string{"[must] List a bike: Only red-74 bikes", "[could] Search: Find bikes by size", "[must] Pay: Card at pickup"},
			Flows:           []string{"Listing: Scan the tag → Publish"}, Constraints: []string{"Only bicycles tagged red-74 can be listed"},
			Questions: []string{"Who prints the red-74 tags?"}}}
	dir := t.TempDir()
	if err := pipeline.Write(dir, *r); err != nil {
		t.Fatal(err)
	}
	return dir, r
}

// recorded answers the outlines, then each epic's stories, from testdata, but for the calls in instead.
func recorded(t *testing.T, asked *[]string, instead map[int]string) func(string) (string, error) {
	return func(prompt string) (string, error) {
		*asked = append(*asked, prompt)
		if a, ok := instead[len(*asked)-1]; ok {
			return a, nil
		}
		name := []string{"outlines", "stories-E1", "stories-E2"}[len(*asked)-1]
		b, err := os.ReadFile(filepath.Join("testdata", name+".md"))
		if err != nil {
			t.Fatal(err)
		}
		return string(b), nil
	}
}

func TestARecordedSetYieldsEpicsJSON(t *testing.T) {
	dir, r := start(t)
	var asked []string
	if err := Run(dir, r, step, recorded(t, &asked, nil)); err != nil {
		t.Fatal(err)
	}
	data := map[string]any{"Input": context, "Personas": "- Rosa (seller)\n- Tom (buyer)",
		"Functionalities":     "- F1 [must] List a bike: Only red-74 bikes\n- F2 [could] Search: Find bikes by size\n- F3 [must] Pay: Card at pickup",
		"MustFunctionalities": "- F1 List a bike: Only red-74 bikes\n- F3 Pay: Card at pickup",
		"Flows":               "- Listing: Scan the tag → Publish", "Constraints": "- Only bicycles tagged red-74 can be listed",
		"Epics": 2, "Stories": 2}
	want := pipeline.Epics{
		{ID: "E1", Title: "List a red-74 bike end-to-end", Text: "A seller lists a tagged bike.", DependsOn: []string{},
			Stories: []pipeline.Story{{ID: "US1", Epic: "E1", Title: "Reject a bike without a red-74 tag",
				Text:       "As Rosa I want untagged bikes refused so that the listing stays valid",
				Acceptance: []string{"Given a bike tagged blue-12, when Rosa lists it, then it is refused"},
				FirstTest:  "Write a test that lists a blue-12 bike and expects a refusal.", Functionalities: []string{"F1"}}}},
		{ID: "E2", Title: "Pay at pickup", Text: "A buyer pays when collecting the bike.", DependsOn: []string{"E1"},
			Stories: []pipeline.Story{{ID: "US2", Epic: "E2", Title: "Charge the card at pickup",
				Text:       "As Tom I want to pay when I collect so that I see the bike first",
				Acceptance: []string{"Given a reserved bike, when Tom collects it, then his card is charged once"},
				FirstTest:  "Write a test that collects a reserved bike and expects one charge.", Functionalities: []string{"F3"}}}},
	}
	if len(asked) != 3 {
		t.Fatalf("%d calls, want the outlines and one per epic", len(asked))
	}
	for i, tmpl := range []string{"p15-epics-outlines", "p16-epics-stories", "p16-epics-stories"} {
		if i > 0 {
			data["Epic"] = pipeline.EpicOutline{ID: want[i-1].ID, Title: want[i-1].Title, Text: want[i-1].Text}
		}
		prompt, err := prompts.Render(tmpl, data)
		if err != nil {
			t.Fatal(err)
		}
		if asked[i] != prompt {
			t.Errorf("call %d asked:\n%s\nwant %s:\n%s", i+1, asked[i], tmpl, prompt)
		}
	}
	if !strings.Contains(asked[0], "at most 2 epics") || !strings.Contains(asked[1], "At most 2") {
		t.Errorf("the caps are not asked as maximums:\n%s\n%s", asked[0], asked[1])
	}
	got, err := pipeline.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Epics, want) {
		t.Errorf("epics.json:\n%+v\nwant:\n%+v", got.Epics, want)
	}
	if s := got.Report.Steps[1]; s.Status != "passed" || s.Finished.IsZero() || got.Report.Status != "done" {
		t.Errorf("report: %+v", got.Report)
	}
	if !strings.Contains(got.Logs["epics"], asked[2]) {
		t.Error("agent/epics.log lacks the last prompt")
	}
}

func TestAFailingCheckStopsWithNoEpics(t *testing.T) {
	two := `## US1 A
Functionalities: F1
A story
### Acceptance
- a
### First test
t

## US2 B
Functionalities: F1
Another story
### Acceptance
- b
### First test
t`
	for name, c := range map[string]struct {
		epics   int
		stories int // 0 is 2
		instead map[int]string
		check   string // the step's status after "failed at epics: "
		says    string // in Run's error
		calls   int
	}{
		"no epic blocks": {instead: map[int]string{0: ""},
			check: "outlines", says: "no epic blocks", calls: 1},
		"no epic title": {instead: map[int]string{0: "## E2\nDepends on: none\nA description."},
			check: "outlines", says: "epic E2 lacks id or title", calls: 1},
		"no story blocks": {instead: map[int]string{1: ""},
			check: "stories", says: "E1 has no story blocks", calls: 2},
		"no acceptance": {instead: map[int]string{1: "## US1 A\nFunctionalities: F1\nA story\n### First test\nt"},
			check: "stories", says: "lacks id, title, acceptance or first test", calls: 2},
		"unknown dependency": {instead: map[int]string{0: `## E1 A
Depends on: E9`},
			check: "unknown dependency", says: "unknown dependency: E1 on E9", calls: 1},
		"cycle": {instead: map[int]string{0: `## E1 A
Depends on: E2

## E2 B
Depends on: E1`},
			check: "cycle", says: "cycle: E1", calls: 1},
		"duplicate epic": {instead: map[int]string{0: `## E1 A
Depends on: E1

## E1 B
Depends on: none`},
			check: "duplicate epic", says: "duplicate epic: E1", calls: 1},
		"uncovered must": {instead: map[int]string{2: `## US2 B
Functionalities: none
No functionality
### Acceptance
- b
### First test
t`},
			check: "uncovered must", says: "uncovered must: F3", calls: 3},
		"too many epics": {epics: 1,
			check: "over cap", says: "over cap: 2 epics, cap 1", calls: 1},
		"too many stories": {stories: 1, instead: map[int]string{1: two},
			check: "over cap", says: "over cap: 2 stories in E1, cap 1", calls: 2},
		"no first test": {instead: map[int]string{1: `## US1 A
Functionalities: F1
A story
### Acceptance
- a`},
			check: "stories", says: "lacks id, title, acceptance or first test", calls: 2},
		"no description": {instead: map[int]string{1: `## US1 A
Functionalities: F1
### Acceptance
- a
### First test
t`},
			check: "stories", says: "lacks description", calls: 2},
		"unknown functionality": {instead: map[int]string{1: `## US1 A
Functionalities: F1, F999
A story
### Acceptance
- a
### First test
t`},
			check: "stories", says: "invalid functionalities", calls: 2},
		"missing functionalities": {instead: map[int]string{1: `## US1 A
A story
### Acceptance
- a
### First test
t`},
			check: "stories", says: "invalid functionalities", calls: 2},
		"agent error": {instead: map[int]string{}, check: "outlines", says: "outlines: down", calls: 1},
	} {
		t.Run(name, func(t *testing.T) {
			dir, r := start(t)
			if c.epics > 0 {
				r.Triage.Epics = c.epics
			}
			r.Triage.StoriesPerEpic = cmp.Or(c.stories, 2)
			var asked []string
			ask := recorded(t, &asked, c.instead)
			if name == "agent error" {
				ask = func(p string) (string, error) { asked = append(asked, p); return "", errors.New("down") }
			}
			err := Run(dir, r, step, ask)
			if err == nil || !strings.Contains(err.Error(), "failed at epics: "+c.check) || !strings.Contains(err.Error(), c.says) {
				t.Fatalf("error: %v, want %q and %q", err, "failed at epics: "+c.check, c.says)
			}
			if len(asked) != c.calls {
				t.Errorf("%d calls, want %d", len(asked), c.calls)
			}
			got, rerr := pipeline.Read(dir)
			if rerr != nil {
				t.Fatal(rerr)
			}
			if got.Epics != nil {
				t.Errorf("epics.json written: %+v", got.Epics)
			}
			if s := got.Report.Steps[1]; s.Status != "failed at epics: "+c.check || got.Report.Status != "failed" {
				t.Errorf("report: %+v", got.Report)
			}
		})
	}
}

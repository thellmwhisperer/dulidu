package export

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// recorded is a run whose epics passed: two epics, the second on the first, three stories.
func recorded() pipeline.Run {
	at := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	return pipeline.Run{Report: pipeline.Report{Status: "done", Started: at, Steps: []pipeline.Step{
		{Name: "epics", Status: "passed", Harness: "claude", Model: "opus", Started: at, Finished: at.Add(time.Minute)}}},
		Research: &pipeline.Research{Dimensions: []pipeline.Dimension{{Name: "users",
			Text:    "Second-hand bike buyers want to see a bike before they pay for it.",
			Sources: []pipeline.Source{{URL: "https://example.com/bike-buyers", Title: "Bike buyers survey", Supports: "buyers want to see a bike before they pay"}}}}},
		Analysis: &pipeline.Analysis{Personas: []string{"Rosa (seller)", "Tom (buyer)"},
			Functionalities: []string{"[must] List a bike: Only red-74 bikes", "[could] Search: Find bikes by size", "[must] Pay: Card at pickup"},
			Flows:           []string{"Listing: Scan the tag → Publish"}, Constraints: []string{"Only bicycles tagged red-74 can be listed"},
			Questions: []string{"Who prints the red-74 tags?"}},
		Epics: pipeline.Epics{
			{ID: "E1", Title: "List a red-74 bike end-to-end", Text: "A seller lists a tagged bike.", DependsOn: []string{},
				Stories: []pipeline.Story{{ID: "US1", Epic: "E1", Title: "Reject a bike without a red-74 tag",
					Text:       "As Rosa I want untagged bikes refused so that the listing stays valid",
					Acceptance: []string{"Given a bike tagged blue-12, when Rosa lists it, then it is refused"},
					FirstTest:  "Write a test that lists a blue-12 bike and expects a refusal.", Functionalities: []string{"F1"}},
					{ID: "US2", Epic: "E1", Title: "Publish a red-74 bike",
						Text:       "As Rosa I want my tagged bike published so that buyers find it",
						Acceptance: []string{"Given a bike tagged red-74, when Rosa lists it, then it is published"},
						FirstTest:  "Write a test that lists a red-74 bike and expects it published.", Functionalities: []string{"F1"}}}},
			{ID: "E2", Title: "Pay at pickup", Text: "A buyer pays when collecting the bike.", DependsOn: []string{"E1"},
				Stories: []pipeline.Story{{ID: "US3", Epic: "E2", Title: "Charge the card at pickup",
					Text:       "As Tom I want to pay when I collect so that I see the bike first",
					Acceptance: []string{"Given a reserved bike, when Tom collects it, then his card is charged once"},
					FirstTest:  "Write a test that collects a reserved bike and expects one charge.", Functionalities: []string{"F3"}}}},
		}}
}

// The recorded run's folder, exported by testdata/export.go, has every file of testdata/golden byte for byte, and no
// other ticket.
func TestExportMatchesGolden(t *testing.T) {
	dir := t.TempDir()
	if err := pipeline.Write(dir, recorded()); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("go", "run", "./testdata/export.go", dir).CombinedOutput(); err != nil {
		t.Errorf("export: %v\n%s", err, out)
	}
	golden := os.DirFS(filepath.Join("testdata", "golden"))
	err := fs.WalkDir(golden, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		want, err := fs.ReadFile(golden, name)
		if err != nil {
			return err
		}
		if got, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(got) != string(want) {
			t.Errorf("%s: %v\n%s\nwant:\n%s", name, err, got, want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := filepath.Glob(filepath.Join(dir, "tickets", "*.md"))
	want, _ := fs.Glob(golden, "tickets/*.md")
	if len(got) != len(want) {
		t.Errorf("%d tickets, want %d", len(got), len(want))
	}
}

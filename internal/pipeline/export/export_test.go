package export

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		Analysis: &pipeline.Analysis{Personas: []pipeline.Persona{{Name: "Rosa", Role: "seller"}, {Name: "Tom", Role: "buyer"}},
			Functionalities: []pipeline.Functionality{
				{ID: "F1", Name: "List a bike", Description: "Only red-74 bikes", Priority: "must"},
				{ID: "F2", Name: "Search", Description: "Find bikes by size", Priority: "could"},
				{ID: "F3", Name: "Pay", Description: "Card at pickup", Priority: "must"}},
			Flows:       []pipeline.Flow{{ID: "FL1", Name: "Listing", Steps: []string{"Scan the tag", "Publish"}}},
			Constraints: []string{"Only bicycles tagged red-74 can be listed"},
			Questions:   []string{"Who prints the red-74 tags?"}},
		Epics: pipeline.Epics{
			{ID: "E1", Title: "List a red-74 bike end-to-end", Text: "A seller lists a tagged bike.", DependsOn: []string{},
				Stories: []pipeline.Story{{ID: "US1", Epic: "E1", Title: "Reject a bike without a red-74 tag",
					Text:       "As Rosa I want untagged bikes refused so that the listing stays valid",
					Acceptance: []string{"Given a bike tagged blue-12, when Rosa lists it, then it is refused"},
					FirstTest:  "Write a test that lists a blue-12 bike and expects a refusal.", Functionalities: []string{"F1"},
					Invariants: []string{"Only bicycles tagged red-74 can be listed"},
					UseCases:   []string{"Rosa scans the blue-12 tag of her bike and sees the listing refused"}},
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

// The recorded run's folder, in a repository folder named bikeswap and exported by testdata/export.go, has every file
// of testdata/golden byte for byte, spec.md titled bikeswap, and no other ticket.
func TestExportMatchesGolden(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bikeswap", ".dulidu", "runs", "20261016-120000")
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

	t.Run("functionality pipes stay in their table cells", func(t *testing.T) {
		dir := t.TempDir()
		r := recorded()
		r.Analysis.Functionalities[0].Name = "Export | rows"
		r.Analysis.Functionalities[0].Description = "Export rows using | as the separator."
		if err := pipeline.Write(dir, r); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(filepath.Join(dir, "analysis.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := Write(dir, &r); err != nil {
			t.Fatal(err)
		}
		spec, err := os.ReadFile(filepath.Join(dir, "spec.md"))
		if err != nil {
			t.Fatal(err)
		}
		want := `| F1 | must | Export \| rows | Export rows using \| as the separator. |`
		if !strings.Contains(string(spec), want+"\n") {
			t.Errorf("spec.md must preserve four functionality cells; want %q in:\n%s", want, spec)
		}
		after, err := os.ReadFile(filepath.Join(dir, "analysis.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Error("export changed analysis.json while escaping the functionality table")
		}
	})
}

func TestReexportRemovesObsoleteTickets(t *testing.T) {
	dir := t.TempDir()
	r := recorded()
	if err := Write(dir, &r); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tickets", "notes.md"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.Epics = r.Epics[:1]
	if err := Write(dir, &r); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"1.md", "2.md", "notes.md"} {
		if _, err := os.Stat(filepath.Join(dir, "tickets", name)); err != nil {
			t.Errorf("%s should remain: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "tickets", "3.md")); !os.IsNotExist(err) {
		t.Errorf("obsolete 3.md remains: %v", err)
	}
}

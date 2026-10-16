package pipeline

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// complete is a fake run with every artefact.
func complete() Run {
	at := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	return Run{
		Report: Report{Status: "done", Started: at, Steps: []Step{
			{Name: "triage", Status: "done", Model: "opus", Harness: "claude", Started: at, Finished: at.Add(time.Minute)},
			{Name: "research", Status: "done", Harness: "codex", Started: at.Add(time.Minute), Finished: at.Add(5 * time.Minute)},
		}},
		Input:       &InputContext{Brief: Brief{Text: "A shop for second-hand bikes.\n"}, Files: map[string]string{"notes.md": "Spain first.\n"}},
		Context:     "# Contexto\n\nSecond-hand bikes.\n",
		Triage:      &Triage{Summary: "A marketplace", Dimensions: []string{"market", "legal"}},
		Research:    &Research{Dimensions: []Dimension{{Name: "market", Text: "Crowded.", Sources: []string{"https://example.com/market"}}}},
		Analysis:    &Analysis{Personas: []string{"seller"}, Functionalities: []string{"list a bike"}, Flows: []string{"sell"}, Questions: []string{"fees?"}},
		Epics:       Epics{{ID: "E1", Title: "Selling", Text: "Sellers list bikes."}},
		Tickets:     map[string]Story{"E1-1": {Epic: "E1", Title: "List a bike", Text: "As a seller", Acceptance: []string{"the bike shows up"}}},
		Spec:        "# Spec\n",
		Plan:        []byte(`{"order": ["E1-1"]}` + "\n"),
		DesignBrief: "# Brief de diseño\n",
		Logs:        map[string]string{"triage": "said hello\n", "research": "searched\n"},
	}
}

func TestRunFolderRoundTrip(t *testing.T) {
	for name, want := range map[string]Run{"complete": complete(), "just started": {Report: complete().Report}} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), ".dulidu", "runs", "label")
			if err := Write(dir, want); err != nil {
				t.Fatal(err)
			}
			got, err := Read(dir)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("read back:\n%+v\nwant:\n%+v", got, want)
			}
		})
	}
	dir := t.TempDir()
	if err := Write(dir, complete()); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"report.json", "contexto.md", "triage.json", "research.json", "analysis.json",
		"epics.json", "input/brief.md", "input/notes.md", "agent/triage.log", "spec.md", "tickets/E1-1.json",
		"plan.json", "brief-diseno.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Error(err)
		}
	}
}

func TestRunFolderRefusesWhatDoesNotValidate(t *testing.T) {
	bad := complete()
	bad.Analysis = &Analysis{}
	bad.Logs = map[string]string{"../escape": ""}
	dir := t.TempDir()
	if err := Write(dir, bad); err == nil {
		t.Fatal("Write took an empty analysis and a log outside agent/")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("Write wrote %d files of a run it refused", len(entries))
	}
	if _, err := Read(t.TempDir()); err == nil {
		t.Fatal("Read took a folder with no report.json")
	}
}

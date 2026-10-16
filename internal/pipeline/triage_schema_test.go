package pipeline

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestTriageRejectsWhatTheSchemaDoesNot(t *testing.T) {
	for name, change := range map[string]func(*Triage){
		"complexity":         func(tr *Triage) { tr.Complexity = "huge" },
		"zero epics":         func(tr *Triage) { tr.Caps.Epics = 0 },
		"negative stories":   func(tr *Triage) { tr.Caps.StoriesPerEpic = -1 },
		"research dimension": func(tr *Triage) { tr.ResearchDimensions = []string{"legal"} },
		"project type":       func(tr *Triage) { tr.ProjectType = "" },
		"blank question":     func(tr *Triage) { tr.MissingQuestions = []string{" "} },
		"blank context":      func(tr *Triage) { tr.ContextPresent = []string{""} },
	} {
		tr := *complete().Triage
		change(&tr)
		if tr.Validate() == nil {
			t.Errorf("%s: a bad triage validates", name)
		}
	}
	tr := *complete().Triage
	tr.ResearchDimensions, tr.MissingQuestions = []string{"business", "users", "market", "technical", "project"}, []string{"Who pays?"}
	if err := tr.Validate(); err != nil {
		t.Errorf("a triage with every dimension and a question: %v", err)
	}
}

func TestStepNoteRoundTrips(t *testing.T) {
	want := complete()
	want.Report.Steps[0].Note = "clamped caps.epics 12 to max_epics 8"
	dir := filepath.Join(t.TempDir(), "run")
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
}

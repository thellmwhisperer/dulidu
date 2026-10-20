package pipeline

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTriageRejectsWhatTheSchemaDoesNot(t *testing.T) {
	for name, change := range map[string]string{
		"complexity":         `{"complexity":"huge"}`,
		"zero epics":         `{"epics":0}`,
		"negative epics":     `{"epics":-1}`,
		"research dimension": `{"research_dimensions":["legal"]}`,
		"no complexity":      `{"complexity":""}`,
		"duplicate":          `{"research_dimensions":["users","users"]}`,
		"no dimension list":  `{"research_dimensions":null}`,
	} {
		tr := *complete().Triage
		if err := json.Unmarshal([]byte(change), &tr); err != nil {
			t.Fatal(err)
		}
		if tr.Validate() == nil {
			t.Errorf("%s: a bad triage validates", name)
		}
	}
	tr := *complete().Triage
	tr.ResearchDimensions = []string{"business", "users", "market", "technical", "project"}
	if err := json.Unmarshal([]byte(`{"reasoning":""}`), &tr); err != nil {
		t.Fatal(err)
	}
	if err := tr.Validate(); err != nil {
		t.Errorf("a triage with every dimension and no reasoning: %v", err)
	}
}

func TestStepNoteRoundTrips(t *testing.T) {
	want := complete()
	want.Report.Steps[0].Note = "clamped epics from 8 to max_epics 2"
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

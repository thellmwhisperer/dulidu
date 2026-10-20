package pipeline

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTriageRejectsWhatTheSchemaDoesNot(t *testing.T) {
	for name, change := range map[string]string{
		"complexity":     `{"complexity":"huge"}`,
		"zero epics":     `{"epics":0}`,
		"negative epics": `{"epics":-1}`,
		"no complexity":  `{"complexity":""}`,
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
	if err := json.Unmarshal([]byte(`{"reasoning":""}`), &tr); err != nil {
		t.Fatal(err)
	}
	if err := tr.Validate(); err != nil {
		t.Errorf("a triage without dimensions or reasoning: %v", err)
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

func TestTriageIgnoresRemovedResearchDimensions(t *testing.T) {
	for _, legacy := range []string{`{}`, `{"research_dimensions":null}`, `{"research_dimensions":[]}`,
		`{"research_dimensions":["legal"]}`, `{"research_dimensions":["users","users"]}`,
		`{"research_dimensions":"not a list"}`} {
		tr := Triage{Complexity: "simple", Epics: 2}
		if err := json.Unmarshal([]byte(legacy), &tr); err != nil {
			t.Fatalf("legacy field %s: %v", legacy, err)
		}
		if err := tr.Validate(); err != nil {
			t.Errorf("legacy field %s affects validation: %v", legacy, err)
		}
		b, err := json.Marshal(tr)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if err := json.Unmarshal(b, &fields); err != nil {
			t.Fatal(err)
		}
		if _, exists := fields["research_dimensions"]; exists {
			t.Errorf("triage.json retains the removed field: %s", b)
		}
	}
}

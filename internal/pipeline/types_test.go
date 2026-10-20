package pipeline

import (
	"encoding/json"
	"testing"
)

func TestValidateRejects(t *testing.T) {
	for name, v := range map[string]interface{ Validate() error }{
		"brief": &Brief{}, "input": &InputContext{}, "triage": &Triage{}, "research": &Research{},
		"analysis": &Analysis{}, "epic": &EpicOutline{}, "epics": &Epics{}, "story": &Story{}, "report": &Report{},
	} {
		if err := json.Unmarshal([]byte("{}"), v); err != nil {
			if err = json.Unmarshal([]byte("[]"), v); err != nil {
				t.Fatal(name, err)
			}
		}
		if v.Validate() == nil {
			t.Errorf("%s: {} validates", name)
		}
	}
	r := complete()
	if err := r.Validate(); err != nil {
		t.Fatalf("a complete run does not validate: %v", err)
	}
	analysis := *r.Analysis
	analysis.Personas = nil
	if analysis.Validate() == nil {
		t.Error("an analysis without personas validates")
	}
	story := r.Tickets["E1-1"]
	story.Acceptance = nil
	if story.Validate() == nil {
		t.Error("a story without acceptance validates")
	}
	research := Research{Dimensions: []Dimension{{Name: "market", Text: "Crowded.", Sources: []Source{}}}}
	if research.Validate() == nil {
		t.Error("a research dimension with empty sources validates")
	}
	triage := *r.Triage
	if err := json.Unmarshal([]byte(`{"research_dimensions":["market","market"]}`), &triage); err != nil {
		t.Fatal(err)
	}
	if err := triage.Validate(); err != nil {
		t.Errorf("the removed research_dimensions field affected triage validation: %v", err)
	}
	dimension := Dimension{Name: "market", Text: "Crowded.", Sources: []Source{{URL: "u", Title: "t", Supports: "s"}}}
	if err := (Research{Dimensions: []Dimension{dimension, dimension}}).Validate(); err == nil ||
		err.Error() != "research: duplicate dimension market" {
		t.Errorf("duplicate research dimensions: %v", err)
	}
	input := *r.Input
	input.Files = map[string]string{"../x": ""}
	if input.Validate() == nil {
		t.Error("an input file outside input/ validates")
	}
}

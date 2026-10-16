package pipeline

import (
	"encoding/json"
	"strings"
	"testing"
)

// Every dependency names an existing epic and none loops back: Write keeps such epics.json out of the run folder.
func TestEpicsDependOnlyOnEpicsAndNeverInACircle(t *testing.T) {
	for check, epics := range map[string]string{
		"unknown dependency": `[{"id": "E1", "title": "Selling", "depends_on": ["E9"]}]`,
		"cycle": `[{"id": "E1", "title": "Selling", "depends_on": ["E2"]},
			{"id": "E2", "title": "Paying", "depends_on": ["E1"]}]`,
	} {
		r := complete()
		if err := json.Unmarshal([]byte(epics), &r.Epics); err != nil {
			t.Fatal(err)
		}
		if err := Write(t.TempDir(), r); err == nil || !strings.Contains(err.Error(), check) {
			t.Errorf("%s: %v", check, err)
		}
	}
	r := complete()
	if err := json.Unmarshal([]byte(`[{"id": "E1", "title": "Selling", "depends_on": []},
		{"id": "E2", "title": "Paying", "depends_on": ["E1"]}]`), &r.Epics); err != nil {
		t.Fatal(err)
	}
	if err := Write(t.TempDir(), r); err != nil {
		t.Errorf("a dependency on an earlier epic: %v", err)
	}
}

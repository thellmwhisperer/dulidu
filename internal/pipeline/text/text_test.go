package text

import (
	"reflect"
	"testing"
)

// An answer's lead, blocks, fields without case, sub-sections, both lists and prose; a missing part is empty.
func TestParse(t *testing.T) {
	got := Parse("Complexity: simple\nWhy it is.\n\n## Personas\nCOUNT: 2\n- Seller\n1. Buyer\nTwo kinds.\n" +
		"### Seller\nGoal: sell fast\nOwns bikes.\n## Empty\n")
	want := []Block{
		{Fields: map[string]string{"complexity": "simple"}, Prose: "Why it is."},
		{Name: "Personas", Fields: map[string]string{"count": "2"}, Items: []string{"Seller", "Buyer"}, Prose: "Two kinds.",
			Sections: []Block{{Name: "Seller", Fields: map[string]string{"goal": "sell fast"}, Prose: "Owns bikes."}}},
		{Name: "Empty"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse:\n%+v\nwant\n%+v", got, want)
	}
	if got[0].Field("COMPLEXITY") != "simple" || got[0].Field("Epics") != "" || got[2].Field("Count") != "" {
		t.Error("Field is not matched without case or a missing one is not empty")
	}
}

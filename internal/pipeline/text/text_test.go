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
		{Fields: map[string]string{"complexity": "simple"}, Prose: "Why it is.",
			RawLines: []string{"Complexity: simple", "Why it is.", ""}},
		{Name: "Personas", Fields: map[string]string{"count": "2"}, Items: []string{"Seller", "Buyer"}, Prose: "Two kinds.",
			RawLines: []string{"COUNT: 2", "- Seller", "1. Buyer", "Two kinds.", "### Seller", "Goal: sell fast", "Owns bikes."},
			Sections: []Block{{Name: "Seller", Fields: map[string]string{"goal": "sell fast"}, Prose: "Owns bikes.",
				RawLines: []string{"Goal: sell fast", "Owns bikes."}}}},
		{Name: "Empty", RawLines: []string{""}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse:\n%+v\nwant\n%+v", got, want)
	}
	if got[0].Field("COMPLEXITY") != "simple" || got[0].Field("Epics") != "" || got[2].Field("Count") != "" {
		t.Error("Field is not matched without case or a missing one is not empty")
	}
}

// An item wrapped over indented lines reads as one item; an unindented line after an item is still prose.
func TestParseWrappedItem(t *testing.T) {
	acc := Parse("## Story\n### Acceptance\n- Given a free court,\n  when I book it,\n  then it is reserved.\n" +
		"1. Second\n   wrapped too.\nNot wrapped.\n")[1].Sections[0]
	if want := []string{"Given a free court, when I book it, then it is reserved.", "Second wrapped too."}; !reflect.DeepEqual(acc.Items, want) {
		t.Errorf("Items = %q, want %q", acc.Items, want)
	}
	if acc.Prose != "Not wrapped." {
		t.Errorf("Prose = %q, want %q", acc.Prose, "Not wrapped.")
	}
}

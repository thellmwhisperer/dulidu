package prompts

import (
	"strings"
	"testing"
)

// P16 keeps the poc's story target, 70% of the maximum to the maximum, and adds no exclusion of its own.
func TestStoriesTarget(t *testing.T) {
	got, err := Render("p16-epics-stories", sample)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Target 4-5 stories.") {
		t.Errorf("no 70%%-to-max target:\n%s", got)
	}
	if strings.Contains(got, "out of scope") {
		t.Error("an exclusion the poc does not have")
	}
}

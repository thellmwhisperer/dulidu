package prompts

import (
	"regexp"
	"strings"
	"testing"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// sample is what a run hands the prompts, for a project that is no product of ours.
var sample = map[string]any{
	"Input":               "A booking tool for a network of community sports courts.\n\n### rates.csv\ncourt,hour,price",
	"Research":            "Business: Clubs rent courts by the hour.\nUsers: Amateur players.\nMarket: Phone and paper bookings.\nTechnical: Card payments.\nProject: Two clubs first.",
	"Personas":            "- Ana (club manager)",
	"Functionalities":     "- [must] Book a court: pick a slot and pay\n- [could] Invite friends: share a booking",
	"MustFunctionalities": "- Book a court: pick a slot and pay",
	"Flows":               "- Booking: choose court → choose hour → pay",
	"Constraints":         "- Budget under $50k",
	"Epic":                pipeline.EpicOutline{ID: "E1", Title: "Book a court end-to-end", Text: "A player books and pays."},
	"Epics":               3,
	"Stories":             5,
}

func TestAllRender(t *testing.T) {
	all := templates.Templates()
	if len(all) != 16 {
		t.Fatalf("%d templates, want 16", len(all))
	}
	for _, tmpl := range all {
		step, ok := strings.CutSuffix(tmpl.Name(), ".md.tmpl")
		if !ok {
			continue
		}
		got, err := Render(step, sample)
		if err != nil {
			t.Errorf("%s: %v", step, err)
		} else if strings.TrimSpace(got) == "" {
			t.Errorf("%s: renders empty", step)
		}
	}
	if _, err := Render("p03-research-business", map[string]any{}); err == nil || !strings.Contains(err.Error(), `"Input"`) {
		t.Errorf("a missing key renders: %v", err)
	}
	if _, err := Render("p99-none", sample); err == nil {
		t.Error("an unknown step renders")
	}
}

func TestNoProductLeaks(t *testing.T) {
	forbidden := regexp.MustCompile(`(?i)wall ?papi|react native|monorepo|\d+\s*(lines|loc)\b|lines of (production )?code|` +
		`(?-i:iOS)`)
	for _, s := range []string{"Wall Papi", "wallpapi", "iOS", "React Native", "monorepo", "~100-200 lines", "300 LOC", "lines of code"} {
		if !forbidden.MatchString(s) {
			t.Errorf("%q passes", s)
		}
	}
	entries, err := files.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := files.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		if leak := forbidden.FindString(string(b)); leak != "" {
			t.Errorf("%s: contains %q", e.Name(), leak)
		}
	}
}

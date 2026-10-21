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
	"Screen":              pipeline.Screen{Name: "Court list", Purpose: "A player picks a free court."},
	"Screens":             "- Court list: A player picks a free court.\n- Booking: A player pays for the court.",
	"Epics":               3,
	"Stories":             5,
}

func TestAllRender(t *testing.T) {
	all := templates.Templates()
	if len(all) != 19 {
		t.Fatalf("%d templates, want the 18 P01-P18 and triage", len(all))
	}
	for _, step := range []string{"p01-triage-classifier", "p02-research-full", "p03-research-business",
		"p04-research-users", "p05-research-market", "p06-research-technical", "p07-research-project",
		"p08-analysis-full", "p09-analysis-personas", "p10-analysis-functionalities", "p11-analysis-flows",
		"p12-analysis-constraints", "p13-analysis-questions", "p14-epics-full", "p15-epics-outlines",
		"p16-epics-stories", "p17-design-screens", "p18-design-screen", "triage"} {
		if templates.Lookup(step+".md.tmpl") == nil {
			t.Errorf("no template %s", step)
		}
	}
	for _, tmpl := range all {
		step, ok := strings.CutSuffix(tmpl.Name(), ".md.tmpl")
		if !ok {
			continue
		}
		data := sample
		if step == "triage" {
			data = map[string]any{"Context": sample["Input"]}
		}
		got, err := Render(step, data)
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

func TestResearchPromptsAskForTextAndSources(t *testing.T) {
	for _, step := range []string{"p03-research-business", "p04-research-users", "p05-research-market",
		"p06-research-technical", "p07-research-project"} {
		got, err := Render(step, sample)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, "plain text") || !strings.Contains(got, "prose in the lead block") ||
			!strings.Contains(got, "## Sources") || !strings.Contains(got, "- [<title>](<url>): <the claim it backs>") ||
			!strings.Contains(got, "at least one source") || !strings.Contains(got, sample["Input"].(string)) {
			t.Errorf("%s does not ask for text and sources on its input:\n%s", step, got)
		}
		if strings.Contains(strings.ToLower(got), "json") {
			t.Errorf("%s still mentions JSON:\n%s", step, got)
		}
		if !strings.Contains(got, "Example:\nMembers need reliable bookings.\n\n## Sources\n"+
			"- [Member survey](https://example.com/survey): Members need reliable bookings.") {
			t.Errorf("%s lacks a short example with prose, title, URL and claim:\n%s", step, got)
		}
	}
}

// The five analysis prompts carry the same context, as the prototype's buildSectionPrompt: the brief and the named
// research, and none of the lists asked before them.
func TestAnalysisPromptsCarryTheSameContext(t *testing.T) {
	var context string
	for _, step := range []string{"p09-analysis-personas", "p10-analysis-functionalities", "p11-analysis-flows",
		"p12-analysis-constraints", "p13-analysis-questions"} {
		got, err := Render(step, sample)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, sample["Input"].(string)) || !strings.Contains(got, sample["Research"].(string)) {
			t.Errorf("%s lacks contexto.md or the research:\n%s", step, got)
		}
		for _, key := range []string{"Personas", "Functionalities", "Flows", "Constraints"} {
			if strings.Contains(got, sample[key].(string)) {
				t.Errorf("%s carries %s:\n%s", step, key, got)
			}
		}
		_, own, _ := strings.Cut(got, "Analyze this project and provide your extraction.")
		if context == "" {
			context = own
		} else if own != context {
			t.Errorf("%s's context differs from p09's:\n%s", step, own)
		}
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
		switch e.Name() {
		case "p15-epics-outlines.md.tmpl", "p16-epics-stories.md.tmpl":
			continue
		case "p03-research-business.md.tmpl", "p04-research-users.md.tmpl", "p05-research-market.md.tmpl",
			"p06-research-technical.md.tmpl", "p07-research-project.md.tmpl",
			"p10-analysis-functionalities.md.tmpl", "p11-analysis-flows.md.tmpl",
			"p12-analysis-constraints.md.tmpl", "p13-analysis-questions.md.tmpl":
			continue
		}
		b, err := files.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		if leak := forbidden.FindString(string(b)); leak != "" {
			t.Errorf("%s: contains %q", e.Name(), leak)
		}
	}
	for _, step := range []string{"p03-research-business", "p04-research-users", "p05-research-market",
		"p06-research-technical", "p07-research-project", "p10-analysis-functionalities",
		"p11-analysis-flows", "p12-analysis-constraints", "p13-analysis-questions", "p15-epics-outlines", "p16-epics-stories"} {
		got, err := Render(step, sample)
		if err != nil {
			t.Fatal(err)
		}
		// The stories prompt keeps the prototype's story size, its one line count.
		if size := "Target ~100-200 lines of production code. Max 300."; step == "p16-epics-stories" {
			if !strings.Contains(got, size) {
				t.Errorf("%s lacks the story size %q", step, size)
			}
			got = strings.Replace(got, size, "", 1)
		}
		if leak := forbidden.FindString(got); leak != "" {
			t.Errorf("%s: rendered prompt contains %q", step, leak)
		}
	}
}

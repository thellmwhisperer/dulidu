//go:build unix

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
)

// researchWith makes the fake claude answer the nth research call with answers[n-1].
func researchWith(t *testing.T, answers ...string) {
	t.Helper()
	dir := t.TempDir()
	for i, answer := range answers {
		line, err := json.Marshal(map[string]string{"type": "result", "result": answer})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, strconv.Itoa(i+1)), append(line, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("FAKE_RESEARCH", dir)
}

// sourceFile is one source as the declared research.json has it.
type sourceFile struct{ URL, Title, Supports string }

// researchFile is research.json as the declared schema has it.
type researchFile struct {
	Dimensions []struct {
		Name, Text string
		Sources    []sourceFile
	}
}

// finding is a valid research answer about dimension.
func finding(dimension string) string {
	return "The " + dimension + " finding.\n\n## Sources\n" +
		"- [On " + dimension + "](https://example.com/" + dimension + "): The " + dimension + " finding."
}

const unique = "Only bicycles tagged red-74 can be listed.\n"

// triageOn makes the fake claude triage in plain text, simple with 2 epics, dimensions and questions as lines it ignores.
func triageOn(t *testing.T, dimensions, questions string) {
	t.Helper()
	triageWith(t, "Complexity: simple\nEpics: 2\nResearch dimensions: "+dimensions+
		"\nContext present: brief, rules.md\nMissing questions: "+questions+"\nProject type: new_product\n\nOne check.\n")
}

func calls(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(b), "\n")
}

func TestRunResearchesAllFiveDimensionsOnceWithSources(t *testing.T) {
	log := runRepo(t, map[string]string{"rules.md": unique})
	triageOn(t, `["market","users","technical"]`, `[]`)
	researchWith(t, finding("business"), finding("users"), finding("market"), finding("technical"), finding("project"))
	if code, out, errOut := dulidu(t, "run", "brief.md", "--attach", "rules.md"); code != 0 {
		t.Fatalf("dulidu run: exit %d\n%s%s", code, out, errOut)
	}
	dir := runs(t)[0]
	if n := calls(t, log+".research"); n != 5 {
		t.Fatalf("%d research calls, want 5", n)
	}
	if got := read(t, log+".research"); strings.Count(got, "--model sonnet") != 5 || strings.Count(got, "--effort medium") != 5 {
		t.Errorf("research calls did not use the research step's model and effort: %q", got)
	}
	context := read(t, filepath.Join(dir, "contexto.md"))
	if !strings.Contains(context, unique) {
		t.Fatalf("contexto.md lacks the attachment:\n%s", context)
	}
	dims := []string{"business", "users", "market", "technical", "project"}
	for i, tmpl := range []string{"p03-research-business", "p04-research-users", "p05-research-market", "p06-research-technical", "p07-research-project"} {
		want, err := prompts.Render(tmpl, map[string]any{"Input": context})
		if err != nil {
			t.Fatal(err)
		}
		got := read(t, log+".research."+strconv.Itoa(i+1))
		if got != want {
			t.Errorf("research call %d is not %s on contexto.md:\n%s", i+1, tmpl, got)
		}
		if !strings.Contains(got, unique) || strings.Contains(strings.ToLower(got), "json") || !strings.Contains(got, "## Sources") {
			t.Errorf("research call %d lacks contexto.md's attachment or does not ask for prose and sources:\n%s", i+1, got)
		}
	}

	var research researchFile
	decodeFile(t, filepath.Join(dir, "research.json"), &research)
	if len(research.Dimensions) != 5 {
		t.Fatalf("research.json: %+v", research)
	}
	for i, d := range research.Dimensions {
		name := dims[i]
		want := []sourceFile{{"https://example.com/" + name, "On " + name, "The " + name + " finding."}}
		if d.Name != name || d.Text != "The "+name+" finding." || !reflect.DeepEqual(d.Sources, want) {
			t.Errorf("research.json dimension %d: %+v", i, d)
		}
	}
	var triage triageFile
	decodeFile(t, filepath.Join(dir, "triage.json"), &triage)
	if triage.ResearchDimensions != nil {
		t.Errorf("triage.json: %+v", triage)
	}
	if read(t, filepath.Join(dir, "input", "rules.md")) != unique || read(t, filepath.Join(dir, "input", "brief.md")) == "" {
		t.Error("the input did not survive research")
	}
	var report reportFile
	decodeFile(t, filepath.Join(dir, "report.json"), &report)
	if report.Status != "done" || len(report.Steps) != 6 || report.Steps[0].Name != "input" || report.Steps[1].Name != "triage" ||
		report.Steps[1].Status != "passed" || report.Steps[2].Name != "research" || report.Steps[2].Status != "passed" ||
		report.Steps[2].Model != "sonnet" || report.Steps[2].Harness != "claude" || report.Steps[2].Effort != "medium" ||
		report.Steps[2].Note != "dimensions: business, users, market, technical, project; web: on" {
		t.Errorf("report.json: %+v", report)
	}
	if !strings.Contains(read(t, filepath.Join(dir, "agent", "research.log")), finding("technical")) ||
		!strings.Contains(read(t, filepath.Join(dir, "agent", "triage.log")), "Research dimensions: ") {
		t.Error("agent logs lack the research or triage answers")
	}
	if _, err := os.Stat(filepath.Join(dir, "analysis.json")); err != nil {
		t.Error("analysis did not follow research")
	}

	before := map[string]string{}
	for _, name := range []string{"report.json", "contexto.md", "triage.json", "research.json", "agent/research.log"} {
		before[name] = read(t, filepath.Join(dir, name))
	}
	r, err := pipeline.Read(dir)
	if err == nil {
		err = pipeline.Write(dir, r)
	}
	if err != nil {
		t.Fatal(err)
	}
	for name, b := range before {
		if read(t, filepath.Join(dir, name)) != b {
			t.Errorf("%s changed through Read and Write", name)
		}
	}
}

func TestRunIgnoresDuplicateTriageDimensionBeforeResearch(t *testing.T) {
	log := runRepo(t, map[string]string{"rules.md": unique})
	triageOn(t, `["market","market"]`, `[]`)
	// The removed field cannot invalidate triage or choose the research dimensions.
	code, _, errOut := dulidu(t, "run", "brief.md", "--attach", "rules.md")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if n := calls(t, log+".research"); n != 5 {
		t.Errorf("%d research calls after duplicate legacy dimensions, want five", n)
	}
	dir := runs(t)[0]
	var triage triageFile
	decodeFile(t, filepath.Join(dir, "triage.json"), &triage)
	if triage.Complexity != "simple" || triage.Epics != 2 || triage.ResearchDimensions != nil {
		t.Errorf("removed research_dimensions affected triage: %+v", triage)
	}
	var research researchFile
	decodeFile(t, filepath.Join(dir, "research.json"), &research)
	if len(research.Dimensions) != 5 {
		t.Fatalf("research.json after duplicate legacy dimensions: %+v", research)
	}
	for i, name := range []string{"business", "users", "market", "technical", "project"} {
		if research.Dimensions[i].Name != name {
			t.Errorf("dimension %d: %s, want %s", i, research.Dimensions[i].Name, name)
		}
	}
}

func TestRunWithNoDimensionsAsksAllFiveDimensions(t *testing.T) {
	log := runRepo(t, map[string]string{"rules.md": unique})
	triageOn(t, `[]`, `[]`)
	if code, out, errOut := dulidu(t, "run", "brief.md", "--attach", "rules.md"); code != 0 {
		t.Fatalf("dulidu run: exit %d\n%s%s", code, out, errOut)
	}
	dir := runs(t)[0]
	if n := calls(t, log+".research"); n != 5 {
		t.Errorf("%d research calls, want five", n)
	}
	var research researchFile
	decodeFile(t, filepath.Join(dir, "research.json"), &research)
	if len(research.Dimensions) != 5 {
		t.Fatalf("research.json: %+v", research)
	}
	for i, name := range []string{"business", "users", "market", "technical", "project"} {
		if research.Dimensions[i].Name != name || len(research.Dimensions[i].Sources) == 0 {
			t.Errorf("dimension %d: %+v", i, research.Dimensions[i])
		}
		if !strings.Contains(read(t, filepath.Join(dir, "spec.md")), "### "+name+"\n") {
			t.Errorf("spec.md lacks research section %s", name)
		}
	}
	var report reportFile
	decodeFile(t, filepath.Join(dir, "report.json"), &report)
	if report.Status != "done" || len(report.Steps) != 6 || report.Steps[2].Name != "research" ||
		report.Steps[2].Status != "passed" || report.Steps[2].Effort != "medium" {
		t.Errorf("report.json: %+v", report)
	}
}

func TestRunWithMissingQuestionsResearches(t *testing.T) {
	log := runRepo(t, map[string]string{"rules.md": unique})
	triageOn(t, `["market"]`, `["Who sells the bikes?"]`)
	if code, out, errOut := dulidu(t, "run", "brief.md", "--attach", "rules.md"); code != 0 {
		t.Fatalf("dulidu run: exit %d\n%s%s", code, out, errOut)
	}
	dir := runs(t)[0]
	if n := calls(t, log+".research"); n != 5 {
		t.Errorf("%d research calls with triage's questions, want 5", n)
	}
	if _, err := os.Stat(filepath.Join(dir, "research.json")); err != nil {
		t.Errorf("research.json was not written with triage's questions: %v", err)
	}
	var report reportFile
	decodeFile(t, filepath.Join(dir, "report.json"), &report)
	if report.Status != "done" || len(report.Steps) != 6 || report.Steps[2].Name != "research" || report.Steps[2].Status != "passed" {
		t.Errorf("report.json: %+v", report)
	}
}

func TestRunFailsAtResearchOnEmptySources(t *testing.T) {
	log := runRepo(t, map[string]string{"rules.md": unique})
	triageOn(t, `["business","market","users"]`, `[]`)
	researchWith(t, finding("business"), "Crowded.\n\n## Sources\n", finding("users"))
	code, _, errOut := dulidu(t, "run", "brief.md", "--attach", "rules.md")
	if code != 1 || !strings.Contains(errOut, "failed at research: users: no sources") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	dir := runs(t)[0]
	if n := calls(t, log+".research"); n != 2 {
		t.Errorf("%d research calls, want 2: none after the failing one", n)
	}
	var report reportFile
	decodeFile(t, filepath.Join(dir, "report.json"), &report)
	if report.Status != "failed" || len(report.Steps) != 3 || report.Steps[1].Status != "passed" ||
		report.Steps[2].Name != "research" || report.Steps[2].Status != "failed at research: users: no sources" ||
		report.Steps[2].Effort != "medium" {
		t.Errorf("report.json: %+v", report)
	}
	for _, name := range []string{"research.json", "analysis.json", "epics.json", "spec.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("%s was written after a failed research", name)
		}
	}
	if read(t, filepath.Join(dir, "input", "rules.md")) != unique || !strings.Contains(read(t, filepath.Join(dir, "contexto.md")), unique) {
		t.Error("the input did not survive a failed research")
	}
	if _, err := os.Stat(filepath.Join(dir, "triage.json")); err != nil {
		t.Error("triage.json did not survive a failed research")
	}
	if !strings.Contains(read(t, filepath.Join(dir, "agent", "research.log")), "Crowded.\n\n## Sources\n") {
		t.Error("agent/research.log lacks the failing plain-text answer with an empty Sources section")
	}
}

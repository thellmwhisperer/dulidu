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
	return `{"text":"The ` + dimension + ` finding.","sources":[{"url":"https://example.com/` + dimension +
		`","title":"On ` + dimension + `","supports":"The ` + dimension + ` finding."}]}`
}

const unique = "Only bicycles tagged red-74 can be listed.\n"

func triageOn(t *testing.T, dimensions, questions string) {
	t.Helper()
	triageWith(t, `{"complexity":"simple","caps":{"epics":2,"stories_per_epic":3},"research_dimensions":`+dimensions+
		`,"context_present":["brief","rules.md"],"missing_questions":`+questions+`,"project_type":"new_product"}`)
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

func TestRunResearchesEachEnabledDimensionOnceWithSources(t *testing.T) {
	log := runRepo(t, map[string]string{"rules.md": unique})
	triageOn(t, `["market","users","technical"]`, `[]`)
	researchWith(t, finding("market"), finding("users"), finding("technical"))
	if code, out, errOut := dulidu(t, "run", "brief.md", "--attach", "rules.md"); code != 0 {
		t.Fatalf("dulidu run: exit %d\n%s%s", code, out, errOut)
	}
	dir := runs(t)[0]
	if n := calls(t, log+".research"); n != 3 {
		t.Fatalf("%d research calls, want 3", n)
	}
	if got := read(t, log+".research"); strings.Count(got, "--model sonnet") != 3 || strings.Count(got, "--effort medium") != 3 {
		t.Errorf("research calls did not use the research step's model and effort: %q", got)
	}
	context := read(t, filepath.Join(dir, "contexto.md"))
	if !strings.Contains(context, unique) {
		t.Fatalf("contexto.md lacks the attachment:\n%s", context)
	}
	dims := []string{"market", "users", "technical"}
	for i, tmpl := range []string{"p05-research-market", "p04-research-users", "p06-research-technical"} {
		want, err := prompts.Render(tmpl, map[string]any{"Input": context})
		if err != nil {
			t.Fatal(err)
		}
		got := read(t, log+".research."+strconv.Itoa(i+1))
		if got != want {
			t.Errorf("research call %d is not %s on contexto.md:\n%s", i+1, tmpl, got)
		}
		if !strings.Contains(got, unique) || strings.Contains(got, "no JSON") {
			t.Errorf("research call %d lacks contexto.md's attachment or asks for prose:\n%s", i+1, got)
		}
	}

	var research researchFile
	decodeFile(t, filepath.Join(dir, "research.json"), &research)
	if len(research.Dimensions) != 3 {
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
	if !reflect.DeepEqual(triage.ResearchDimensions, dims) {
		t.Errorf("triage.json: %+v", triage)
	}
	if read(t, filepath.Join(dir, "input", "rules.md")) != unique || read(t, filepath.Join(dir, "input", "brief.md")) == "" {
		t.Error("the input did not survive research")
	}
	var report reportFile
	decodeFile(t, filepath.Join(dir, "report.json"), &report)
	if report.Status != "done" || len(report.Steps) != 3 || report.Steps[0].Name != "input" || report.Steps[1].Name != "triage" ||
		report.Steps[1].Status != "passed" || report.Steps[2].Name != "research" || report.Steps[2].Status != "passed" ||
		report.Steps[2].Model != "sonnet" || report.Steps[2].Harness != "claude" ||
		report.Steps[2].Note != "dimensions: market, users, technical" {
		t.Errorf("report.json: %+v", report)
	}
	if !strings.Contains(read(t, filepath.Join(dir, "agent", "research.log")), finding("technical")) ||
		!strings.Contains(read(t, filepath.Join(dir, "agent", "triage.log")), "research_dimensions") {
		t.Error("agent logs lack the research or triage answers")
	}
	if _, err := os.Stat(filepath.Join(dir, "analysis.json")); err == nil {
		t.Error("analysis.json was written after research")
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

func TestRunWithNoDimensionsPassesResearchWithoutACall(t *testing.T) {
	log := runRepo(t, map[string]string{"rules.md": unique})
	triageOn(t, `[]`, `[]`)
	if code, out, errOut := dulidu(t, "run", "brief.md", "--attach", "rules.md"); code != 0 {
		t.Fatalf("dulidu run: exit %d\n%s%s", code, out, errOut)
	}
	dir := runs(t)[0]
	if n := calls(t, log+".research"); n != 0 {
		t.Errorf("%d research calls, want none", n)
	}
	if got := strings.Join(strings.Fields(read(t, filepath.Join(dir, "research.json"))), ""); got != `{"dimensions":[]}` {
		t.Errorf("research.json: %s", got)
	}
	var report reportFile
	decodeFile(t, filepath.Join(dir, "report.json"), &report)
	if report.Status != "done" || len(report.Steps) != 3 || report.Steps[2].Name != "research" || report.Steps[2].Status != "passed" {
		t.Errorf("report.json: %+v", report)
	}
}

func TestRunWithMissingQuestionsDoesNotResearch(t *testing.T) {
	log := runRepo(t, map[string]string{"rules.md": unique})
	triageOn(t, `["market"]`, `["Who sells the bikes?"]`)
	if code, out, errOut := dulidu(t, "run", "brief.md", "--attach", "rules.md"); code != 0 {
		t.Fatalf("dulidu run: exit %d\n%s%s", code, out, errOut)
	}
	dir := runs(t)[0]
	if n := calls(t, log+".research"); n != 0 {
		t.Errorf("%d research calls with questions pending, want none", n)
	}
	if _, err := os.Stat(filepath.Join(dir, "research.json")); err == nil {
		t.Error("research.json was written with questions pending")
	}
	var report reportFile
	decodeFile(t, filepath.Join(dir, "report.json"), &report)
	if len(report.Steps) != 2 || report.Steps[1].Name != "triage" || report.Steps[1].Status != "passed" {
		t.Errorf("report.json: %+v", report)
	}
}

func TestRunFailsAtResearchOnEmptySources(t *testing.T) {
	log := runRepo(t, map[string]string{"rules.md": unique})
	triageOn(t, `["business","market","users"]`, `[]`)
	researchWith(t, finding("business"), `{"text":"Crowded.","sources":[]}`, finding("users"))
	code, _, errOut := dulidu(t, "run", "brief.md", "--attach", "rules.md")
	if code != 1 || !strings.Contains(errOut, "failed at research: market: no sources") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	dir := runs(t)[0]
	if n := calls(t, log+".research"); n != 2 {
		t.Errorf("%d research calls, want 2: none after the failing one", n)
	}
	var report reportFile
	decodeFile(t, filepath.Join(dir, "report.json"), &report)
	if report.Status != "failed" || len(report.Steps) != 3 || report.Steps[1].Status != "passed" ||
		report.Steps[2].Name != "research" || report.Steps[2].Status != "failed at research: market: no sources" {
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
	if !strings.Contains(read(t, filepath.Join(dir, "agent", "research.log")), `"sources":[]`) {
		t.Error("agent/research.log lacks the failing answer")
	}
}

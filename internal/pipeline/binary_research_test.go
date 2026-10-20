//go:build unix

package pipeline_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
)

func TestBinaryResearchAsksFiveDimensions(t *testing.T) {
	bin, repo, replies := binary(t), realTempDir(t), t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "brief.md"), []byte("A bike exchange for red-74 bicycles.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	names := []string{"business", "users", "market", "technical", "project"}
	answers := []string{filepath.Join(replies, "triage.json")}
	if err := os.WriteFile(answers[0], []byte(`{"complexity":"simple","epics":2,"reasoning":"One exchange."}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		path := filepath.Join(replies, name+".json")
		answer := `{"text":"Finding for ` + name + `.","sources":[{"url":"https://example.com/` + name +
			`","title":"Source for ` + name + `","supports":"Finding for ` + name + `."}]}`
		if err := os.WriteFile(path, []byte(answer), 0o644); err != nil {
			t.Fatal(err)
		}
		answers = append(answers, path)
	}
	for _, step := range []string{"analysis", "epics"} {
		recorded, err := filepath.Glob(filepath.Join(binaryExample, "answers", "*-"+step+"*"))
		if err != nil || len(recorded) == 0 {
			t.Fatalf("recorded %s answers: %v, %v", step, recorded, err)
		}
		answers = append(answers, recorded...)
	}
	replay := fakeReplay(t, answers)
	t.Setenv("HOME", t.TempDir())
	for _, args := range [][]string{{"init"}, {"run", "brief.md"}} {
		cmd := exec.Command(bin, args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("dulidu %v: %v\n%s", args, err, out)
		}
	}
	dirs, _ := filepath.Glob(filepath.Join(repo, ".dulidu", "runs", "*"))
	if len(dirs) != 1 {
		t.Fatalf("run folders: %v", dirs)
	}
	r, err := pipeline.Read(dirs[0])
	if err != nil {
		t.Fatal(err)
	}
	if r.Research == nil || len(r.Research.Dimensions) != 5 {
		t.Fatalf("research without triage dimensions: %+v, want five dimensions", r.Research)
	}
	if n := binaryLines(filepath.Join(replay, "calls")); n != len(answers) {
		t.Errorf("agent calls: %d, want %d including five research calls", n, len(answers))
	}
	_, researchSpec, found := strings.Cut(r.Spec, "## Research\n")
	if !found {
		t.Fatalf("spec.md lacks research: %s", r.Spec)
	}
	researchSpec, _, _ = strings.Cut(researchSpec, "## Personas\n")
	if strings.Count(researchSpec, "### ") != 5 || strings.Contains(researchSpec, "- None") {
		t.Errorf("spec.md needs five research sections:\n%s", researchSpec)
	}
	log := r.Logs["research"]
	for i, name := range names {
		d := r.Research.Dimensions[i]
		if d.Name != name || d.Text != "Finding for "+name+"." || len(d.Sources) != 1 ||
			d.Sources[0].URL != "https://example.com/"+name {
			t.Errorf("dimension %d: %+v", i, d)
		}
		tmpl := []string{"p03-research-business", "p04-research-users", "p05-research-market", "p06-research-technical", "p07-research-project"}[i]
		prompt, err := prompts.Render(tmpl, map[string]any{"Input": r.Context})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(r.Logs["research"], prompt) != 1 || !strings.HasPrefix(log, prompt) {
			t.Errorf("research call %d must ask %s exactly once in order", i+1, name)
		}
		_, log, _ = strings.Cut(log, binaryRead(t, answers[i+1])+"\n\n")
		if !strings.Contains(researchSpec, "### "+name+"\n\nFinding for "+name+".") {
			t.Errorf("spec.md lacks the %s finding:\n%s", name, researchSpec)
		}
	}
	var triage map[string]any
	if err := json.Unmarshal([]byte(binaryRead(t, filepath.Join(dirs[0], "triage.json"))), &triage); err != nil {
		t.Fatal(err)
	}
	if _, exists := triage["research_dimensions"]; exists || strings.Contains(r.Logs["triage"], "research_dimensions") {
		t.Error("triage.json or the triage prompt still chooses research dimensions")
	}
	if r.Report.Status != "done" || r.Triage.Complexity != "simple" || r.Triage.Epics != 2 {
		t.Errorf("run did not finish with the original classification: %+v, %+v", r.Report, r.Triage)
	}
}

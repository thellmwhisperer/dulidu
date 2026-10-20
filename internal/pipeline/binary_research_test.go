//go:build unix

package pipeline_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
	// Triage and research answer in plain text; the run files remain JSON.
	answers := []string{filepath.Join(replies, "triage.json")}
	if err := os.WriteFile(answers[0], []byte("Complexity: simple\nEpics: 2\n\nOne exchange.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		path := filepath.Join(replies, name+".json")
		answer := "Finding for " + name + ".\n\n## Sources\n" +
			"- [Source for " + name + "](https://example.com/" + name + "): Finding for " + name + "."
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
			d.Sources[0].URL != "https://example.com/"+name ||
			d.Sources[0].Title != "Source for "+name || d.Sources[0].Supports != "Finding for "+name+"." {
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

// argsClaude is replayClaude that writes its arguments, one call per line, to $FAKE_REPLAY/calls.
const argsClaude = `#!/bin/sh
cat > /dev/null
echo "$*" >> "$FAKE_REPLAY/calls"
n=$(wc -l < "$FAKE_REPLAY/calls")
exec cat "$FAKE_REPLAY/$((n))"
`

func TestBinaryResearchSearchesTheWeb(t *testing.T) {
	bin, repo, replies := binary(t), realTempDir(t), t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "brief.md"), []byte("A bike exchange for red-74 bicycles.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	names := []string{"business", "users", "market", "technical", "project"}
	// Triage and research answer in plain text; the run files remain JSON.
	answers := []string{filepath.Join(replies, "triage.json")}
	if err := os.WriteFile(answers[0], []byte("Complexity: simple\nEpics: 2\n\nOne exchange.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		path := filepath.Join(replies, name+".json")
		answer := "Finding for " + name + ".\n\n## Sources\n" +
			"- [Source for " + name + "](https://example.com/" + name + "): Finding for " + name + "."
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
	replay, fake := fakeReplay(t, answers), t.TempDir()
	if err := os.WriteFile(filepath.Join(fake, "claude"), []byte(argsClaude), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
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
	lines := strings.Split(strings.TrimSuffix(binaryRead(t, filepath.Join(replay, "calls")), "\n"), "\n")
	if len(lines) != len(answers) {
		t.Fatalf("agent calls: %d, want %d", len(lines), len(answers))
	}
	for i, line := range lines {
		research := i >= 1 && i <= len(names)
		args := strings.Fields(line)
		for _, flag := range []string{"--tools", "--allowedTools"} {
			at := slices.Index(args, flag)
			if at < 0 || at+1 >= len(args) {
				t.Fatalf("call %d lacks %s: %s", i+1, flag, line)
			}
			tools := strings.Split(args[at+1], ",")
			for _, web := range []string{"WebSearch", "WebFetch"} {
				if slices.Contains(tools, web) != research {
					t.Errorf("call %d, research %v: %s %s", i+1, research, flag, args[at+1])
				}
			}
		}
	}
	const web = "Search the internet for this dimension, read the pages you cite, cite each with its url, and read no local file other than contexto.md in this folder."
	roles := []string{
		"You are a business analyst. Research the business context around this product idea: company type, revenue model, value proposition, market positioning.",
		"You are a UX researcher. Research the target users for this product: demographics, personas, needs, pain points, behavior patterns.",
		"You are a market analyst. Research the competitive landscape: existing solutions, market trends, differentiators, opportunities.",
		"You are a technical architect. Research the technical requirements: platforms, integrations, infrastructure, constraints, technology choices.",
		"You are a project strategist. Research the project scope: MVP definition, phases, key milestones, risks, timeline considerations.",
	}
	for i, tmpl := range []string{"p03-research-business", "p04-research-users", "p05-research-market", "p06-research-technical", "p07-research-project"} {
		prompt, err := prompts.Render(tmpl, map[string]any{"Input": r.Context})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(prompt, roles[i]) || strings.Count(prompt, web) != 1 {
			t.Errorf("%s lacks the prototype's role or the web sentence:\n%s", tmpl, prompt)
		}
	}
	if n := strings.Count(r.Logs["research"], web); n != len(names) {
		t.Errorf("research prompts holding the web sentence: %d, want %d", n, len(names))
	}
	var note string
	for _, s := range r.Report.Steps {
		if s.Name == "research" {
			note = s.Note
		}
	}
	if !strings.HasSuffix(note, "web: on") {
		t.Errorf("research note %q does not end with web: on", note)
	}
}

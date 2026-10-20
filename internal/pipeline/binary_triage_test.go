//go:build unix

package pipeline_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// TestBinaryTriageCapsByComplexity runs the built dulidu on the example's brief with a claude that replays the
// recorded answers, triage's replaced: the epics in triage.json are the complexity's cap, simple 3, moderate 5,
// complex 8, under max_epics, and a triage that fails is moderate. triage.json holds the classification only.
func TestBinaryTriageCapsByComplexity(t *testing.T) {
	answers, _ := filepath.Glob(filepath.Join(binaryExample, "answers", "*"))
	answers = answers[1:] // the brief alone has no image, so the input step asks nothing
	example := binaryRead(t, answers[0])
	if !strings.HasPrefix(example, "Complexity: ") {
		t.Fatalf("the recorded triage answer is not plain text: %q", example)
	}
	if strings.Contains(strings.ToLower(example), "research") {
		t.Fatal("recorded triage still chooses research dimensions")
	}
	complex := "Complexity: complex\nEpics: 12\n\nMany users.\n"
	for name, c := range map[string]struct {
		triage    string // "" is a triage that exits 1; research still runs all five dimensions
		maxEpics  string
		epics     float64
		note      string
		reasoning bool
	}{
		"complex":      {complex, "", 8, "", true},
		"agent error":  {"", "", 5, "classification failed, moderate", false},
		"invalid":      {"Complexity: huge\nEpics: 4\nResearch dimensions:\n", "", 5, "classification failed, moderate", false},
		"max_epics 2":  {complex, "2", 2, "clamped epics from 8 to max_epics 2", true},
		"no epics yet": {"Complexity: simple\nResearch dimensions: users, market\n", "", 3, "", false},
	} {
		t.Run(name, func(t *testing.T) {
			bin, repo := binary(t), realTempDir(t)
			if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("git init: %v\n%s", err, out)
			}
			if err := os.WriteFile(filepath.Join(repo, "brief.md"), []byte(binaryRead(t, filepath.Join(binaryExample, "brief.md"))), 0o644); err != nil {
				t.Fatal(err)
			}
			replayed := slices.Clone(answers)
			if c.triage != "" {
				replayed[0] = filepath.Join(t.TempDir(), "triage.txt")
				if err := os.WriteFile(replayed[0], []byte(c.triage), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			replay := fakeReplay(t, replayed)
			if c.triage == "" {
				if err := os.Remove(filepath.Join(replay, "1")); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("HOME", t.TempDir())
			run := func(args ...string) string {
				t.Helper()
				cmd := exec.Command(bin, args...)
				cmd.Dir = repo
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("dulidu %s: %v\n%s", strings.Join(args, " "), err, out)
				}
				return string(out)
			}
			run("init")
			yml := filepath.Join(repo, "dulidu.yaml")
			if err := os.WriteFile(yml, []byte(strings.Replace(binaryRead(t, yml), "max_epics:\n", "max_epics: "+c.maxEpics+"\n", 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			run("run", "brief.md")

			dirs, _ := filepath.Glob(filepath.Join(repo, ".dulidu", "runs", "*"))
			if len(dirs) != 1 {
				t.Fatalf("run folders: %v", dirs)
			}
			r, err := pipeline.Read(dirs[0])
			if err != nil {
				t.Fatal(err)
			}
			if r.Report.Status != "done" || len(r.Report.Steps) < 2 || r.Report.Steps[1].Status != "passed" ||
				!strings.HasPrefix(r.Report.Steps[1].Note, c.note) || (c.note == "") != (r.Report.Steps[1].Note == "") {
				t.Errorf("report.json: %+v", r.Report)
			}
			var triage map[string]any
			if err := json.Unmarshal([]byte(binaryRead(t, filepath.Join(dirs[0], "triage.json"))), &triage); err != nil {
				t.Fatal(err)
			}
			keys := []string{}
			for k := range triage {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			if !slices.Equal(keys, []string{"complexity", "epics", "reasoning"}) || triage["epics"] != c.epics ||
				(triage["reasoning"] == "Many users.") != c.reasoning {
				t.Errorf("triage.json: %v", triage)
			}
			if want := fmt.Sprintf("Generate EXACTLY %d epics", int(c.epics)); !strings.Contains(binaryRead(t, filepath.Join(dirs[0], "agent", "epics.log")), want) {
				t.Errorf("the outlines prompt does not say %q", want)
			}
			if r.Research == nil || len(r.Research.Dimensions) != 5 {
				t.Errorf("research must ask five dimensions even after failed triage: %+v", r.Research)
			}
			if strings.Contains(strings.SplitN(r.Logs["triage"], "Context:", 2)[0], "research_dimensions") {
				t.Error("triage prompt still asks for research dimensions")
			}
			if n := binaryLines(filepath.Join(replay, "calls")); n != len(replayed) {
				t.Errorf("claude was called %d times, want %d", n, len(replayed))
			}
		})
	}
}

// TestBinaryTriageReadsPlainText runs the built dulidu on the example's brief with a claude that replays the recorded
// answers, triage's in plain text: the triage note has no "classification failed" and triage.json holds the
// recorded complexity and epics. The triage prompt asks for that form and never says JSON.
func TestBinaryTriageReadsPlainText(t *testing.T) {
	answers, _ := filepath.Glob(filepath.Join(binaryExample, "answers", "*"))
	answers = answers[1:] // the brief alone has no image, so the input step asks nothing
	if recorded := binaryRead(t, answers[0]); !strings.HasPrefix(recorded, "Complexity: simple\nEpics: 2\n") {
		t.Fatalf("the recorded triage answer is not plain text: %q", recorded)
	}
	bin, repo := binary(t), realTempDir(t)
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "brief.md"), []byte(binaryRead(t, filepath.Join(binaryExample, "brief.md"))), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeReplay(t, answers)
	t.Setenv("HOME", t.TempDir())
	for _, args := range [][]string{{"init"}, {"run", "brief.md"}} {
		cmd := exec.Command(bin, args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("dulidu %s: %v\n%s", strings.Join(args, " "), err, out)
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
	if len(r.Report.Steps) < 2 || strings.Contains(r.Report.Steps[1].Note, "classification failed") {
		t.Errorf("report.json: %+v", r.Report)
	}
	var triage map[string]any
	if err := json.Unmarshal([]byte(binaryRead(t, filepath.Join(dirs[0], "triage.json"))), &triage); err != nil {
		t.Fatal(err)
	}
	if triage["complexity"] != "simple" || triage["epics"] != 2.0 {
		t.Errorf("triage.json: %v", triage)
	}
	prompt := strings.SplitN(r.Logs["triage"], "Context:", 2)[0]
	if strings.Contains(strings.ToLower(prompt), "json") || !strings.Contains(r.Logs["triage"], "Complexity: simple | moderate | complex\nEpics: <number>") {
		t.Errorf("the triage prompt does not ask for plain text:\n%s", r.Logs["triage"])
	}
}

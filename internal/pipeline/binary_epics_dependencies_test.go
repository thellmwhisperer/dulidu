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
)

func TestBinaryRunDropsBadDependencies(t *testing.T) {
	bin := binary(t)
	for _, duplicate := range []bool{false, true} {
		name := "unknown and cycle"
		if duplicate {
			name = "duplicate still fails"
		}
		t.Run(name, func(t *testing.T) {
			repo := realTempDir(t)
			t.Setenv("HOME", t.TempDir())
			if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("git init: %v\n%s", err, out)
			}
			if err := os.WriteFile(filepath.Join(repo, "brief.md"), []byte(binaryRead(t, filepath.Join(binaryExample, "brief.md"))), 0o644); err != nil {
				t.Fatal(err)
			}
			outlines := "## E1 Book\nDepends on: E9\nBook.\n\n## E2 Fire\nDepends on: E3\nFire.\n\n## E3 Glaze\nDepends on: E2\nGlaze.\n"
			if duplicate {
				outlines = strings.Replace(outlines, "## E3 Glaze", "## E2 Glaze", 1)
			}
			path := filepath.Join(t.TempDir(), "outlines.md")
			if err := os.WriteFile(path, []byte(outlines), 0o644); err != nil {
				t.Fatal(err)
			}
			triage := filepath.Join(filepath.Dir(path), "triage.md")
			if err := os.WriteFile(triage, []byte("Complexity: simple\nEpics: 3\n\nOne studio.\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			answers, err := filepath.Glob(filepath.Join(binaryExample, "answers", "*"))
			if err != nil {
				t.Fatal(err)
			}
			answers = answers[1:]
			answers[0] = triage
			for i, answer := range answers {
				if filepath.Base(answer) == "13-epics-outlines.md" {
					answers[i] = path
				}
			}
			answers = append(answers, filepath.Join(binaryExample, "answers", "15-epics-E2.md"))
			replay := fakeReplay(t, answers)
			cmd := exec.Command(bin, "init")
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("dulidu init: %v\n%s", err, out)
			}
			cmd = exec.Command(bin, "run", "brief.md")
			cmd.Dir = repo
			out, runErr := cmd.CombinedOutput()
			dirs, _ := filepath.Glob(filepath.Join(repo, ".dulidu", "runs", "*"))
			if len(dirs) != 1 {
				t.Fatalf("run folders: %v", dirs)
			}
			r, err := pipeline.Read(dirs[0])
			if err != nil {
				t.Fatal(err)
			}
			if duplicate {
				if runErr == nil || !strings.Contains(string(out), "duplicate epic: E2") {
					t.Fatalf("duplicate epic: %v\n%s", runErr, out)
				}
				if r.Report.Status != "failed" || r.Epics != nil || binaryLines(filepath.Join(replay, "calls")) != len(answers)-3 {
					t.Fatalf("duplicate must fail before stories: %+v", r.Report)
				}
				return
			}
			if runErr != nil {
				t.Fatalf("dulidu run: %v\n%s", runErr, out)
			}
			if r.Report.Status != "done" || len(r.Epics) != 3 || binaryLines(filepath.Join(replay, "calls")) != len(answers) {
				t.Fatalf("incomplete run: %+v, epics: %+v", r.Report, r.Epics)
			}
			for i, want := range [][]string{nil, {"E3"}, nil} {
				if r.Epics[i].ID != []string{"E1", "E2", "E3"}[i] || !slices.Equal(r.Epics[i].DependsOn, want) {
					t.Errorf("epic %d: %+v, want dependencies %v", i, r.Epics[i], want)
				}
			}
			note := ""
			for _, step := range r.Report.Steps {
				if step.Name == "epics" {
					note = step.Note
				}
			}
			if note != "dropped dependency E1 on E9; dropped dependency E3 on E2: cycle" {
				t.Errorf("epics note: %q", note)
			}
			var plan struct {
				Tickets []struct {
					Issue     int   `json:"issue"`
					DependsOn []int `json:"depends_on"`
				} `json:"tickets"`
			}
			if err := json.Unmarshal(r.Plan, &plan); err != nil {
				t.Fatal(err)
			}
			// Two E1 stories, then one each for E2 and E3: only E2 waits for E3.
			if len(plan.Tickets) != 4 {
				t.Fatalf("plan tickets: %+v", plan.Tickets)
			}
			for i, ticket := range plan.Tickets {
				var want []int
				if i == 2 {
					want = []int{plan.Tickets[3].Issue}
				}
				if !slices.Equal(ticket.DependsOn, want) {
					t.Errorf("plan ticket %d: dependencies %v, want %v", ticket.Issue, ticket.DependsOn, want)
				}
			}
		})
	}
}

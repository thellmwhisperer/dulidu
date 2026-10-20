//go:build unix

package pipeline_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
)

func TestBinaryRunReadsPlainText(t *testing.T) {
	bin := binary(t)
	for _, missingTitle := range []bool{false, true} {
		name := "every recorded answer is plain text"
		if missingTitle {
			name = "E2 without a title fails at epics"
		}
		t.Run(name, func(t *testing.T) {
			repo := realTempDir(t)
			t.Setenv("HOME", t.TempDir())
			if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("git init: %v\n%s", err, out)
			}
			for _, file := range []string{"brief.md", "one.png"} {
				if err := os.WriteFile(filepath.Join(repo, file), []byte(binaryRead(t, filepath.Join(binaryExample, file))), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			answers, err := filepath.Glob(filepath.Join(binaryExample, "answers", "*"))
			if err != nil || len(answers) != 15 {
				t.Fatalf("recorded answers: %v, %v", answers, err)
			}
			for i, path := range answers {
				if missingTitle && strings.Contains(path, "epics-outlines") {
					path = filepath.Join(t.TempDir(), "outlines.md")
					if err := os.WriteFile(path, []byte("## E1 Book a wheel\nDepends on: none\nA member books a wheel.\n\n## E2\nDepends on: E1\nPieces make Friday's kiln.\n"), 0o644); err != nil {
						t.Fatal(err)
					}
					answers[i] = path
				}
				if json.Valid([]byte(binaryRead(t, path))) {
					t.Errorf("recorded answer %s must be plain text", path)
				}
			}
			replay := fakeReplay(t, answers)
			init := exec.Command(bin, "init")
			init.Dir = repo
			if out, err := init.CombinedOutput(); err != nil {
				t.Fatalf("dulidu init: %v\n%s", err, out)
			}
			cmd := exec.Command(bin, "run", "brief.md", "--attach", "one.png")
			cmd.Dir = repo
			out, runErr := cmd.CombinedOutput()
			dirs, _ := filepath.Glob(filepath.Join(repo, ".dulidu", "runs", "*"))
			if len(dirs) != 1 {
				t.Fatalf("run folders: %v\n%s", dirs, out)
			}
			r, err := pipeline.Read(dirs[0])
			if err != nil {
				t.Fatal(err)
			}
			if missingTitle {
				var exit *exec.ExitError
				if !errors.As(runErr, &exit) || exit.ExitCode() != 1 {
					t.Fatalf("missing title must exit 1: %v\n%s", runErr, out)
				}
				for _, message := range []string{"failed at epics", "E2", "title"} {
					if !strings.Contains(string(out), message) {
						t.Errorf("error must name %q:\n%s", message, out)
					}
				}
				if r.Report.Status != "failed" || len(r.Report.Steps) != 5 ||
					!strings.HasPrefix(r.Report.Steps[4].Status, "failed at epics") || r.Epics != nil ||
					binaryLines(filepath.Join(replay, "calls")) != 13 {
					t.Fatalf("missing title must stop before stories: %+v", r.Report)
				}
				return
			}
			if runErr != nil {
				t.Fatalf("plain-text run must exit 0: %v\n%s", runErr, out)
			}
			if r.Report.Status != "done" || len(r.Report.Steps) != 6 || len(r.Epics) != 2 {
				t.Fatalf("incomplete run: %+v", r)
			}
			for _, step := range r.Report.Steps {
				if step.Status != "passed" {
					t.Errorf("%s status = %q, want passed", step.Name, step.Status)
				}
			}
			if binaryLines(filepath.Join(replay, "calls")) != len(answers) {
				t.Error("each recorded answer must be used once")
			}
			if !strings.Contains(binaryRead(t, filepath.Join(dirs[0], "spec.md")), r.Epics[0].Title) {
				t.Error("spec.md lacks the first epic's title")
			}
			if err := filepath.WalkDir(filepath.Join(dirs[0], "agent"), func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				for _, line := range strings.Split(binaryRead(t, path), "\n") {
					for _, prefix := range []string{`{"complexity"`, `[{`, `{"text"`} {
						if strings.HasPrefix(line, prefix) {
							t.Errorf("%s still contains a structured model answer: %s", path, line)
						}
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
	data := map[string]any{"Input": "A brief.", "Personas": "- P", "Functionalities": "- F1 Book",
		"MustFunctionalities": "- F1 Book", "Flows": "- Book", "Constraints": "- One hour",
		"Epics": 2, "Stories": 3, "Epic": pipeline.EpicOutline{ID: "E1", Title: "Book", Text: "Book a wheel."}}
	for tmpl, form := range map[string][]string{
		"p15-epics-outlines": {"one block per epic", "## <id> <title>", "E1", "E2", "Depends on: none", "Depends on: E1, E2", "prose", "Example:", "## E1 "},
		"p16-epics-stories":  {"one block per story", "## <id> <title>", "US1", "Functionalities: F1, F3", "prose", "### Acceptance", "\n- ", "### First test", "Example:"},
	} {
		prompt, err := prompts.Render(tmpl, data)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToLower(prompt), "json") {
			t.Errorf("%s still asks for JSON", tmpl)
		}
		for _, part := range form {
			if !strings.Contains(prompt, part) {
				t.Errorf("%s lacks %q", tmpl, part)
			}
		}
	}
}

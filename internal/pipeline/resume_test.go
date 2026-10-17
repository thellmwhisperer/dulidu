//go:build unix

package pipeline_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/thellmwhisperer/dulidu/internal/cli"
	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// TestMain makes the test binary dulidu itself when DULIDU_MAIN is set, so tests run the real command line.
func TestMain(m *testing.M) {
	if os.Getenv("DULIDU_MAIN") != "" {
		cli.Execute()
	}
	os.Exit(m.Run())
}

// fakeClaude answers the triage prompt with $FAKE_TRIAGE's line, a research prompt with a valid dimension and an
// analysis list's or epics phase's prompt validly. Each call adds a line to $FAKE_LOG.<step>, the analysis and epics
// ones to $FAKE_LOG.graph; the last research prompt is kept in $FAKE_LOG.research.prompt.
const fakeClaude = `#!/bin/sh
prompt=$(cat)
a=
case "$prompt" in
*'Extract target user personas'*) a='[{"name":"Lead","role":"Team lead","goals":["Plan"],"painPoints":["Chaos"]}]' ;;
*'Extract key functionalities'*) a='[{"name":"Plan a sprint","description":"A lead plans a sprint.","priority":"must"}]' ;;
*'Extract user flows'*) a='[{"name":"Plan","steps":["Open","Plan"]}]' ;;
*'Extract project constraints'*) a='["Teams of five"]' ;;
*'Identify open questions'*) a='["Which calendar?"]' ;;
*'who thinks about HOW'*) a='[{"id":"E1","title":"Sprints","description":"Teams plan sprints.","depends_on":[]}]' ;;
*'implementation-ready tickets'*) a='[{"id":"S1","title":"Plan a sprint","description":"A lead plans a sprint.","acceptance":["The sprint shows up."],"first_test":"TestPlanSprint","functionalities":["F1"]}]' ;;
esac
if [ -n "$a" ]; then
  echo >> "$FAKE_LOG.graph"
  printf '{"type":"result","result":"%s"}\n' "$(printf '%s' "$a" | sed 's/"/\\"/g')"
  exit 0
fi
case "$prompt" in
*"You triage a product brief"*)
  echo >> "$FAKE_LOG.triage"
  cat "$FAKE_TRIAGE" ;;
*"Research the following product idea"*)
  echo >> "$FAKE_LOG.research"
  printf '%s' "$prompt" > "$FAKE_LOG.research.prompt"
  echo '{"type":"result","result":"{\"text\":\"Teams plan.\",\"sources\":[{\"url\":\"https://example.com/teams\",\"title\":\"Teams\",\"supports\":\"Teams plan.\"}]}"}' ;;
*) exit 1 ;;
esac
`

// thinTriage is triage's answer on the thin brief: three questions before going on.
const thinTriage = `{"complexity":"simple","caps":{"epics":2,"stories_per_epic":3},"research_dimensions":["users"],` +
	`"context_present":["brief"],"missing_questions":["What do teams do with the app?","Who are the users?",` +
	`"How many teams are there?"],"project_type":"new_product"}`

// dulidu runs dulidu with args in dir and returns its exit code, stdout and stderr.
func dulidu(t *testing.T, dir string, args ...string) (int, string, string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	cmd := exec.Command(self, args...)
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = dir, append(os.Environ(), "DULIDU_MAIN=1"), &out, &errOut
	if err := cmd.Run(); err != nil && !errors.As(err, new(*exec.ExitError)) {
		t.Fatal(err)
	}
	return cmd.ProcessState.ExitCode(), out.String(), errOut.String()
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// lines is how many lines path has, 0 when it is not there.
func lines(path string) int {
	b, _ := os.ReadFile(path)
	return strings.Count(string(b), "\n")
}

func TestStopAndResume(t *testing.T) {
	repo, bin, log := t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "calls")
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	triage := filepath.Join(t.TempDir(), "triage")
	line, _ := json.Marshal(map[string]string{"type": "result", "result": thinTriage})
	for path, s := range map[string]string{filepath.Join(bin, "claude"): fakeClaude, triage: string(line) + "\n",
		filepath.Join(repo, "brief.md"):   "An app for teams.\n",
		filepath.Join(repo, "answers.md"): "Teams of five plan their sprints.\n"} {
		if err := os.WriteFile(path, []byte(s), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_LOG", log)
	t.Setenv("FAKE_TRIAGE", triage)
	if code, out, errOut := dulidu(t, repo, "init"); code != 0 {
		t.Fatalf("dulidu init: exit %d\n%s%s", code, out, errOut)
	}

	code, out, errOut := dulidu(t, repo, "run", "brief.md")
	if code != 0 {
		t.Fatalf("dulidu run: exit %d\n%s%s", code, out, errOut)
	}
	dirs, _ := filepath.Glob(filepath.Join(repo, ".dulidu", "runs", "*"))
	if len(dirs) != 1 {
		t.Fatalf("run folders: %v", dirs)
	}
	dir, label := dirs[0], filepath.Base(dirs[0])
	stopped, err := pipeline.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Report.Status != "waiting_answers" || len(stopped.Report.Steps) != 2 || stopped.Report.Steps[1].Name != "triage" ||
		stopped.Report.Steps[1].Status != "passed" {
		t.Fatalf("report.json after triage: %+v", stopped.Report)
	}
	var questions []string
	if b, err := os.ReadFile(filepath.Join(dir, "questions.json")); err != nil || json.Unmarshal(b, &questions) != nil ||
		len(questions) != 3 || !slices.Equal(questions, stopped.Triage.MissingQuestions) || !strings.HasPrefix(out, string(b)) {
		t.Fatalf("questions.json %q (%v) is not triage's 3 questions as printed:\n%s", b, err, out)
	}
	if lines(log+".research") != 0 || stopped.Research != nil {
		t.Fatal("research ran with questions pending")
	}
	kept := map[string]string{}
	for _, name := range []string{"input/brief.md", "triage.json", "questions.json", "agent/triage.log"} {
		kept[name] = read(t, filepath.Join(dir, name))
	}

	if code, out, errOut := dulidu(t, repo, "run", "--resume", label, "--answers", "answers.md"); code != 0 {
		t.Fatalf("dulidu run --resume: exit %d\n%s%s", code, out, errOut)
	}
	resumed, err := pipeline.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	steps := resumed.Report.Steps
	if resumed.Report.Status != "done" || len(steps) != 5 || !reflect.DeepEqual(steps[:2], stopped.Report.Steps) ||
		steps[2].Name != "research" || steps[3].Name != "analysis" || steps[4].Name != "epics" {
		t.Errorf("report.json after resume: %+v", resumed.Report)
	}
	for _, s := range steps[1:] {
		if s.Status != "passed" {
			t.Errorf("step %s after resume: %s", s.Name, s.Status)
		}
	}
	if lines(log+".graph") != 7 || resumed.Analysis == nil || len(resumed.Epics) != 1 || resumed.Spec == "" ||
		len(resumed.Plan) == 0 || resumed.DesignBrief == "" {
		t.Errorf("resume did not reach export: %+v", resumed)
	}
	if _, err := os.Stat(filepath.Join(dir, "tickets", "1.md")); err != nil {
		t.Errorf("resume wrote no ticket: %v", err)
	}
	if n := lines(log + ".triage"); n != 1 {
		t.Errorf("triage ran %d times, want once", n)
	}
	if want := stopped.Context + "\n## Answers\n\nTeams of five plan their sprints.\n"; resumed.Context != want {
		t.Errorf("contexto.md after resume:\n%s", resumed.Context)
	}
	if lines(log+".research") != 1 || !strings.Contains(read(t, log+".research.prompt"), "## Answers") ||
		resumed.Research == nil || len(resumed.Research.Dimensions) != 1 || resumed.Logs["research"] == "" {
		t.Errorf("research did not run once on the answers: %+v", resumed.Research)
	}
	for name, s := range kept {
		if read(t, filepath.Join(dir, name)) != s {
			t.Errorf("%s changed on resume", name)
		}
	}

	code, _, errOut = dulidu(t, repo, "run", "--resume", label)
	if want := "run " + label + " is done, nothing to resume"; code != 1 || !strings.Contains(errOut, want) {
		t.Errorf("resuming a passed run: exit %d, stderr %q", code, errOut)
	}
	if lines(log+".triage") != 1 || lines(log+".research") != 1 || read(t, filepath.Join(dir, "contexto.md")) != resumed.Context {
		t.Error("resuming a passed run ran a step or changed contexto.md")
	}
}

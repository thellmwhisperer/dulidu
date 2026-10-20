//go:build unix

package pipeline_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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

// thinTriage is triage's answer on the thin brief, with three questions triage.json does not keep and never stop the run.
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

// realTempDir is t.TempDir() with its symlinks resolved, as git rev-parse --show-toplevel prints it: on macOS
// /var/folders is /private/var/folders.
func realTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestBinaryRunNeverStopsAtTriage runs the built dulidu on the example's brief with a claude that replays the
// recorded answers, triage's with one missing question: the run goes on to the export, and --resume is no flag.
func TestBinaryRunNeverStopsAtTriage(t *testing.T) {
	bin, repo := binary(t), realTempDir(t)
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "brief.md"), []byte(read(t, filepath.Join(binaryExample, "brief.md"))), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "a.md"), []byte("Bike shops print the tags.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The brief alone has no image, so the input step asks nothing; triage asks one question.
	answers, _ := filepath.Glob(filepath.Join(binaryExample, "answers", "*"))
	answers = answers[1:]
	var triage map[string]any
	if err := json.Unmarshal([]byte(read(t, answers[0])), &triage); err != nil {
		t.Fatal(err)
	}
	triage["missing_questions"] = []string{"Who prints the tags?"}
	asked, _ := json.Marshal(triage)
	answers[0] = filepath.Join(t.TempDir(), "triage.json")
	if err := os.WriteFile(answers[0], asked, 0o644); err != nil {
		t.Fatal(err)
	}
	replay := fakeReplay(t, answers)
	t.Setenv("HOME", t.TempDir())
	run := func(args ...string) (string, error) {
		cmd := exec.Command(bin, args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := run("init"); err != nil {
		t.Fatalf("dulidu init: %v\n%s", err, out)
	}

	out, err := run("run", "brief.md")
	if err != nil {
		t.Fatalf("dulidu run: %v\n%s", err, out)
	}
	if n := lines(filepath.Join(replay, "calls")); n != len(answers) {
		t.Errorf("claude was called %d times, want %d", n, len(answers))
	}
	dirs, _ := filepath.Glob(filepath.Join(repo, ".dulidu", "runs", "*"))
	if len(dirs) != 1 {
		t.Fatalf("run folders: %v", dirs)
	}
	dir := dirs[0]
	if want := filepath.Join(dir, "contexto.md") + "\n"; out != want {
		t.Errorf("dulidu run printed %q, want %q", out, want)
	}
	r, err := pipeline.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, s := range r.Report.Steps {
		names = append(names, s.Name)
		if s.Name != "input" && s.Status != "passed" {
			t.Errorf("step %s: %s", s.Name, s.Status)
		}
	}
	if r.Report.Status != "done" || strings.Join(names, " ") != "input triage research analysis epics export" {
		t.Errorf("dulidu run: status %s, steps %v, want done through the export", r.Report.Status, names)
	}
	if got := read(t, filepath.Join(dir, "triage.json")); strings.Contains(got, "missing_questions") || !strings.Contains(got, `"epics": 2`) {
		t.Errorf("triage.json keeps the question or loses the epics:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "questions.json")); err == nil {
		t.Error("questions.json was written")
	}
	if strings.Contains(r.Context, "## Answers") {
		t.Errorf("contexto.md has answers:\n%s", r.Context)
	}

	out, err = run("run", "--resume", "x", "--answers", "a.md")
	if err == nil || !strings.Contains(out, "unknown flag: --resume") {
		t.Errorf("dulidu run --resume x --answers a.md: %v\n%s", err, out)
	}
	if n := lines(filepath.Join(replay, "calls")); n != len(answers) {
		t.Errorf("--resume called claude: %d calls, want %d", n, len(answers))
	}
}

// TestThinTriageGoesOnToTheExport runs dulidu on a thin brief whose triage asks three questions: research, analysis,
// epics and the export run once each on contexto.md as input packaged it, and the open questions are analysis' list.
func TestThinTriageGoesOnToTheExport(t *testing.T) {
	repo, bin, log := realTempDir(t), t.TempDir(), filepath.Join(t.TempDir(), "calls")
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	triage := filepath.Join(t.TempDir(), "triage")
	line, _ := json.Marshal(map[string]string{"type": "result", "result": thinTriage})
	for path, s := range map[string]string{filepath.Join(bin, "claude"): fakeClaude, triage: string(line) + "\n",
		filepath.Join(repo, "brief.md"): "An app for teams.\n"} {
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
	if want := filepath.Join(dir, "contexto.md") + "\n"; out != want {
		t.Errorf("dulidu run printed %q, want %q", out, want)
	}
	r, err := pipeline.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	steps := r.Report.Steps
	if r.Report.Status != "done" || len(steps) != 6 || steps[1].Name != "triage" || steps[2].Name != "research" ||
		steps[3].Name != "analysis" || steps[4].Name != "epics" || steps[5].Name != "export" {
		t.Fatalf("report.json: %+v", r.Report)
	}
	for _, s := range steps[1:] {
		if s.Status != "passed" {
			t.Errorf("step %s: %s", s.Name, s.Status)
		}
	}
	if got := read(t, filepath.Join(dir, "triage.json")); strings.Contains(got, "missing_questions") || !strings.Contains(got, `"epics": 3`) {
		t.Errorf("triage.json keeps the questions or loses the epics:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "questions.json")); err == nil {
		t.Error("questions.json was written")
	}
	if n := lines(log + ".triage"); n != 1 {
		t.Errorf("triage ran %d times, want once", n)
	}
	if lines(log+".research") != 1 || strings.Contains(read(t, log+".research.prompt"), "## Answers") ||
		r.Research == nil || len(r.Research.Dimensions) != 1 {
		t.Errorf("research did not run once on contexto.md: %+v", r.Research)
	}
	if strings.Contains(r.Context, "## Answers") {
		t.Errorf("contexto.md has answers:\n%s", r.Context)
	}
	if lines(log+".graph") != 7 || r.Analysis == nil || len(r.Epics) != 1 || r.Spec == "" || len(r.Plan) == 0 ||
		r.DesignBrief == "" {
		t.Fatalf("the run did not reach export: %+v", r)
	}
	if _, err := os.Stat(filepath.Join(dir, "tickets", "1.md")); err != nil {
		t.Errorf("the run wrote no ticket: %v", err)
	}
	if !slices.Equal(r.Analysis.Questions, []string{"Which calendar?"}) || !strings.Contains(r.Spec, "Which calendar?") {
		t.Errorf("open questions are not analysis' list: %q\n%s", r.Analysis.Questions, r.Spec)
	}
	for _, q := range []string{"What do teams do with the app?", "Who are the users?", "How many teams are there?"} {
		if strings.Contains(r.Spec, q) {
			t.Errorf("spec.md has triage's question %q", q)
		}
	}

	code, _, errOut = dulidu(t, repo, "run", "--resume", label)
	if code == 0 || !strings.Contains(errOut, "unknown flag: --resume") {
		t.Errorf("dulidu run --resume: exit %d, stderr %q", code, errOut)
	}
	if lines(log+".triage") != 1 || lines(log+".research") != 1 || read(t, filepath.Join(dir, "contexto.md")) != r.Context {
		t.Error("dulidu run --resume ran a step or changed contexto.md")
	}
}

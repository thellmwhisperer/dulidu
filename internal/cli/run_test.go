//go:build unix

package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// fakeDescriber is the run's claude. The triage prompt's call it appends to $FAKE_LOG.triage, keeps its prompt in
// $FAKE_LOG.prompt and answers with $FAKE_TRIAGE's line, else a valid plain-text triage, simple with no epics line, so
// simple's cap of 3. The nth research prompt's call it appends to $FAKE_LOG.research, keeps its prompt in $FAKE_LOG.research.<n> and answers with $FAKE_RESEARCH/<n>'s line,
// else a valid dimension. An analysis list's or epics phase's call it appends to $FAKE_LOG.graph, as the list or phase
// and its arguments, and answers validly, failing the agent for the one named $FAKE_FAIL. Every other call is the
// input step's: it appends its arguments to $FAKE_LOG and describes any image the same.
const fakeDescriber = `#!/bin/sh
prompt=$(mktemp)
cat > "$prompt"
` + fakeGraph + `
if grep -q '^You triage a product brief' "$prompt"; then
  printf '%s\n' "$*" >> "$FAKE_LOG.triage"
  mv "$prompt" "$FAKE_LOG.prompt"
  if [ -n "$FAKE_TRIAGE" ]; then cat "$FAKE_TRIAGE"; exit 0; fi
  printf '%s\n' '{"type":"result","result":"Complexity: simple\nResearch dimensions: business\nContext present: brief\nMissing questions:\nProject type: new_product\n\nA fake triage."}'
  exit 0
fi
if grep -q '^Research the following product idea' "$prompt"; then
  printf '%s\n' "$*" >> "$FAKE_LOG.research"
  n=$(wc -l < "$FAKE_LOG.research" | tr -d ' ')
  mv "$prompt" "$FAKE_LOG.research.$n"
  if [ -n "$FAKE_RESEARCH" ]; then cat "$FAKE_RESEARCH/$n"; exit 0; fi
  # The event is JSON; the model answer inside it is prose with a Sources list.
  printf '%s\n' '{"type":"result","result":"A fake finding.\n\n## Sources\n- [Fake](https://example.com/fake): A fake finding."}'
  exit 0
fi
rm -f "$prompt"
printf '%s\n' "$*" >> "$FAKE_LOG"
echo '{"type":"result","result":"A fake description."}'
`

// Model answers use plain-text blocks; only the agent transport wraps them in a structured result event.
// fakeGraph answers the analysis and epics prompts in $prompt, as fakeDescriber says.
const fakeGraph = `case $(head -n 1 "$prompt") in
*'Extract target user personas'*) list=personas a='## Seller\nRole: Bike owner\n### Goals\n- Sell fast\n### Pain points\n- No buyers' ;;
*'Extract key functionalities'*) list=functionalities a='## F1 List a bike\nPriority: must\nA seller lists a bike.' ;;
*'Extract user flows'*) list=flows a='## FL1 Sell\n1. List\n2. Sell' ;;
*'Extract project constraints'*) list=constraints a='- Spain only' ;;
*'Identify open questions'*) list=questions a='- Which payments?' ;;
*'who thinks about HOW'*) list=outlines a='## E1 Listings\nDepends on: none\nBikes get listed.' ;;
*'implementation-ready tickets'*) list=stories a='## S1 List a bike\nFunctionalities: F1\nA seller lists a bike.\n### Acceptance\n- The bike shows up.\n### First test\nTestListBike' ;;
*) list= ;;
esac
if [ -n "$list" ]; then
  rm -f "$prompt"
  printf '%s %s\n' "$list" "$*" >> "$FAKE_LOG.graph"
  if [ "$list" = "$FAKE_FAIL" ]; then
    echo 'fake agent failed' >&2
    exit 1
  fi
  printf '{"type":"result","result":"%s"}\n' "$(printf '%s' "$a" | sed 's/"/\\"/g')"
  exit 0
fi`

// runRepo is a repository from dulidu init with a brief and files by name, its claude fakeDescriber; it returns
// where the fake logs its calls.
func runRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := repo(t)
	if code, out, errOut := dulidu(t, "init"); code != 0 {
		t.Fatalf("dulidu init: exit %d\n%s%s", code, out, errOut)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(fakeDescriber), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	calls := filepath.Join(t.TempDir(), "calls")
	t.Setenv("FAKE_LOG", calls)
	files["brief.md"] = "A shop for second-hand bikes.\n"
	for name, text := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return calls
}

// runs is the run folders in .dulidu/runs.
func runs(t *testing.T) []string {
	t.Helper()
	dirs, _ := filepath.Glob(filepath.Join(".dulidu", "runs", "*"))
	return dirs
}

func TestRunPackagesTextAttachmentsTheSameEveryTime(t *testing.T) {
	calls := runRepo(t, map[string]string{"notes.md": "Spain first.\n", "data.json": `{"bikes": 3}` + "\n",
		"v1.0.md": "First version.\n", "v10.md": "Tenth version.\n"})
	args := []string{"run", "brief.md", "--attach", "notes.md", "--attach", "data.json", "--attach", "v1.0.md",
		"--attach", "v10.md"}
	for range 2 {
		if code, out, errOut := dulidu(t, args...); code != 0 {
			t.Fatalf("dulidu run: exit %d\n%s%s", code, out, errOut)
		}
	}
	dirs := runs(t)
	if len(dirs) != 2 {
		t.Fatalf("want 2 run folders, got %v", dirs)
	}
	first, second := read(t, filepath.Join(dirs[0], "contexto.md")), read(t, filepath.Join(dirs[1], "contexto.md"))
	if first != second {
		t.Fatalf("contexto.md differs between two runs:\n%s\n---\n%s", first, second)
	}
	for _, want := range []string{"Spain first.", `{"bikes": 3}`, "Tenth version.", "A shop for second-hand bikes."} {
		if !strings.Contains(first, want) {
			t.Errorf("contexto.md lacks %q:\n%s", want, first)
		}
	}
	if n := strings.Count(first, "\n## "); n != 5 {
		t.Errorf("want one heading per file, 5, got %d:\n%s", n, first)
	}
	targets := map[string]bool{}
	for _, name := range []string{"v1.0.md", "v10.md"} {
		row := regexp.MustCompile(`(?m)^\| ` + regexp.QuoteMeta(name) + ` \|.*\]\(#([^)]+)\) \|$`).FindStringSubmatch(first)
		if row == nil {
			t.Fatalf("no index row for %s:\n%s", name, first)
		}
		if !strings.Contains(first, "<a id=\""+row[1]+"\"></a>\n\n## "+name+"\n") {
			t.Errorf("index target #%s of %s is not its heading:\n%s", row[1], name, first)
		}
		targets[row[1]] = true
	}
	if len(targets) != 2 {
		t.Errorf("v1.0.md and v10.md share an index target:\n%s", first)
	}
	if _, err := os.Stat(calls); err == nil {
		t.Error("text attachments called the agent")
	}
}

func TestRunRejectsAnUnsupportedAttachmentBeforeAnyAgentCall(t *testing.T) {
	calls := runRepo(t, map[string]string{"foo.xyz": "?", "shot.png": "png"})
	code, _, errOut := dulidu(t, "run", "brief.md", "--attach", "shot.png", "--attach", "foo.xyz")
	if code != 1 || !strings.Contains(errOut, "unsupported attachment: foo.xyz") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if _, err := os.Stat(calls); err == nil {
		t.Error("the agent was called")
	}
	if dirs := runs(t); len(dirs) != 0 {
		t.Errorf("a run folder was written: %v", dirs)
	}
}

func TestRunRejectsTwoAttachmentsWithOneName(t *testing.T) {
	calls := runRepo(t, map[string]string{"a/notes.md": "one\n", "b/notes.md": "two\n", "c/shot.png": "png"})
	code, _, errOut := dulidu(t, "run", "brief.md", "--attach", "c/shot.png", "--attach", "a/notes.md",
		"--attach", "b/notes.md")
	if code != 1 || !strings.Contains(errOut, "duplicate attachment name: notes.md") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if _, err := os.Stat(calls); err == nil {
		t.Error("the agent was called")
	}
	if dirs := runs(t); len(dirs) != 0 {
		t.Errorf("a run folder was written: %v", dirs)
	}
}

func TestRunDescribesAnImageWithTheInputStepsAgent(t *testing.T) {
	png, err := os.ReadFile(filepath.Join("..", "pipeline", "input", "testdata", "pattern.png"))
	if err != nil {
		t.Fatal(err)
	}
	calls := runRepo(t, map[string]string{"pattern.png": string(png)})
	if code, out, errOut := dulidu(t, "run", "brief.md", "--attach", "pattern.png"); code != 0 {
		t.Fatalf("dulidu run: exit %d\n%s%s", code, out, errOut)
	}
	dirs := runs(t)
	if len(dirs) != 1 {
		t.Fatalf("want 1 run folder, got %v", dirs)
	}
	context := read(t, filepath.Join(dirs[0], "contexto.md"))
	if !strings.Contains(context, "## pattern.png\n") || !strings.Contains(context, "### Description\n\nA fake description.\n") {
		t.Fatalf("no description under the image's heading:\n%s", context)
	}
	if got := read(t, calls); strings.Count(got, "\n") != 1 || !strings.Contains(got, "--model haiku") {
		t.Fatalf("want one call to the input step's model, got %q", got)
	}
	if log := read(t, filepath.Join(dirs[0], "agent", "input.log")); !strings.Contains(log, "input/pattern.png") {
		t.Fatalf("agent/input.log: %q", log)
	}
	if got := read(t, filepath.Join(dirs[0], "input", "pattern.png")); got != string(png) {
		t.Fatal("input/pattern.png is not the attachment")
	}
}

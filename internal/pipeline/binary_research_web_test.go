//go:build unix

package pipeline_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
	"github.com/thellmwhisperer/dulidu/internal/pipeline/prompts"
)

// Each research call records its argv separately, preserving spaces and newlines.
const otherResearchAgent = `#!/bin/sh
echo >> "$FAKE_RESEARCH/calls"
n=$(wc -l < "$FAKE_RESEARCH/calls")
printf '%s\0' "$@" > "$FAKE_RESEARCH/args-$((n))"
case "$(basename "$0")" in
  codex)
    while [ "$#" -gt 0 ]; do
      if [ "$1" = "-o" ]; then cp "$FAKE_RESEARCH/answer" "$2"; exit; fi
      shift
    done
    exit 1
    ;;
  hermes)
    read -r line
    echo '{"jsonrpc":"2.0","id":1,"result":{}}'
    read -r line
    echo '{"jsonrpc":"2.0","id":2,"result":{"sessionId":"research"}}'
    read -r line
    cat "$FAKE_RESEARCH/event"
    echo '{"jsonrpc":"2.0","id":3,"result":{"stopReason":"end_turn"}}'
    cat > /dev/null
    ;;
  *) cat "$FAKE_RESEARCH/event" ;;
esac
`

func TestBinaryResearchWebOtherHarness(t *testing.T) {
	bin := binary(t)
	for _, harness := range []string{"opencode", "codex", "pi", "cursor", "hermes"} {
		t.Run(harness, func(t *testing.T) {
			repo, fake := realTempDir(t), t.TempDir()
			t.Setenv("HOME", t.TempDir())
			if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("git init: %v\n%s", err, out)
			}
			// Claude still answers every step except research.
			var answers []string
			for _, step := range []string{"triage", "analysis", "epics", "design"} {
				paths, err := filepath.Glob(filepath.Join(binaryExample, "answers", "*-"+step+"*"))
				if err != nil || len(paths) == 0 {
					t.Fatalf("%s answers: %v, %v", step, paths, err)
				}
				answers = append(answers, paths...)
			}
			replay := fakeReplay(t, answers)
			name := harness
			if name == "cursor" {
				name = "cursor-agent"
			}
			// All harnesses deliver the same prose and Sources list to research.
			answer := "A finding.\n\n## Sources\n" +
				"- [Source](https://example.com): A finding."
			text := map[string]string{"type": "text", "text": answer}
			var event any
			switch harness {
			case "opencode":
				event = map[string]any{"type": "text", "part": text}
			case "pi", "cursor":
				kind := "message_end"
				if harness == "cursor" {
					kind = "assistant"
				}
				event = map[string]any{"type": kind, "message": map[string]any{
					"role": "assistant", "content": []any{text}}}
			case "hermes":
				event = map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{
					"update": map[string]any{"sessionUpdate": "agent_message_chunk", "content": text}}}
			}
			line, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			for path, content := range map[string]string{
				filepath.Join(fake, name):       otherResearchAgent,
				filepath.Join(fake, "answer"):   answer,
				filepath.Join(fake, "event"):    string(line) + "\n",
				filepath.Join(repo, "brief.md"): binaryRead(t, filepath.Join(binaryExample, "brief.md")),
			} {
				if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("FAKE_RESEARCH", fake)
			t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
			run := func(args ...string) {
				t.Helper()
				cmd := exec.Command(bin, args...)
				cmd.Dir = repo
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("dulidu %v: %v\n%s", args, err, out)
				}
			}
			run("init")
			config := filepath.Join(repo, "dulidu.yaml")
			content := binaryRead(t, config)
			content = strings.Replace(content, "research:\n    harness: claude\n    model: sonnet\n    effort: medium",
				"research:\n    harness: "+harness+"\n    model: ''\n    effort: ''", 1)
			if err := os.WriteFile(config, []byte(content), 0o644); err != nil {
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
			if r.Report.Status != "done" || binaryLines(filepath.Join(fake, "calls")) != 5 ||
				binaryLines(filepath.Join(replay, "calls")) != len(answers) {
				t.Fatalf("run status %q or unexpected agent call counts", r.Report.Status)
			}
			want := "web: off for " + harness
			if harness == "codex" {
				want = "web: on"
			}
			for _, step := range r.Report.Steps {
				if step.Name == "research" && (step.Harness != harness || !strings.HasSuffix(step.Note, want)) {
					t.Errorf("research harness %q, note %q; want %s, ending %q", step.Harness, step.Note, harness, want)
				}
			}
			for i, tmpl := range []string{"p03-research-business", "p04-research-users", "p05-research-market", "p06-research-technical", "p07-research-project"} {
				args := strings.Split(strings.TrimSuffix(binaryRead(t, filepath.Join(fake, "args-"+strconv.Itoa(i+1))), "\x00"), "\x00")
				if harness == "codex" {
					if len(args) < 4 || !slices.Equal(args[:4], []string{"exec", "-c", `web_search="live"`, "--json"}) {
						t.Errorf("research call %d lacks codex exec -c web_search=\"live\": %q", i+1, args)
					}
					continue
				}
				wantArgs := map[string][]string{
					"opencode": {"run", "--pure", "--format", "json", "--dir", dirs[0]},
					"pi":       {"-p", "--mode", "json", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files", "--no-session"},
					"cursor":   {"-p", "--output-format", "stream-json", "--trust", "--force", "--workspace", dirs[0]},
					"hermes":   {"--ignore-rules", "acp"},
				}[harness]
				if harness != "hermes" {
					prompt, err := prompts.Render(tmpl, map[string]any{"Input": r.Context})
					if err != nil {
						t.Fatal(err)
					}
					wantArgs = append(wantArgs, prompt)
				}
				if !slices.Equal(args, wantArgs) {
					t.Errorf("research call %d changed %s arguments: %q, want %q", i+1, harness, args, wantArgs)
				}
			}
		})
	}
}

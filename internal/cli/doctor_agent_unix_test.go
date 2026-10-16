//go:build unix

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeAgent is every harness doctor --agent tests: it writes its arguments to $FAKE_CALL, fails when it inherits
// CLAUDE_CODE_CHILD_SESSION, then plays $FAKE_MODE: no-auth exits 2, hang writes its group to $FAKE_GROUP and
// waits on a child that never ends, any other answers OK the way its harness does.
const fakeAgent = `#!/bin/sh
printf '%s\n' "$*" > "$FAKE_CALL"
[ -z "$CLAUDE_CODE_CHILD_SESSION" ] || { echo 'inherited CLAUDE_CODE_CHILD_SESSION' >&2; exit 5; }
case "$FAKE_MODE" in
no-auth) echo 'no auth' >&2; exit 2 ;;
hang) tail -f /dev/null & echo $$ > "$FAKE_GROUP"; wait; exit 0 ;;
esac
case "${0##*/}" in
claude) grep -q OK && echo '{"type":"result","result":"OK"}' ;;
codex)
	git rev-parse --is-inside-work-tree >/dev/null 2>&1 || { echo 'not a git repository' >&2; exit 2; }
	while [ $# -gt 0 ]; do [ "$1" = -o ] && printf OK > "$2"; shift; done
	echo '{"type":"thread.started"}' ;;
pi) echo '{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"OK"}]}}' ;;
esac
`

// agentRepo is a repository whose dulidu.yaml has harness, model m1, effort high and extra, its agent fakeAgent
// playing mode; it returns where the fake writes its call.
func agentRepo(t *testing.T, harness, mode, extra string) string {
	t.Helper()
	dir := repo(t)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, harness), []byte(fakeAgent), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	call := filepath.Join(t.TempDir(), "call")
	t.Setenv("FAKE_CALL", call)
	t.Setenv("FAKE_MODE", mode)
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "1")
	yml := "harness: " + harness + "\nmodel: m1\neffort: high\n" + extra
	if err := os.WriteFile(filepath.Join(dir, "dulidu.yaml"), []byte(yml), 0o600); err != nil {
		t.Fatal(err)
	}
	return call
}

func lastLine(out string) string {
	ls := strings.Split(strings.TrimSpace(out), "\n")
	return ls[len(ls)-1]
}

func TestDoctorAgentSaysHelloToEachHarness(t *testing.T) {
	for _, c := range []struct{ harness, model, effort string }{
		{"claude", "--model m1", "--effort high"},
		{"codex", `model="m1"`, `model_reasoning_effort="high"`},
		{"pi", "--model m1", "--thinking high"},
	} {
		t.Run(c.harness, func(t *testing.T) {
			call := agentRepo(t, c.harness, "ok", "")
			code, out, errOut := dulidu(t, "doctor", "--agent")
			if code != 0 || !strings.HasPrefix(lastLine(out), "ok agent: "+c.harness+" m1 answered in ") ||
				!strings.HasSuffix(lastLine(out), " s") {
				t.Fatalf("dulidu doctor --agent: exit %d\n%s%s", code, out, errOut)
			}
			args := read(t, call)
			if !strings.Contains(args, c.model) || !strings.Contains(args, c.effort) {
				t.Fatalf("want model and effort untouched, got %q", args)
			}
		})
	}
}

func TestDoctorAgentFailsWithTheCauseAndTheAgentsLastLines(t *testing.T) {
	agentRepo(t, "claude", "no-auth", "")
	code, out, _ := dulidu(t, "doctor", "--agent")
	if got := lastLine(out); code != 1 || got != "FAIL agent: claude: exit code 2: no auth" {
		t.Fatalf("exit %d, got %q\n%s", code, got, out)
	}
}

func TestDoctorAgentTimeoutKillsTheWholeGroup(t *testing.T) {
	agentRepo(t, "claude", "hang", "agent_timeout: 1s\n")
	group := filepath.Join(t.TempDir(), "group")
	t.Setenv("FAKE_GROUP", group)
	code, out, _ := dulidu(t, "doctor", "--agent")
	if got := lastLine(out); code != 1 || !strings.HasPrefix(got, "FAIL agent: claude: timed out after 0 min 01 s") {
		t.Fatalf("exit %d, got %q\n%s", code, got, out)
	}
	pgid := strings.TrimSpace(read(t, group))
	for deadline := time.Now().Add(5 * time.Second); ; {
		left, _ := exec.Command("pgrep", "-g", pgid).Output()
		if len(left) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("processes of group %s outlived the timeout: %s", pgid, left)
		}
	}
}

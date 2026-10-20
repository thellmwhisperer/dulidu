package run

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestResearchWebArgvKeepsOtherStepsUnchanged(t *testing.T) {
	for harness, base := range map[string][]string{
		"codex":    {"codex", "exec", "--json", "-C", "/d", "--sandbox", "workspace-write", "-o", "/f", "-c", `model="m"`, "-c", `model_reasoning_effort="high"`, "hi"},
		"opencode": {"opencode", "run", "--pure", "--format", "json", "--dir", "/d", "-m", "m", "--variant", "high", "hi"},
		"pi":       {"pi", "-p", "--mode", "json", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files", "--no-session", "--model", "m", "--thinking", "high", "hi"},
		"cursor":   {"cursor-agent", "-p", "--output-format", "stream-json", "--trust", "--force", "--workspace", "/d", "--model", "m", "hi"},
		"hermes":   {"hermes", "--ignore-rules", "acp"},
	} {
		for _, web := range []bool{false, true} {
			a := Agent{Harness: harness, Model: "m", Effort: "high", Web: web}
			if harness == "cursor" {
				a.Effort = ""
			}
			if harness == "hermes" {
				a.Model, a.Effort = "", ""
			}
			want := base
			if web && harness == "codex" {
				want = append([]string{"codex", "--search"}, base[1:]...)
			}
			wantStdin := ""
			if harness == "hermes" {
				wantStdin = "hi"
			}
			args, stdin := a.argv("/d", "/f", "hi")
			if !slices.Equal(args, want) || stdin != wantStdin {
				t.Errorf("%s web=%v: argv %q, stdin %q; want %q, %q", harness, web, args, stdin, want, wantStdin)
			}
		}
	}
}

func TestArgvPassesModelAndEffortUntouched(t *testing.T) {
	for harness, want := range map[string]string{
		"claude":   "--model x/My-Model:1 --effort max",
		"codex":    `-c model="x/My-Model:1" -c model_reasoning_effort="max"`,
		"opencode": "-m x/My-Model:1 --variant max",
		"pi":       "--model x/My-Model:1 --thinking max",
	} {
		argv, _ := Agent{Harness: harness, Model: "x/My-Model:1", Effort: "max"}.argv("/d", "/f", "hi")
		if got := strings.Join(argv, " "); !strings.Contains(got, want) {
			t.Errorf("%s: got %q, want %q in it", harness, got, want)
		}
	}
}

func TestCheckRejectsWhatTheHarnessCannotTake(t *testing.T) {
	for _, a := range []Agent{{Harness: "nope"}, {Harness: "hermes", Model: "m"}, {Harness: "cursor", Effort: "e"}} {
		if a.check() == nil {
			t.Errorf("%+v: want an error", a)
		}
	}
}

func TestStreamReadsEachHarnessFinalMessage(t *testing.T) {
	for harness, line := range map[string]string{
		"claude":   `{"type":"result","result":"OK"}`,
		"opencode": `{"type":"text","part":{"text":"OK"}}`,
		"pi":       `{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"OK"}]}}`,
		"cursor":   `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"OK"}]}}`,
		"hermes": `{"method":"session/update","params":{"update":{"sessionUpdate":"agent_message_chunk",` +
			`"content":{"type":"text","text":"OK"}}}}`,
	} {
		var s stream
		s.see(harness, []byte(line))
		if s.final != "OK" {
			t.Errorf("%s: final %q, want OK", harness, s.final)
		}
	}
	var s stream
	s.see("cursor", []byte(`{"type":"result","is_error":true,"result":"rate limited"}`))
	if s.failure != "rate limited" {
		t.Errorf("cursor error result: failure %q", s.failure)
	}
}

func TestHuman(t *testing.T) {
	for d, want := range map[time.Duration]string{
		5 * time.Second: "0 min 05 s", 268 * time.Second: "4 min 28 s", 62 * time.Minute: "1 h 02 min",
	} {
		if got := Human(d); got != want {
			t.Errorf("Human(%v) = %q, want %q", d, got, want)
		}
	}
}

package run

import (
	"strings"
	"testing"
	"time"
)

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

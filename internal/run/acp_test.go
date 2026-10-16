package run

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestACPConversationEndsAtThePromptsReply(t *testing.T) {
	s := stream{dir: "/clone", prompt: "hi"}
	if got := string(s.answer([]byte(`{"id":1,"result":{}}`))); !strings.Contains(got, `"method":"session/new"`) {
		t.Fatalf("after initialize: %s", got)
	}
	got := string(s.answer([]byte(`{"id":2,"result":{"sessionId":"s1"}}`)))
	if !strings.Contains(got, `"method":"session/prompt"`) || !strings.Contains(got, `"text":"hi"`) {
		t.Fatalf("after session/new: %s", got)
	}
	if s.answer([]byte(`{"id":3,"result":{"stopReason":"end_turn"}}`)) != nil || s.stop != "end_turn" {
		t.Fatalf("after the prompt: stop %q", s.stop)
	}
	if cause, err := s.acpEnded("signal killed", "signal killed", nil); cause != "" || err != nil {
		t.Fatalf("end_turn: cause %q, %v", cause, err)
	}
}

func TestACPPermitsOnlyInsideTheDirectory(t *testing.T) {
	s := stream{dir: "/clone"}
	for call, want := range map[string]string{
		`{"locations":[{"path":"/clone/a.go"}]}`:                    "yes",
		`{"rawInput":{"command":"go test ./..."}}`:                  "yes",
		`{"locations":[{"path":"/etc/passwd"}]}`:                    "no",
		`{"rawInput":{"command":"cat /clone/../secret"}}`:           "no",
		`{"rawInput":{"command":"curl localhost"}}`:                 "no",
		`{"content":[{"content":{"type":"text","text":"$ ls /"}}]}`: "no",
	} {
		params := `{"toolCall":` + call + `,"options":[{"optionId":"yes","kind":"allow_once"},` +
			`{"optionId":"no","kind":"reject_once"}]}`
		b, _ := json.Marshal(s.permit(json.RawMessage(params)))
		if !strings.Contains(string(b), `"optionId":"`+want+`"`) {
			t.Errorf("%s: got %s, want %s", call, b, want)
		}
	}
}

package screen

import (
	"encoding/json"
	"strings"
)

// activities is the agent lines as prose, thinking and tool calls, in order.
func activities(lines []string) []*activity {
	var out []*activity
	byID := map[string]*activity{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !json.Valid([]byte(line)) {
			out = append(out, &activity{says: line})
			continue
		}
		var e lineEvent
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		out = appendLine(out, byID, e)
	}
	return out
}

// lineEvent is every field of an agent line the screen reads. A field of another type is left empty.
type lineEvent struct {
	Type, Method         string
	Text, Thinking, Says string
	Name                 string
	ID, Tool, Detail     string
	Result, Summary      string
	Input                json.RawMessage
	Message              struct{ Content []content }
	Part                 part
	Item                 item
	Params               struct{ Update acpUpdate }
}

type content struct {
	Type, Text, Name, ID, Thinking string
	Input                          json.RawMessage
	ToolUseID                      string `json:"tool_use_id"`
	Content                        json.RawMessage
	IsError                        bool `json:"is_error"`
	Arguments                      json.RawMessage
}

type part struct {
	Type, Text, Tool, Name string
	State                  struct {
		Status string
		Input  json.RawMessage
	}
}

type item struct {
	ID, Type, Text, Command, Status string
	ExitCode                        *int   `json:"exit_code"`
	AggregatedOutput                string `json:"aggregated_output"`
}

type acpUpdate struct {
	SessionUpdate string
	Content       json.RawMessage
	ToolCallID    string
	Title         string
	Status        string
	RawInput      json.RawMessage
}

func appendLine(out []*activity, byID map[string]*activity, e lineEvent) []*activity {
	if e.Thinking != "" || e.Says != "" || e.Tool != "" || e.ID != "" {
		return appendDirect(out, byID, e)
	}
	switch {
	case e.Type == "result":
		if e.Result != "" {
			out = append(out, &activity{says: e.Result})
		}
	case e.Method == "session/update":
		return appendACP(out, byID, e.Params.Update)
	case e.Type == "assistant" || e.Type == "message_end" || e.Type == "message":
		return appendContent(out, byID, e.Message.Content)
	case e.Type == "user":
		return appendContent(out, byID, e.Message.Content)
	case e.Type == "text" || e.Type == "thinking":
		if text := cmpOr(e.Part.Text, e.Text); text != "" {
			if e.Type == "thinking" {
				out = append(out, &activity{thinking: text})
			} else {
				out = append(out, &activity{says: text})
			}
		}
	case e.Part.Tool != "" || e.Part.Name != "":
		return appendTool(out, byID, "", cmpOr(e.Part.Tool, e.Part.Name), "", detailOf(e.Part.State.Input), e.Part.State.Status, "")
	case e.Type == "tool_use" || e.Type == "toolCall":
		out = appendTool(out, byID, e.ID, cmpOr(e.Name, e.Tool), "", cmpOr(e.Detail, detailOf(e.Input)), "running", "")
	case e.Type == "item.completed" || e.Type == "item.started" || e.Type == "item.updated":
		return appendItem(out, byID, e.Item, e.Type != "item.completed")
	}
	return out
}

func appendDirect(out []*activity, byID map[string]*activity, e lineEvent) []*activity {
	if e.Thinking != "" || e.Says != "" {
		out = append(out, &activity{thinking: e.Thinking, says: e.Says})
	}
	if e.Tool != "" || e.ID != "" {
		out = appendTool(out, byID, e.ID, e.Tool, "", e.Detail, e.Result, e.Summary)
	}
	return out
}

func appendContent(out []*activity, byID map[string]*activity, blocks []content) []*activity {
	for _, c := range blocks {
		switch c.Type {
		case "thinking":
			if t := cmpOr(c.Thinking, c.Text); t != "" {
				out = append(out, &activity{thinking: t})
			}
		case "text":
			if c.Text != "" {
				out = append(out, &activity{says: c.Text})
			}
		case "tool_use", "toolCall":
			out = appendTool(out, byID, c.ID, cmpOr(c.Name, ""), "", detailOf(cmpRaw(c.Input, c.Arguments)), "running", "")
		case "tool_result":
			state := "passed"
			if c.IsError {
				state = "failed"
			}
			out = appendTool(out, byID, c.ToolUseID, "", "", "", state, firstText(c.Content))
		}
	}
	return out
}

func appendItem(out []*activity, byID map[string]*activity, it item, started bool) []*activity {
	switch it.Type {
	case "agent_message", "message":
		if it.Text != "" {
			out = append(out, &activity{says: it.Text})
		}
	case "reasoning":
		if it.Text != "" {
			out = append(out, &activity{thinking: it.Text})
		}
	case "command_execution", "tool_call", "function_call":
		state := "running"
		summary := ""
		if !started {
			state = "passed"
			if it.ExitCode != nil && *it.ExitCode != 0 {
				state = "failed"
			}
			summary = it.AggregatedOutput
		}
		name := it.Type
		if it.Command != "" {
			name = "bash"
		}
		out = appendTool(out, byID, it.ID, name, "", it.Command, state, summary)
	}
	return out
}

func appendACP(out []*activity, byID map[string]*activity, u acpUpdate) []*activity {
	var text struct{ Text string }
	_ = json.Unmarshal(u.Content, &text)
	switch u.SessionUpdate {
	case "agent_message_chunk":
		if text.Text != "" {
			out = append(out, &activity{says: text.Text})
		}
	case "agent_thought_chunk":
		if text.Text != "" {
			out = append(out, &activity{thinking: text.Text})
		}
	case "tool_call", "tool_call_update":
		state := "running"
		switch u.Status {
		case "completed", "done":
			state = "passed"
		case "failed", "error":
			state = "failed"
		}
		out = appendTool(out, byID, u.ToolCallID, u.Title, "", detailOf(u.RawInput), state, "")
	}
	return out
}

func appendTool(out []*activity, byID map[string]*activity, id, tool, _, detail, result, summary string) []*activity {
	var a *activity
	if id != "" {
		a = byID[id]
	}
	if a == nil {
		a = &activity{id: id}
		out = append(out, a)
		if id != "" {
			byID[id] = a
		}
	}
	if tool != "" {
		a.tool = tool
	}
	if detail != "" {
		a.detail = detail
	}
	if result != "" {
		a.result = result
	}
	if summary != "" {
		a.summary = summary
	}
	return out
}

func detailOf(raw json.RawMessage) string {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return oneLine(string(raw))
	}
	for _, k := range []string{"command", "cmd", "file_path", "path", "pattern", "query", "url"} {
		if s, ok := m[k].(string); ok && s != "" {
			return s
		}
	}
	for _, v := range m {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func firstText(raw json.RawMessage) string {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []struct{ Type, Text string }
	if json.Unmarshal(raw, &blocks) == nil {
		var texts []string
		for _, b := range blocks {
			if b.Text != "" {
				texts = append(texts, b.Text)
			}
		}
		return strings.Join(texts, "\n")
	}
	return ""
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func cmpRaw(a, b json.RawMessage) json.RawMessage {
	if len(strings.TrimSpace(string(a))) > 0 {
		return a
	}
	return b
}

// readableLines is what a person reads in agent lines: prose and tool names, not raw events.
func readableLines(lines []string) []string {
	var out []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !json.Valid([]byte(line)) {
			out = append(out, line)
			continue
		}
		for _, a := range activities([]string{line}) {
			switch {
			case a.says != "":
				out = append(out, strings.Split(a.says, "\n")...)
			case a.thinking != "":
				out = append(out, strings.Split(a.thinking, "\n")...)
			case a.tool != "":
				out = append(out, "→ "+a.tool)
			}
		}
	}
	return out
}

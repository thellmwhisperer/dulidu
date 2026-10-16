package run

import (
	"cmp"
	"encoding/json"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// acpUpdate is an ACP session/update's update, and the tool call a permission request names.
type acpUpdate struct {
	SessionUpdate string
	Content       json.RawMessage // a chunk's block, a tool call's list
	Locations     []struct{ Path string }
	RawInput      json.RawMessage
}

// acp reads one line of an ACP agent: its final message is the message chunks after the last tool call.
func (s *stream) acp(line []byte) {
	var m struct {
		Method string
		Params struct{ Update acpUpdate }
	}
	if json.Unmarshal(line, &m) != nil || m.Method != "session/update" {
		return
	}
	switch m.Params.Update.SessionUpdate {
	case "agent_message_chunk":
		var text struct{ Text string }
		_ = json.Unmarshal(m.Params.Update.Content, &text)
		s.final += text.Text
	case "tool_call":
		s.final = ""
	}
}

// answer is what dulidu writes back to one line of an ACP agent, nil for nothing: the conversation's next call
// once the agent replies to one, the answer to a permission request, method not found to any other request. A
// failed call or the prompt's reply ends the conversation, stop its stopReason or the call and its error.
func (s *stream) answer(line []byte) []byte {
	var m struct {
		ID     json.RawMessage
		Method string
		Params json.RawMessage
		Result struct{ SessionID, StopReason string }
		Error  *struct{ Message string }
	}
	if json.Unmarshal(line, &m) != nil || m.ID == nil {
		return nil
	}
	switch id := string(m.ID); {
	case m.Method == "session/request_permission":
		return jsonLine(map[string]any{"id": m.ID, "result": s.permit(m.Params)})
	case m.Method != "":
		return jsonLine(map[string]any{"id": m.ID, "error": map[string]any{"code": -32601, "message": "method not found"}})
	case m.Error != nil:
		s.stop = map[string]string{"1": "initialize", "2": "session/new", "3": "session/prompt"}[id] + ": " +
			m.Error.Message
	case id == "1":
		return jsonLine(map[string]any{"id": 2, "method": "session/new",
			"params": map[string]any{"cwd": s.dir, "mcpServers": []any{}}})
	case id == "2":
		return jsonLine(map[string]any{"id": 3, "method": "session/prompt", "params": map[string]any{
			"sessionId": m.Result.SessionID, "prompt": []any{map[string]any{"type": "text", "text": s.prompt}}}})
	case id == "3":
		s.stop = cmp.Or(m.Result.StopReason, "no stopReason")
	}
	return nil
}

// talk makes cmd's stdin dulidu's ACP conversation when harness is an ACP agent, prompt its prompt, and opens
// it with initialize: protocol version 1, and neither fs nor terminal for the agent.
func (s *stream) talk(cmd *exec.Cmd, harness, prompt string) (err error) {
	if _, acp := acpArgv[harness]; !acp {
		return nil
	}
	cmd.Stdin, s.prompt = nil, prompt
	if s.rpc, err = cmd.StdinPipe(); err == nil { // the pipe holds it until the agent starts
		s.say(cmd, jsonLine(map[string]any{"id": 1, "method": "initialize",
			"params": map[string]any{"protocolVersion": 1, "clientCapabilities": map[string]any{}}}))
	}
	return err
}

// say writes msg, if any, to the ACP agent cmd runs; once the conversation ends, dulidu stops the agent.
func (s *stream) say(cmd *exec.Cmd, msg []byte) {
	if msg != nil {
		_, _ = s.rpc.Write(msg) // a dead agent's failure is its own
	}
	if s.stop != "" {
		_ = s.rpc.Close()
		s.rpc = nil
		_ = cmd.Cancel()
	}
}

// acpEnded is the cause an ACP agent fails with, from how its process ended: at its prompt's reply, which
// dulidu stopped it at, end_turn passes and any other stopReason or failed call fails; without one, even exit
// code 0 fails.
func (s *stream) acpEnded(exit, cause string, err error) (string, error) {
	switch {
	case s.stop != "":
		return map[bool]string{false: s.stop}[s.stop == "end_turn"], nil
	case s.prompt != "" && cause == "" && exit != "":
		cause = exit + "; the agent never replied to its prompt"
	}
	return cause, err
}

func jsonLine(m map[string]any) []byte {
	m["jsonrpc"] = "2.0"
	b, _ := json.Marshal(m)
	return append(b, '\n')
}

// permit answers a permission request: its first allow_once option when the tool call is inside the directory,
// else its first reject_once, never an always one.
func (s *stream) permit(params json.RawMessage) map[string]any {
	var p struct {
		ToolCall acpUpdate
		Options  []struct{ OptionID, Kind string }
	}
	_ = json.Unmarshal(params, &p)
	kind := map[bool]string{true: "allow_once", false: "reject_once"}[p.ToolCall.inside(s.dir)]
	for _, o := range p.Options {
		if o.Kind == kind {
			return map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": o.OptionID}}
		}
	}
	return map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}
}

// inside says whether every path u names, each whole, a relative one dir's, is inside dir, and none of its
// commands reaches outside it.
func (u acpUpdate) inside(dir string) bool {
	for _, p := range u.paths() {
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return false
		}
	}
	for _, c := range u.commands() {
		if reaches(c, dir) {
			return false
		}
	}
	return true
}

// paths is u's locations and every value of a rawInput key naming a path, as path, file_path or cwd.
func (u acpUpdate) paths() (ps []string) {
	for _, l := range u.Locations {
		ps = append(ps, l.Path)
	}
	var in map[string]any
	_ = json.Unmarshal(u.RawInput, &in)
	for k, v := range in {
		if !strings.Contains(strings.ToLower(k), "path") && k != "cwd" {
			continue
		}
		vs, _ := v.([]any)
		for _, v := range append(vs, v) {
			if p, ok := v.(string); ok {
				ps = append(ps, p)
			}
		}
	}
	return ps
}

// commands is u's rawInput command and each command in its content, which hermes shows as "$ <command>".
func (u acpUpdate) commands() (cs []string) {
	var in struct{ Command string }
	_ = json.Unmarshal(u.RawInput, &in)
	var content []struct{ Content struct{ Type, Text string } }
	_ = json.Unmarshal(u.Content, &content)
	for _, c := range content {
		if command, ok := strings.CutPrefix(c.Content.Text, "$ "); ok && c.Content.Type == "text" {
			cs = append(cs, command)
		}
	}
	if in.Command != "" {
		cs = append(cs, in.Command)
	}
	return cs
}

var (
	link    = regexp.MustCompile("https?://[^\\s\"'`;|&)]+")
	sep     = " \t\r\n\"'`;|&()<>,="
	absPath = regexp.MustCompile(`"/[^"]*"|'/[^']*'|/[^\s"'` + "`" + `;|&()<>,=]*`)
)

// reaches says whether command names an absolute path not under dir, runs curl or wget, or names a URL off this
// machine. ponytail: curl or wget only as its first word; ATM's shell parser is the upgrade.
func reaches(command, dir string) bool {
	if fields := strings.Fields(command); badURL(command) ||
		len(fields) > 0 && slices.Contains([]string{"curl", "wget"}, strings.Trim(fields[0], "\"'`;|&()")) {
		return true
	}
	dir = filepath.ToSlash(dir)
	for _, m := range absPath.FindAllStringIndex(command, -1) {
		if m[0] > 0 && !strings.ContainsRune(sep, rune(command[m[0]-1])) ||
			m[1] < len(command) && !strings.ContainsRune(sep+"/", rune(command[m[1]])) {
			continue
		}
		if p := path.Clean(strings.Trim(command[m[0]:m[1]], `"'`)); p != dir && !strings.HasPrefix(p, dir+"/") {
			return true
		}
	}
	return false
}

// badURL says whether word names a URL off this machine.
func badURL(word string) bool {
	for _, l := range link.FindAllString(word, -1) {
		authority := l[strings.Index(l, "://")+3:]
		if end := strings.IndexAny(authority, "/?#"); end >= 0 {
			authority = authority[:end]
		}
		if user := strings.LastIndexByte(authority, '@'); user >= 0 {
			authority = authority[user+1:]
		}
		host := authority
		if strings.HasPrefix(host, "[") {
			end := strings.IndexByte(host, ']')
			if end < 0 || host[end+1:] != "" && !strings.HasPrefix(host[end+1:], ":") {
				return true
			}
			host = host[1:end]
		} else if end := strings.IndexByte(host, ':'); end >= 0 {
			host = host[:end]
		}
		if !slices.Contains([]string{"localhost", "127.0.0.1", "::1"}, strings.ToLower(host)) {
			return true
		}
	}
	return false
}

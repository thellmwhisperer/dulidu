// Package run runs the user's agent CLI and reads its final message.
package run

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Agent is the agent CLI dulidu runs: its harness, its model and effort, passed through untouched, and how long
// it may take, 0 for agentTimeout. Log, when set, gets each line it writes as it writes it.
type Agent struct {
	Harness, Model, Effort string
	Timeout                time.Duration
	Log                    io.Writer
}

// acpArgv is the agents dulidu speaks ACP with, by harness: the command line it runs in the directory.
var acpArgv = map[string][]string{"hermes": {"hermes", "--ignore-rules", "acp"}}

// tools is all claude needs.
const tools = "Read,Edit,Write,Bash,Glob,Grep,Skill"

// agentTimeout bounds the agent when dulidu.yaml sets no agent_timeout.
var agentTimeout = 30 * time.Minute

// errSilent cancels an agent that said nothing for agentSilence.
var errSilent = errors.New("silent")

// agentSilence is how long the agent may write nothing, on stdout or stderr, before dulidu stops it.
var agentSilence = 10 * time.Minute

// check says whether dulidu can run a.
func (a Agent) check() error {
	switch {
	case !slices.Contains([]string{"claude", "codex", "opencode", "pi", "cursor", "hermes"}, a.Harness):
		return fmt.Errorf("harness %q unknown, want claude, codex, opencode, pi, cursor or hermes", a.Harness)
	case a.Harness == "hermes" && (a.Model != "" || a.Effort != ""):
		return errors.New("hermes takes model and effort from its own config; unset them in dulidu.yaml")
	case a.Harness == "cursor" && a.Effort != "":
		return fmt.Errorf("cursor has no effort flag: put it in the model, as '<model>[effort=%s]'", a.Effort)
	}
	return nil
}

// argv is the agent's command line, in its most minimal mode, and what goes on its stdin: claude reads the
// prompt there, the others take it as the last argument, but an ACP agent, whose prompt it is. final is where
// codex writes its last message.
func (a Agent) argv(dir, final, prompt string) (argv []string, stdin string) {
	if acp, ok := acpArgv[a.Harness]; ok {
		return slices.Clone(acp), prompt
	}
	with := func(argv []string, value string, args ...string) []string {
		if value == "" {
			return argv
		}
		return append(argv, args...)
	}
	switch a.Harness {
	case "claude":
		argv = []string{"claude", "-p", "--output-format", "stream-json", "--verbose", "--setting-sources", "project",
			"--strict-mcp-config", "--permission-mode", "acceptEdits", "--tools", tools, "--allowedTools", tools}
		return with(with(argv, a.Model, "--model", a.Model), a.Effort, "--effort", a.Effort), prompt
	case "codex":
		argv = []string{"codex", "exec", "--json", "-C", dir, "--sandbox", "workspace-write", "-o", final}
		argv = with(argv, a.Model, "-c", "model="+strconv.Quote(a.Model))
		argv = with(argv, a.Effort, "-c", "model_reasoning_effort="+strconv.Quote(a.Effort))
	case "opencode":
		argv = []string{"opencode", "run", "--pure", "--format", "json", "--dir", dir}
		argv = with(with(argv, a.Model, "-m", a.Model), a.Effort, "--variant", a.Effort)
	case "cursor":
		argv = []string{"cursor-agent", "-p", "--output-format", "stream-json", "--trust", "--force", "--workspace", dir}
		argv = with(argv, a.Model, "--model", a.Model)
	case "pi":
		argv = []string{"pi", "-p", "--mode", "json", "--no-extensions", "--no-skills", "--no-prompt-templates",
			"--no-context-files", "--no-session"}
		argv = with(with(argv, a.Model, "--model", a.Model), a.Effort, "--thinking", a.Effort)
	}
	return append(argv, prompt), ""
}

// Run runs the agent on prompt in dir, with pipes and no terminal, in its own process group, and returns its
// final message, which check accepts. The timeout, agentSilence without a line, SIGINT and SIGTERM kill the
// group. Its error is the cause, then the agent's last 5 lines.
func (a Agent) Run(dir, prompt string, check func(final string) error) (string, error) {
	if err := a.check(); err != nil {
		return "", err
	}
	f, err := os.CreateTemp("", "dulidu-final-*.md")
	if err != nil {
		return "", err
	}
	final := f.Name()
	_ = f.Close()
	defer func() { _ = os.Remove(final) }()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	limit := cmp.Or(a.Timeout, agentTimeout)
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	ctx, hush := context.WithCancelCause(ctx)
	defer hush(nil)
	quiet := time.AfterFunc(agentSilence, func() { hush(errSilent) })
	defer quiet.Stop()
	argv, stdin := a.argv(dir, final, prompt)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir, cmd.Stdin, cmd.WaitDelay = dir, strings.NewReader(stdin), time.Second
	cmd.Env = slices.DeleteFunc(os.Environ(), func(kv string) bool {
		return strings.HasPrefix(kv, "CLAUDE_CODE_CHILD_SESSION=") // set, the agent's transcripts are off
	})
	if a.Harness == "claude" { // else a background command outlives its turn
		cmd.Env = append(cmd.Env, "CLAUDE_CODE_DISABLE_BACKGROUND_TASKS=1")
	}
	s := &stream{dir: dir, log: a.Log, heard: func() { quiet.Reset(agentSilence) }}
	if err := s.talk(cmd, a.Harness, stdin); err != nil {
		return "", err
	}
	err = s.read(cmd, a.Harness)
	quiet.Stop()
	if a.Harness == "codex" {
		b, _ := os.ReadFile(final) // none is no final message
		s.final = string(b)
	}
	exit, cause := ended(ctx, cmd.ProcessState, limit)
	if cause, err = s.acpEnded(exit, cause, err); cause != "" {
		err = errors.New(cause)
	}
	if err == nil && s.failure != "" {
		err = errors.New(s.failure)
	}
	if err == nil {
		err = check(s.final)
	}
	if err != nil && len(s.tail) > 0 {
		err = fmt.Errorf("%w: %s", err, strings.Join(s.tail, " | "))
	}
	return s.final, err
}

// ended is how the agent run under ctx, bounded by limit, ended, its exit, "" when it never ran, and the cause
// it fails with, "" when it exited 0.
func ended(ctx context.Context, p *os.ProcessState, limit time.Duration) (exit, cause string) {
	switch {
	case errors.Is(context.Cause(ctx), errSilent):
		return "silent for " + Human(agentSilence), "the agent said nothing for " + Human(agentSilence) +
			"; dulidu stopped it"
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		exit = "timed out after " + Human(limit)
	case ctx.Err() != nil:
		exit = "interrupted"
	case p == nil:
		return "", ""
	case p.Success():
		return "exit code 0", ""
	case p.ExitCode() >= 0:
		exit = "exit code " + strconv.Itoa(p.ExitCode())
	default:
		exit = "signal " + strings.TrimPrefix(p.String(), "signal: ")
	}
	return exit, exit
}

// Human is d as dulidu prints every time: 0 min 05 s, 4 min 28 s, 1 h 02 min.
func Human(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%d min %02d s", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%d h %02d min", int(d.Hours()), int(d.Minutes())%60)
}

// stream is what the agent's lines said: its final message, cursor's error result, even when the process exits
// 0, and its last 5 lines.
type stream struct {
	final, failure string
	tail           []string
	heard          func()    // each line calls it
	log            io.Writer // each line goes to it, when set
	// An ACP agent's: its directory, its prompt, its stdin until its prompt's reply, and the reply's stopReason
	// or the failed call.
	dir, prompt, stop string
	rpc               io.WriteCloser
}

// read runs cmd, reading the stream of harness from each line of its stdout and stderr.
func (s *stream) read(cmd *exec.Cmd, harness string) error {
	ownGroup(cmd)
	var mu sync.Mutex
	emit := func(line string, stdout bool) {
		line = strings.TrimSpace(line)
		mu.Lock()
		defer mu.Unlock()
		s.heard()
		if s.log != nil {
			_, _ = io.WriteString(s.log, line+"\n") // the live view's copy: losing it loses no output
		}
		s.tail = append(s.tail, line)
		s.tail = s.tail[max(0, len(s.tail)-5):]
		b := []byte(line)
		if json.Valid(b) {
			s.see(harness, b)
		}
		if stdout && s.rpc != nil {
			s.say(cmd, s.answer(b))
		}
	}
	out, errout := &lineWriter{fn: func(l string) { emit(l, true) }}, &lineWriter{fn: func(l string) { emit(l, false) }}
	cmd.Stdout, cmd.Stderr = out, errout
	err := cmd.Run()
	// Exited but failed, or left something holding its output: kill what it left, whatever its exit.
	if err != nil && cmd.ProcessState != nil {
		_ = cmd.Cancel() // its group may be empty already, which is no failure
	}
	if errors.Is(err, exec.ErrWaitDelay) { // exited 0, and only left something holding its output
		err = nil
	}
	out.flush()
	errout.flush()
	return err
}

// event is the fields of every harness's events that say what its final message is.
type event struct {
	Type    string
	Result  json.RawMessage // claude's and cursor's
	IsError bool            `json:"is_error"` // cursor's
	Message struct {
		Role    string
		Content []struct{ Type, Text string }
	}
	Part struct{ Text string } // opencode's
}

// see reads one JSON line of harness's stream.
func (s *stream) see(harness string, line []byte) {
	if _, acp := acpArgv[harness]; acp {
		s.acp(line)
		return
	}
	var e event
	_ = json.Unmarshal(line, &e) // a field of another type is left empty, the rest is read
	if harness == "cursor" && e.Type == "result" && e.IsError {
		var text string
		_ = json.Unmarshal(e.Result, &text)
		s.failure = cmp.Or(s.failure, text, "error result")
	}
	if final, ends := e.final(harness); ends {
		s.final = final
	}
}

// final is the final message e ends, if it ends one; codex's is in its -o file instead.
func (e event) final(harness string) (string, bool) {
	switch {
	case harness == "claude" && e.Type == "result":
		var text string
		_ = json.Unmarshal(e.Result, &text)
		return text, true
	case harness == "opencode" && e.Type == "text":
		return e.Part.Text, true
	case (harness == "pi" && e.Type == "message_end" || harness == "cursor" && e.Type == "assistant") &&
		e.Message.Role == "assistant":
		var texts []string
		for _, b := range e.Message.Content {
			if b.Type == "text" {
				texts = append(texts, b.Text)
			}
		}
		return strings.Join(texts, "\n"), true
	}
	return "", false
}

// lineWriter calls fn with each line written to it, without its newline.
type lineWriter struct {
	fn  func(string)
	buf []byte
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			return len(p), nil
		}
		line := string(bytes.TrimSuffix(w.buf[:i], []byte("\r")))
		w.buf = w.buf[i+1:]
		w.fn(line)
	}
}

func (w *lineWriter) flush() {
	if len(w.buf) > 0 {
		w.fn(string(w.buf))
		w.buf = nil
	}
}

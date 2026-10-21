//go:build unix

package pipeline_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Exercise the foreground view and its independent run through the actual
// executable. FIFO handshakes hold the fake agent without sleeps or timing races.
func TestBinaryRunOnATerminalShowsAgentProse(t *testing.T) {
	bin := binary(t)
	for _, exit := range []string{"q", "ctrl+c", "finished"} {
		t.Run(exit, func(t *testing.T) {
			repo := realTempDir(t)
			if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("git init: %v\n%s", err, out)
			}
			if err := os.WriteFile(filepath.Join(repo, "brief.md"), []byte(binaryRead(t, filepath.Join(binaryExample, "brief.md"))), 0o644); err != nil {
				t.Fatal(err)
			}
			answers, err := filepath.Glob(filepath.Join(binaryExample, "answers", "*"))
			if err != nil || len(answers) != 18 {
				t.Fatalf("recorded answers: %v, %v", answers, err)
			}
			replay := fakeReplay(t, answers[1:])
			fake := t.TempDir()
			if err := os.WriteFile(filepath.Join(fake, "claude"), []byte(terminalClaude), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("HOME", t.TempDir())
			init := exec.Command(bin, "init")
			init.Dir = repo
			if out, err := init.CombinedOutput(); err != nil {
				t.Fatalf("dulidu init: %v\n%s", err, out)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "python3", "-c", runTerminalCheck, bin, replay, exit)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("run in a terminal: %v\n%s", err, out)
			}
		})
	}
}

const terminalClaude = `#!/bin/sh
cat > /dev/null
echo >> "$FAKE_REPLAY/calls"
n=$(wc -l < "$FAKE_REPLAY/calls")
if [ "$((n))" = 1 ]; then
  i=0
  while [ "$i" -lt 60 ]; do
    printf '{"type":"assistant","message":{"content":[{"type":"text","text":"History %02d"}]}}\n' "$i"
    i=$((i+1))
  done
  python3 - <<'EVENTS'
import json
print(json.dumps({"type": "result", "result": "Long event survived.\n" + json.dumps({"private": "x" * 70000})}))
EVENTS
  printf '%s\n' '{"type":"message_end","message":{"content":[{"type":"text","text":"Pi is reading.\n{\"private\":true}"},{"type":"toolCall","id":"pi-read","name":"read","arguments":{"path":"pi.md"}}]}}'
  printf '%s\n' '{"type":"text","part":{"text":"OpenCode is reading.\n{\"private\":true}"}}'
  printf '%s\n' '{"type":"tool_use","part":{"tool":"glob","state":{"input":{"pattern":"*.md"},"status":"running"}}}'
  printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"Codex is reading.\n{\"private\":true}"}}'
  printf '%s\n' '{"type":"item.started","item":{"id":"codex-call","type":"command_execution","command":"pwd"}}'
  printf '%s\n' '{"type":"item.completed","item":{"id":"codex-call","type":"command_execution","command":"pwd","exit_code":0,"aggregated_output":"project"}}'
  printf '%s\n' '{"method":"session/update","params":{"update":{"sessionUpdate":"agent_message_chunk","content":{"text":"Hermes is reading.\n{\"private\":true}"}}}}'
  printf '%s\n' '{"method":"session/update","params":{"update":{"sessionUpdate":"agent_thought_chunk","content":{"text":"Checking the brief.\n{\"private\":true}"}}}}'
  printf '%s\n' '{"method":"session/update","params":{"update":{"sessionUpdate":"tool_call","toolCallId":"hermes-call","title":"Inspect","rawInput":{"path":"hermes.md"}}}}'
  printf '%s\n' '{"method":"session/update","params":{"update":{"sessionUpdate":"tool_call_update","toolCallId":"hermes-call","status":"completed"}}}'
  printf '%s\n' '{"type":"result","result":"A readable final result."}'
  printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"text","text":"Reading the brief now.\n{\"private\":true}"},{"type":"tool_use","id":"claude-read","name":"Read","input":{"file_path":"brief.md"}}]}}'
  echo 'stderr must stay out of Agent' >&2
  read release < "$FAKE_REPLAY/gate"
  test -f "$FAKE_REPLAY/released" || exit 1
fi
exec cat "$FAKE_REPLAY/$((n))"
`

const runTerminalCheck = `
import fcntl, glob, json, os, pty, re, select, signal, struct, subprocess, sys, termios, time

os.setsid()
master, slave = pty.openpty()
fcntl.ioctl(slave, termios.TIOCSCTTY, 0)
signal.signal(signal.SIGINT, signal.SIG_IGN)
signal.signal(signal.SIGHUP, signal.SIG_IGN)
fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 30, 100, 0, 0))
def settings():
    return subprocess.check_output(['stty', '-g'], stdin=slave)
subprocess.check_call(['stty', 'icanon', 'echo', 'min', '0', 'time', '0'], stdin=slave)
before = settings()
def terminal_child():
    signal.signal(signal.SIGINT, signal.SIG_DFL)
    signal.signal(signal.SIGHUP, signal.SIG_DFL)
replay = sys.argv[2]
os.mkfifo(replay + '/gate')
# Keep both ends open so cleanup can always release the agent, even on failure.
gate = os.open(replay + '/gate', os.O_RDWR | os.O_NONBLOCK)
p = subprocess.Popen([sys.argv[1], 'run', 'brief.md'], stdin=slave, stdout=slave,
                     stderr=slave, preexec_fn=terminal_child)
pending = b''
last = ''
seen = []
def frame_until(description, predicate):
    global pending, last
    deadline = time.monotonic() + 2
    while time.monotonic() < deadline:
        ready, _, _ = select.select([master], [], [], max(0, deadline-time.monotonic()))
        if not ready:
            break
        pending += os.read(master, 65536)
        if b'\x1b[6n' in pending:
            os.write(master, b'\x1b]11;rgb:0000/0000/0000\x1b\\\x1b[1;1R')
            pending = pending.replace(b'\x1b[6n', b'')
        while b'\x1b[2J\x1b[H' in pending:
            _, frame = pending.split(b'\x1b[2J\x1b[H', 1)
            end = re.search(rb'detach.*?tools\x1b\[0m', frame)
            if end is None:
                break
            pending = frame[end.end():]
            last = re.sub(r'\x1b\[[0-?]*[ -/]*[@-~]', '', frame[:end.end()].decode())
            seen.append(last)
            assert '{"type"' not in last and '"private"' not in last, 'JSON in Agent: ' + last
            assert 'stderr must stay out' not in last, 'stderr in Agent: ' + last
            if predicate(last):
                return last
    raise AssertionError(description + '\nlast frame:\n' + last + '\npending: ' + repr(pending[-3000:]))
def release():
    with open(replay + '/released', 'w') as f:
        f.write('continue')
    os.write(gate, b'continue\n')
def exited():
    deadline = time.monotonic() + 1
    while p.poll() is None and time.monotonic() < deadline:
        ready, _, _ = select.select([master], [], [], 0.01)
        if ready:
            os.read(master, 65536)
    assert p.poll() == 0, 'view must exit 0 within 1 s'
    assert settings() == before, 'view must restore stty -g'
def report_done():
    deadline = time.monotonic() + 2
    while time.monotonic() < deadline:
        paths = glob.glob('.dulidu/runs/*/report.json')
        if paths:
            try:
                with open(paths[0]) as f:
                    if json.load(f)['status'] == 'done':
                        return
            except (ValueError, KeyError):
                pass
        ready, _, _ = select.select([master], [], [], 0.01)
        if ready:
            os.read(master, 65536)
    raise AssertionError('run did not reach done after releasing the agent')
try:
    frame_until('prose, tool and final result before release', lambda s:
        all(word in s for word in ['Pipeline', 'Input', 'Triage', 'Research', 'Analysis',
                                  'Epics', 'Export', 'Reading the brief now.',
                                  '→ Read', 'A readable final result.', 'Long event survived.',
                                  'Pi is reading.', 'OpenCode is reading.', 'Codex is reading.',
                                  'Hermes is reading.', 'Checking the brief.',
                                  '→ read', '→ glob', '→ bash', '→ Inspect']))
    os.write(master, b't')
    frame_until('t unfolds tool arguments and completed calls', lambda s:
        all(word in s for word in ['running read pi.md', 'running glob *.md',
                                  'ok bash pwd', 'ok Inspect hermes.md', 'running Read brief.md']))
    os.write(master, b't')
    frame_until('t folds tool details again', lambda s: '→ Read' in s and 'running Read brief.md' not in s)
    assert not os.path.exists(replay + '/released'), 'agent was not held'
    os.write(master, b'\t')
    frame_until('tab focuses Pipeline', lambda s: 'Pipeline*' in s)
    os.write(master, b'\x1b[5~')
    frame_until('page up stays attached', lambda s: 'Pipeline*' in s)
    os.write(master, b'\t\x1b[5~')
    frame_until('page up scrolls Agent', lambda s: 'Agent*' in s and 'History' in s and '→ Read' not in s)
    os.write(master, b'\x1b[F')
    frame_until('end follows the agent', lambda s: '→ Read' in s)
    os.write(master, b'\x1b[A')
    frame_until('up scrolls one line', lambda s: '→ Read' not in s and 'Reading the brief now.' in s)
    os.write(master, b'\x1b[B')
    frame_until('down scrolls one line', lambda s: '→ Read' in s)
    os.write(master, b'\x1b[<64;80;20M')
    frame_until('wheel up scrolls', lambda s: '→ Read' not in s and 'History' in s)
    os.write(master, b'\x1b[<65;80;20M')
    frame_until('wheel down scrolls', lambda s: '→ Read' in s)
    os.write(master, b'\x1b[5~\x1b[6~')
    frame_until('page down returns to end', lambda s: '→ Read' in s)
    os.write(master, b'\x1b[H')
    frame_until('home retains history before the large event', lambda s: 'History 00' in s)
    os.write(master, b'\x1b[F')
    frame_until('end returns to the latest tool', lambda s: '→ Read' in s)
    assert p.poll() is None, 'a navigation key left the view'
    if sys.argv[3] == 'finished':
        release()
        report_done()
        exited()
    else:
        os.write(master, b'q' if sys.argv[3] == 'q' else b'\x03')
        exited()
        with open(glob.glob('.dulidu/runs/*/report.json')[0]) as f:
            assert json.load(f)['status'] == 'running', 'leaving the view stopped the run'
        release()
        report_done()
finally:
    release()
    if p.poll() is None:
        p.kill()
        p.wait(timeout=1)
    # Wait for the independent run to finish before Go removes its fixture.
    report_done()
    os.close(gate)
    os.close(master)
    os.close(slave)
`

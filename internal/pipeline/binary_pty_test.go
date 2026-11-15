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
			cmd := exec.CommandContext(ctx, "python3", "-c", terminalScreen+runTerminalCheck, bin, replay, exit)
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
print(json.dumps({"type": "assistant", "message": {"content": [{"type": "text", "text": "Long event survived."}]},
                  "private": "x" * 70000}))
EVENTS
  printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"Checking the brief."}]},"private":true}'
  printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"tool_use","id":"glob","name":"Glob","input":{"pattern":"*.md"}}]}}'
  printf '%s\n' '{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"glob","content":"brief.md"}]}}'
  printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"tool_use","id":"pwd","name":"Bash","input":{"command":"pwd"}}]}}'
  printf '%s\n' '{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"pwd","content":[{"type":"text","text":"project"}]}]}}'
  printf '%s\n' '{"type":"result","result":"A readable final result."}'
  printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"text","text":"Reading the brief now."},{"type":"tool_use","id":"claude-read","name":"Read","input":{"file_path":"brief.md"}}]},"private":true}'
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
screen = Screen(100, 30)
last = ''
def frame_until(description, predicate):
    global last
    deadline = time.monotonic() + 2
    while time.monotonic() < deadline:
        ready, _, _ = select.select([master], [], [], max(0, deadline-time.monotonic()))
        if not ready:
            break
        screen.feed(os.read(master, 65536))
        while screen.asked:
            os.write(master, b'\x1b]11;rgb:0000/0000/0000\x1b\\\x1b[1;1R')
            screen.asked -= 1
        assert screen.clears <= 1, 'the view cleared the screen'
        frames, screen.frames = screen.frames, []
        for frame in frames:
            last = frame
            assert '{"type"' not in last and '"private"' not in last, 'JSON in Agent: ' + last
            assert 'stderr must stay out' not in last, 'stderr in Agent: ' + last
            if predicate(last):
                return last
    raise AssertionError(description + '\nlast frame:\n' + last)
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
    frame_until('prose, thinking and tools before release', lambda s:
        all(word in s for word in ['Pipeline', 'Input', 'Triage', 'Research', 'Analysis',
                                  'Epics', 'Export', 'Agent · claude', 'Reading the brief now.',
                                  '1 tool: Read · brief.md', 'Long event survived.', '∴ Checking the brief.',
                                  '2 tools: Glob, Bash']))
    os.write(master, b't')
    frame_until('t unfolds tool arguments and completed calls', lambda s:
        all(word in s for word in ['ok Glob *.md · brief.md', 'ok Bash pwd · project', 'running Read brief.md']))
    os.write(master, b't')
    frame_until('t folds tool details again', lambda s: '1 tool: Read' in s and 'running Read brief.md' not in s)
    assert not os.path.exists(replay + '/released'), 'agent was not held'
    os.write(master, b'\t')
    frame_until('tab focuses Pipeline', lambda s: 'Pipeline*' in s)
    os.write(master, b'\x1b[5~')
    frame_until('page up stays attached', lambda s: 'Pipeline*' in s)
    os.write(master, b'\t\x1b[5~')
    frame_until('page up scrolls Agent', lambda s: re.search(r'Agent[^─]*\*', s) and 'History' in s and '1 tool: Read' not in s)
    os.write(master, b'\x1b[F')
    frame_until('end follows the agent', lambda s: '1 tool: Read' in s)
    os.write(master, b'\x1b[A')
    frame_until('up scrolls one line', lambda s: '1 tool: Read' not in s and 'Reading the brief now.' in s)
    os.write(master, b'\x1b[B')
    frame_until('down scrolls one line', lambda s: '1 tool: Read' in s)
    os.write(master, b'\x1b[<64;80;20M')
    frame_until('wheel up scrolls', lambda s: '1 tool: Read' not in s and 'History' in s)
    os.write(master, b'\x1b[<65;80;20M')
    frame_until('wheel down scrolls', lambda s: '1 tool: Read' in s)
    os.write(master, b'\x1b[5~\x1b[6~')
    frame_until('page down returns to end', lambda s: '1 tool: Read' in s)
    os.write(master, b'\x1b[H')
    frame_until('home retains history before the large event', lambda s: 'History 00' in s)
    os.write(master, b'\x1b[F')
    frame_until('end returns to the latest tool', lambda s: '1 tool: Read' in s)
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

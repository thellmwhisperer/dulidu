//go:build unix

package pipeline_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBinaryAttachAnswersKeysInATerminal(t *testing.T) {
	bin, repo := binary(t), realTempDir(t)
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "brief.md"), []byte(binaryRead(t, filepath.Join(binaryExample, "brief.md"))), 0o644); err != nil {
		t.Fatal(err)
	}
	answers, err := filepath.Glob(filepath.Join(binaryExample, "answers", "*"))
	if err != nil || len(answers) != 15 {
		t.Fatalf("recorded answers: %v, %v", answers, err)
	}
	fakeReplay(t, answers[1:]) // The brief alone needs no image description.
	t.Setenv("HOME", t.TempDir())
	for _, args := range [][]string{{"init"}, {"run", "brief.md"}} {
		cmd := exec.Command(bin, args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("dulidu %v: %v\n%s", args, err, out)
		}
	}
	dirs, err := filepath.Glob(filepath.Join(repo, ".dulidu", "runs", "*"))
	if err != nil || len(dirs) != 1 {
		t.Fatalf("run folders: %v, %v", dirs, err)
	}
	// Give the finished run enough assistant history to prove scrolling.
	// Non-event lines belong only in failed-step findings, never in Agent.
	logPath := filepath.Join(dirs[0], "agent.out")
	history := `{"type":"assistant","message":{"content":[{"type":"text","text":"terminal scroll history"}]}}` + "\n"
	end := `{"type":"result","result":"terminal history end"}` + "\n"
	log := binaryRead(t, logPath) + strings.Repeat(history, 60) + end
	if err := os.WriteFile(logPath, []byte(log), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, minimum := range []string{"1", "0"} {
		for _, key := range []string{"q", "ctrl+c"} {
			t.Run("min="+minimum+"/"+key, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "python3", "-c", attachTerminalCheck, bin, filepath.Base(dirs[0]), key, minimum)
				cmd.Dir = repo
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("attach in a terminal: %v\n%s", err, out)
				}
			})
		}
	}
}

// Python's standard-library pty keeps this binary test runnable with only this
// test file added to the base revision: it needs no new Go module or source API.
const attachTerminalCheck = `
import fcntl, os, pty, re, select, signal, struct, subprocess, sys, termios, time

# Keep the controlling session alive after attach exits so stty can inspect it.
os.setsid()
master, slave = pty.openpty()
fcntl.ioctl(slave, termios.TIOCSCTTY, 0)
signal.signal(signal.SIGINT, signal.SIG_IGN)
signal.signal(signal.SIGHUP, signal.SIG_IGN)
fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 30, 100, 0, 0))
def settings():
    return subprocess.check_output(['stty', '-g'], stdin=slave)
# Canonical terminals may retain zero-byte reads from a previous application.
# Disabling canonical mode must also set the byte-read minimum.
subprocess.check_call(['stty', 'icanon', 'min', sys.argv[4], 'time', '0'], stdin=slave)
before = settings()
def terminal_child():
    signal.signal(signal.SIGINT, signal.SIG_DFL)
    signal.signal(signal.SIGHUP, signal.SIG_DFL)
p = subprocess.Popen([sys.argv[1], 'attach', sys.argv[2]],
                     stdin=slave, stdout=slave, stderr=slave,
                     preexec_fn=terminal_child)
pending = b''
last = ''
def frame_until(description, predicate):
    global pending, last
    deadline = time.monotonic() + 2
    while time.monotonic() < deadline:
        ready, _, _ = select.select([master], [], [], max(0, deadline-time.monotonic()))
        if not ready:
            break
        pending += os.read(master, 65536)
        # Reply as a terminal would to colour/cursor queries during startup.
        if b'\x1b[6n' in pending:
            os.write(master, b'\x1b]11;rgb:0000/0000/0000\x1b\\\x1b[1;1R')
            pending = pending.replace(b'\x1b[6n', b'')
        # A complete frame ends in the footer; don't mistake a partial write
        # or a timer repaint for a key response.
        while b'\x1b[2J\x1b[H' in pending:
            _, frame = pending.split(b'\x1b[2J\x1b[H', 1)
            end = re.search(rb'detach.*?tools\x1b\[0m', frame)
            if end is None:
                break
            pending = frame[end.end():]
            last = re.sub(r'\x1b\[[0-?]*[ -/]*[@-~]', '', frame[:end.end()].decode())
            if predicate(last):
                return last
    raise AssertionError(description + '\nlast frame:\n' + last + '\npending: ' + repr(pending[-5000:]))
try:
    initial = frame_until('pipeline with every step within 2 s', lambda s:
        all(word in s for word in ['╭', 'Pipeline', 'Input', 'Triage', 'Research',
                                  'Analysis', 'Epics', 'Export', 'terminal history end']))
    os.write(master, b'\t')
    frame_until('tab must focus Pipeline', lambda s: 'Pipeline*' in s)
    os.write(master, b'\t')
    frame_until('tab must return focus to Agent', lambda s: 'Agent*' in s and 'Pipeline*' not in s)
    os.write(master, b'\x1b[5~')
    frame_until('pgup must scroll history', lambda s:
        'terminal scroll history' in s and 'terminal history end' not in s and s != initial)
    os.write(master, b'q' if sys.argv[3] == 'q' else b'\x03')
    # Drain final redraws/terminal cleanup so a full PTY cannot block exit.
    deadline = time.monotonic() + 1
    while p.poll() is None and time.monotonic() < deadline:
        ready, _, _ = select.select([master], [], [], 0.01)
        if ready:
            os.read(master, 65536)
    assert p.poll() == 0, 'attach must exit 0 within 1 s'
    assert settings() == before, 'attach must restore stty -g'
finally:
    if p.poll() is None:
        p.kill()
        os.close(master)
        master = None
        p.wait(timeout=1)
    if master is not None:
        os.close(master)
    os.close(slave)
`

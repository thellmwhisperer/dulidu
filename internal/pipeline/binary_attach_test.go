//go:build unix

package pipeline_test

import (
	"context"
	"encoding/json"
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
	if err != nil || len(answers) != 18 {
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
	end := `{"type":"assistant","message":{"content":[{"type":"text","text":"terminal history end"}]}}` + "\n"
	log := binaryRead(t, logPath) + strings.Repeat(history, 60) + end
	if err := os.WriteFile(logPath, []byte(log), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, size := range [][2]string{{"100", "30"}, {"120", "40"}} {
		for _, minimum := range []string{"1", "0"} {
			for _, key := range []string{"q", "ctrl+c", "sigint"} {
				if size[0] == "120" && (minimum != "1" || key != "q") {
					continue // the real run's frame at 120×40, once
				}
				t.Run(size[0]+"x"+size[1]+"/min="+minimum+"/"+key, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					cmd := exec.CommandContext(ctx, "python3", "-c", terminalScreen+attachTerminalCheck, bin,
						filepath.Base(dirs[0]), key, minimum, size[0], size[1])
					cmd.Dir = repo
					out, err := cmd.CombinedOutput()
					if err != nil {
						t.Fatalf("attach in a terminal: %v\n%s", err, out)
					}
					t.Logf("dulidu attach %s at %sx%s:\n%s", filepath.Base(dirs[0]), size[0], size[1], out)
				})
			}
		}
	}
}

// terminalScreen is a terminal's screen in Python, for the scripts that run dulidu on a pseudo-terminal: what the
// view's writes leave on it, as bubbletea writes them, only the rows that changed since its last frame. Each frame
// ends with the cursor sent to its last row, and Screen.frames is the screen's text at each of them.
const terminalScreen = `
import codecs, re, unicodedata

class Screen:
    token = re.compile(r'\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[^\[\]]|[\r\n\b]|[^\x1b\r\n\b]+')

    def __init__(self, cols, rows):
        self.cols, self.rows = cols, rows
        self.cells = [[' '] * cols for _ in range(rows)]
        self.x = self.y = 0
        self.pending = ''
        self.decode = codecs.getincrementaldecoder('utf-8')('replace').decode
        self.frames, self.clears, self.asked = [], 0, 0

    def feed(self, data):
        self.pending += self.decode(data)
        while True:
            m = self.token.match(self.pending)
            if m is None:
                return
            tok, self.pending = m.group(), self.pending[m.end():]
            if tok == '\r':
                self.x = 0
            elif tok == '\n':
                self.y = min(self.y + 1, self.rows - 1)
            elif tok == '\b':
                self.x = max(0, self.x - 1)
            elif tok.startswith('\x1b['):
                self.csi(tok[2:-1], tok[-1])
            elif not tok.startswith('\x1b'):
                self.write(tok)

    def write(self, text):
        for ch in text:
            if unicodedata.category(ch) in ('Mn', 'Me', 'Cf'):
                continue
            self.cells[self.y][self.x] = ch
            self.x = min(self.x + (2 if unicodedata.east_asian_width(ch) in 'WF' else 1), self.cols - 1)

    def csi(self, body, final):
        if body == '6' and final == 'n':
            self.asked += 1
        if body[:1] in '?>=<' and body:
            return
        params = [int(p) if p.isdigit() else 0 for p in body.split(';')]
        arg = lambda i, d: params[i] if i < len(params) and params[i] else d
        if final in 'Hf':
            self.y, self.x = min(arg(0, 1), self.rows) - 1, min(arg(1, 1), self.cols) - 1
            if re.fullmatch(r'\d+;', body):
                self.frames.append(self.text())
        elif final == 'A':
            self.y = max(0, self.y - arg(0, 1))
        elif final == 'B':
            self.y = min(self.rows - 1, self.y + arg(0, 1))
        elif final == 'C':
            self.x = min(self.cols - 1, self.x + arg(0, 1))
        elif final == 'D':
            self.x = max(0, self.x - arg(0, 1))
        elif final == 'K':
            row, mode = self.cells[self.y], arg(0, 0)
            lo, hi = {0: (self.x, self.cols), 1: (0, self.x + 1)}.get(mode, (0, self.cols))
            row[lo:hi] = [' '] * (hi - lo)
        elif final == 'J':
            mode = arg(0, 0)
            if mode == 2:
                self.clears += 1
                self.cells = [[' '] * self.cols for _ in range(self.rows)]
            elif mode == 0:
                self.cells[self.y][self.x:] = [' '] * (self.cols - self.x)
                for row in self.cells[self.y + 1:]:
                    row[:] = [' '] * self.cols

    def text(self):
        return '\n'.join(''.join(row).rstrip() for row in self.cells)
`

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
cols, rows = int(sys.argv[5]), int(sys.argv[6])
fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', rows, cols, 0, 0))
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
screen = Screen(cols, rows)
last = ''
def frame_until(description, predicate):
    global last
    deadline = time.monotonic() + 2
    while time.monotonic() < deadline:
        ready, _, _ = select.select([master], [], [], max(0, deadline-time.monotonic()))
        if not ready:
            break
        screen.feed(os.read(master, 65536))
        # Reply as a terminal would to colour/cursor queries during startup.
        while screen.asked:
            os.write(master, b'\x1b]11;rgb:0000/0000/0000\x1b\\\x1b[1;1R')
            screen.asked -= 1
        # The alternate screen is cleared once, as it opens; a redraw never clears it, so it never flickers.
        assert screen.clears <= 1, 'attach cleared the screen'
        # A complete frame ends on the cursor sent to its last row; don't mistake a partial write for a key response.
        frames, screen.frames = screen.frames, []
        for frame in frames:
            last = frame
            if predicate(last):
                return last
    raise AssertionError(description + '\nlast frame:\n' + last)
try:
    initial = frame_until('pipeline with every step within 2 s', lambda s:
        all(word in s for word in ['╭', 'Pipeline', 'Input', 'Triage', 'Research',
                                  'Analysis', 'Epics', 'Export', 'terminal history end']) and
        'Pipeline' in s.split('\n')[0] and 'Agent · claude' in s.split('\n')[0])
    print(initial)
    os.write(master, b'\t')
    frame_until('tab must focus Pipeline', lambda s: 'Pipeline*' in s)
    os.write(master, b'\t')
    frame_until('tab must return focus to Agent', lambda s: re.search(r'Agent[^─]*\*', s) and 'Pipeline*' not in s)
    os.write(master, b'\x1b[5~')
    frame_until('pgup must scroll history', lambda s:
        'terminal scroll history' in s and 'terminal history end' not in s and s != initial)
    if sys.argv[3] == 'sigint':
        os.kill(p.pid, signal.SIGINT)
    else:
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

// attachStep is one step of attachPlan: wait for a frame that shows each of Want and none of Not, append Append to
// the run's agent output, write Write's files in the run folder, release the run's lock, or type Keys.
type attachStep struct {
	Want, Not []string          `json:",omitempty"`
	Append    string            `json:",omitempty"`
	Write     map[string]string `json:",omitempty"`
	Unlock    bool              `json:",omitempty"`
	Keys      string            `json:",omitempty"`
}

// attachPlan runs dulidu attach on a pseudo-terminal of argv[3] by argv[4] in the repository it runs in, on the run
// argv[2], while a live run holds its agent output when the plan says lock; it follows the plan's steps, then wants
// attach to exit with the plan's exit within 5 s, and prints its last frame.
const attachPlan = `
import fcntl, json, os, pty, select, signal, struct, subprocess, sys, termios, time

os.setsid()
master, slave = pty.openpty()
fcntl.ioctl(slave, termios.TIOCSCTTY, 0)
signal.signal(signal.SIGINT, signal.SIG_IGN)
signal.signal(signal.SIGHUP, signal.SIG_IGN)
bin, label, cols, rows, plan = sys.argv[1], sys.argv[2], int(sys.argv[3]), int(sys.argv[4]), json.loads(sys.argv[5])
fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', rows, cols, 0, 0))
run = os.path.join('.dulidu', 'runs', label)
held = None
if plan['lock']:
    held = open(os.path.join(run, 'agent.out'), 'a')
    fcntl.flock(held, fcntl.LOCK_EX)
def terminal_child():
    signal.signal(signal.SIGINT, signal.SIG_DFL)
    signal.signal(signal.SIGHUP, signal.SIG_DFL)
p = subprocess.Popen([bin, 'attach', label], stdin=slave, stdout=slave, stderr=slave, preexec_fn=terminal_child)
screen = Screen(cols, rows)
last = ''
def read(timeout):
    ready, _, _ = select.select([master], [], [], timeout)
    if ready:
        screen.feed(os.read(master, 65536))
        while screen.asked:
            os.write(master, b'\x1b]11;rgb:0000/0000/0000\x1b\\\x1b[1;1R')
            screen.asked -= 1
def frame_until(want, unwanted):
    global last
    deadline = time.monotonic() + 3
    while time.monotonic() < deadline:
        read(max(0, deadline - time.monotonic()))
        frames, screen.frames = screen.frames, []
        for frame in frames:
            last = frame
            if all(w in frame for w in want) and not any(n in frame for n in unwanted):
                return
    raise AssertionError('no frame with %r and without %r\nlast frame:\n%s' % (want, unwanted, last))
try:
    for step in plan['steps']:
        if step.get('Want') or step.get('Not'):
            frame_until(step.get('Want', []), step.get('Not', []))
        if step.get('Append'):
            with open(os.path.join(run, 'agent.out'), 'a') as f:
                f.write(step['Append'])
        for name, text in step.get('Write', {}).items():
            with open(os.path.join(run, name), 'w') as f:
                f.write(text)
        if step.get('Unlock'):
            held.close()
        if step.get('Keys'):
            os.write(master, step['Keys'].encode())
    deadline = time.monotonic() + 5
    while p.poll() is None and time.monotonic() < deadline:
        read(0.01)
    assert p.poll() == plan['exit'], 'attach exited %r, want %r\nlast frame:\n%s' % (p.poll(), plan['exit'], last)
    print(last)
finally:
    if p.poll() is None:
        p.kill()
        p.wait(timeout=1)
    os.close(master)
    os.close(slave)
`

// TestBinaryAttachDrawsEachRunWithRunscreen attaches the built dulidu to a run folder of each kind, on a terminal
// of 120×40: a live run before and after its report, whose pi agent writes a line in two halves, until it ends; a
// failed run with codex's events and dulidu's own lines; a run waiting for answers; a run that never wrote a report.
func TestBinaryAttachDrawsEachRunWithRunscreen(t *testing.T) {
	bin, repo := binary(t), realTempDir(t)
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	t.Setenv("HOME", t.TempDir())
	const (
		started  = `"started": "2026-10-17T01:00:00Z"`
		input    = `{"name": "input", "status": "done", "harness": "claude", ` + started + `, "finished": "2026-10-17T01:00:05Z"}`
		triage   = `{"name": "triage", "status": "passed", "harness": "%s", ` + started + `, "finished": "2026-10-17T01:01:10Z"}`
		piSays   = `{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"%s"}]}}` + "\n"
		halfLine = `{"type":"message_end","message":{"role":"assistant","content":[{"type":"text",`
	)
	steps := func(harness string, more ...string) string {
		return strings.Join(append([]string{input, strings.Replace(triage, "%s", harness, 1)}, more...), ", ")
	}
	research := `{"name": "research", "status": "running", "harness": "pi", "started": "` +
		time.Now().UTC().Format(time.RFC3339) + `", "note": "dimensions: business, users; web: off for pi"}`
	failed := `{"name": "research", "status": "failed at research: market: no sources", "harness": "codex", ` + started +
		`, "finished": "2026-10-17T01:01:42Z", "note": "dimensions: business, market, users; web: on"}`
	codexOut := strings.Join([]string{
		`{"type":"item.completed","item":{"type":"agent_message","text":"codex looked at the market"}}`,
		`{"type":"item.started","item":{"id":"c1","type":"command_execution","command":"ls sources"}}`,
		`{"type":"item.completed","item":{"id":"c1","type":"command_execution","command":"ls sources","exit_code":1,"aggregated_output":"no sources"}}`,
		`dulidu: failed at research: market: no sources`,
	}, "\n") + "\n"
	for _, c := range []struct {
		name  string
		files map[string]string // the run folder's, before attach
		lock  bool
		steps []attachStep
		exit  int
	}{
		{"live run", map[string]string{}, true, []attachStep{
			{Want: []string{"Pipeline", "pending", "Input", "Export"}, Not: []string{"Agent"},
				Write: map[string]string{"report.json": `{"status": "running", ` + started + `, "steps": [` +
					steps("pi", research) + `]}`},
				Append: strings.Replace(piSays, "%s", "pi is researching", 1) + halfLine},
			{Want: []string{"Agent · pi", "pi is researching", "Research", "business", "users", "running"},
				Not: []string{"half written line"}, Append: `"text":"half written line"}]}}` + "\n"},
			{Want: []string{"half written line"}, Unlock: true},
		}, 0},
		{"failed run", map[string]string{"report.json": `{"status": "failed", ` + started + `, "steps": [` +
			steps("codex", failed) + `]}`, "agent.out": codexOut}, false, []attachStep{
			{Want: []string{"Agent · codex", "codex looked at the market", "✗ failed shell ls sources · no sources",
				"Findings", "Research: failed at research: market: no sources", "Log",
				"dulidu: failed at research: market: no sources", "✓ business", "✗ market", "– users", "✗ failed"},
				Keys: "q"},
		}, 0},
		{"waiting run", map[string]string{"report.json": `{"status": "waiting_answers", ` + started + `, "steps": [` +
			steps("claude") + `]}`, "questions.json": `["Who pays?", "For how long?"]`}, false, []attachStep{
			{Want: []string{"Questions", "1. Who pays?", "2. For how long?", "⏸ waiting", "Agent · claude", "starting"},
				Keys: "q"},
		}, 0},
		{"no report", map[string]string{}, false, []attachStep{{Want: []string{"pending", "Input"}}}, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			label := strings.ReplaceAll(c.name, " ", "-")
			c.steps[0].Want = append([]string{label}, c.steps[0].Want...)
			dir := filepath.Join(repo, ".dulidu", "runs", label)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			for name, text := range c.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			plan, err := json.Marshal(map[string]any{"lock": c.lock, "steps": c.steps, "exit": c.exit})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "python3", "-c", terminalScreen+attachPlan, bin, label, "120", "40", string(plan))
			cmd.Dir = repo
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("attach in a terminal: %v\n%s", err, out)
			}
			t.Logf("dulidu attach %s:\n%s", label, out)
		})
	}
}

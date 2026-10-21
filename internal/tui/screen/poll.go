package screen

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// tick is how often a live screen reads the run folder again and advances the spinner.
const tick = 100 * time.Millisecond

// Poll draws load's snapshots on w until q, ctx ends, or load returns an error. keys are the terminal's.
// io.EOF from load draws its final snapshot and exits successfully.
func Poll(ctx context.Context, w io.Writer, keys io.Reader, size func() (int, int), load func(time.Time) (Snap, error)) error {
	if ctx.Err() != nil {
		return nil
	}
	_, _ = fmt.Fprint(w, "\x1b[?1049h\x1b[?25l\x1b[?1000h\x1b[?1006h")
	defer func() { _, _ = fmt.Fprint(w, "\x1b[?1006l\x1b[?1000l\x1b[?25h\x1b[?1049l") }()
	msgs := make(chan tea.Msg, 64)
	go readKeys(keys, msgs)
	cols, rows := 80, 24
	if size != nil {
		cols, rows = size()
	}
	m := New("", cols, rows)
	last := ""
	for {
		if ctx.Err() != nil {
			return nil
		}
		snap, err := load(time.Now())
		if err != nil && err != io.EOF {
			return err
		}
		if size != nil {
			cols, rows = size()
		}
		m.width, m.height, m.spin = cols, rows, m.spin+1
		m.Load(snap)
		if err := redraw(w, m.View(), cols, &last); err != nil {
			return err
		}
		if err == io.EOF {
			return nil
		}
		timer := time.NewTimer(tick)
		for {
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil
			case msg, ok := <-msgs:
				if !ok {
					msgs = nil
					continue
				}
				if key, isKey := msg.(tea.KeyMsg); isKey && (key.String() == "q" || key.String() == "ctrl+c") {
					timer.Stop()
					return nil
				}
				m.Update(msg)
				if err := redraw(w, m.View(), cols, &last); err != nil {
					timer.Stop()
					return err
				}
			case <-timer.C:
				goto next
			}
		}
	next:
	}
}

// redraw paints view over last in place from the top, so the terminal never shows a blank screen.
// An unchanged view writes nothing. A line as wide as the screen keeps its last cell: erasing there would blank it.
func redraw(w io.Writer, view string, cols int, last *string) error {
	if view == *last {
		return nil
	}
	*last = view
	var b strings.Builder
	b.WriteString("\x1b[H")
	for i, line := range strings.Split(view, "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
		if ansi.StringWidth(line) < cols {
			b.WriteString("\x1b[K")
		}
	}
	b.WriteString("\x1b[J")
	_, err := io.WriteString(w, b.String())
	return err
}

func readKeys(r io.Reader, out chan<- tea.Msg) {
	defer close(out)
	if r == nil {
		return
	}
	br := bufio.NewReader(r)
	for {
		b, err := br.ReadByte()
		if err != nil {
			return
		}
		switch b {
		case 'q', 't':
			out <- tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune(b)}}
		case '\t':
			out <- tea.KeyMsg{Type: tea.KeyTab}
		case 0x03:
			out <- tea.KeyMsg{Type: tea.KeyCtrlC}
		case '\x1b':
			parseEsc(br, out)
		}
	}
}

func parseEsc(br *bufio.Reader, out chan<- tea.Msg) {
	b, err := br.ReadByte()
	if err != nil {
		return
	}
	if b == 'O' {
		b, err = br.ReadByte()
		if err != nil {
			return
		}
		switch b {
		case 'A':
			out <- tea.KeyMsg{Type: tea.KeyUp}
		case 'B':
			out <- tea.KeyMsg{Type: tea.KeyDown}
		case 'H':
			out <- tea.KeyMsg{Type: tea.KeyHome}
		case 'F':
			out <- tea.KeyMsg{Type: tea.KeyEnd}
		}
		return
	}
	if b != '[' {
		return
	}
	var seq []byte
	for {
		b, err = br.ReadByte()
		if err != nil {
			return
		}
		seq = append(seq, b)
		if b >= 0x40 && b <= 0x7e {
			break
		}
		if len(seq) > 48 {
			return
		}
	}
	switch string(seq) {
	case "A":
		out <- tea.KeyMsg{Type: tea.KeyUp}
	case "B":
		out <- tea.KeyMsg{Type: tea.KeyDown}
	case "H", "1~":
		out <- tea.KeyMsg{Type: tea.KeyHome}
	case "F", "4~":
		out <- tea.KeyMsg{Type: tea.KeyEnd}
	case "5~":
		out <- tea.KeyMsg{Type: tea.KeyPgUp}
	case "6~":
		out <- tea.KeyMsg{Type: tea.KeyPgDown}
	default:
		if len(seq) > 0 && seq[0] == '<' {
			parseMouse(string(seq), out)
		}
	}
}

func parseMouse(s string, out chan<- tea.Msg) {
	if len(s) < 2 {
		return
	}
	final := s[len(s)-1]
	parts := strings.Split(s[1:len(s)-1], ";")
	if len(parts) != 3 {
		return
	}
	btn, err1 := strconv.Atoi(parts[0])
	x, err2 := strconv.Atoi(parts[1])
	y, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return
	}
	var button tea.MouseButton
	switch btn &^ (4 | 8 | 16 | 32) {
	case 0:
		button = tea.MouseButtonLeft
	case 64:
		button = tea.MouseButtonWheelUp
	case 65:
		button = tea.MouseButtonWheelDown
	default:
		return
	}
	action := tea.MouseActionPress
	if final == 'm' {
		action = tea.MouseActionRelease
	}
	out <- tea.MouseMsg{X: x - 1, Y: y - 1, Button: button, Action: action}
}

package screen

import (
	"cmp"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/parser"

	"github.com/thellmwhisperer/dulidu/internal/run"
)

// Colours are ANSI roles only, so they follow the terminal's theme. Nothing is told by colour alone.
const (
	red, green, yellow, blue, cyan, dim = "1", "2", "3", "4", "6", "8"
)

func init() { lipgloss.SetColorProfile(termenv.ANSI) }

func paint(color, s string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(s)
}

func bold(color, s string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(color)).Render(s)
}

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// states is each state's icon and colour. Running's icon is the spinner. The word is the state's name.
var states = map[string][2]string{
	"pending": {"○", dim}, "running": {"", blue}, "waiting": {"⏸", yellow}, "waiting_answers": {"⏸", yellow},
	"passed": {"✓", green}, "done": {"✓", green}, "failed": {"✗", red}, "skipped": {"–", dim},
	"not run": {"○", dim}, "unknown": {"?", yellow},
}

func (m *Model) icon(state string) string {
	s := states[state]
	if state == "running" {
		s[0] = spinner[m.spin%len(spinner)]
	}
	if s[0] == "" {
		s[0] = "?"
	}
	return paint(cmp.Or(s[1], dim), s[0])
}

var severities = map[string][2]string{"error": {"E", red}, "warning": {"W", yellow}}

// Two columns from 100, the left a third between 38 and 48. Connectors between steps from 30 rows.
const (
	twoColumns, leftMin, leftMax, gap, connectors = 100, 38, 48, 2, 30
)

// View is the screen: the pipeline box, then the Agent, Questions, Findings or Log box beside it from 100
// columns and under it below that, and the footer.
func (m *Model) View() string {
	width := max(m.width, 40)
	available := max(0, m.height-lipgloss.Height(m.footer()))
	if width < twoColumns || !m.sidebar() {
		pipeHeight, rightHeight := available, 0
		if m.sidebar() {
			pipeHeight, rightHeight = stackedHeights(available, m.rightMinimum(width))
		}
		leftBox := m.pipelineBox(width, pipeHeight)
		m.pipelineBounds = bounds{width: width, height: lipgloss.Height(leftBox)}
		m.historyBounds = bounds{width: width}
		m.historyHeight = 0
		panels := []string{leftBox}
		if rightBox := m.right(width, rightHeight); rightBox != "" {
			m.historyBounds.y = lipgloss.Height(leftBox)
			m.historyBounds.height = m.historyHeight
			panels = append(panels, rightBox)
		}
		return strings.Join(panels, "\n") + m.footer()
	}
	left := min(max(width/3, leftMin), leftMax)
	rightWidth := width - left - gap
	m.historyBounds = bounds{x: left + gap, width: rightWidth}
	m.pipelineBounds = bounds{width: left}
	m.historyHeight = 0
	leftBox := m.pipelineBox(left, available)
	rightBox := m.right(rightWidth, available)
	m.pipelineBounds.height = lipgloss.Height(leftBox)
	m.historyBounds.height = m.historyHeight
	return columns(leftBox, rightBox, left) + m.footer()
}

const (
	boxBorderRows       = 2
	minimumBoxRows      = boxBorderRows + 1
	minimumPipelineRows = minimumBoxRows + 2
)

func stackedHeights(available, rightMinimum int) (int, int) {
	right := max(rightMinimum, available/3)
	pipe := available - right
	if pipe < minimumPipelineRows {
		pipe = minimumPipelineRows
		right = available - pipe
	}
	return max(1, pipe), max(1, right)
}

func (m *Model) rightMinimum(width int) int {
	if m.agentShows() || m.questionsShow() {
		if m.showLog() {
			return minimumBoxRows * 2
		}
		return minimumBoxRows
	}
	top := 0
	if len(m.findings) > 0 {
		top = lipgloss.Height(m.findingsBox(width, 0))
	}
	if m.showLog() {
		if top == 0 {
			return minimumBoxRows
		}
		return top + minimumBoxRows
	}
	return top
}

func (m *Model) footer() string {
	gap := "\n\n  "
	if m.height < 10 {
		gap = "\n  "
	}
	keys := lipgloss.NewStyle().Bold(true).Render("q") + " " + paint(dim, "detach")
	if m.agentShows() {
		keys += "  " + lipgloss.NewStyle().Bold(true).Render("t") + " " + paint(dim, "tools")
	}
	return gap + keys
}

func columns(left, right string, width int) string {
	l, r := strings.Split(left, "\n"), strings.Split(right, "\n")
	lines := make([]string, max(len(l), len(r)))
	for i := range lines {
		if i < len(l) {
			lines[i] = l[i] + strings.Repeat(" ", max(0, width-lipgloss.Width(l[i])))
		} else {
			lines[i] = strings.Repeat(" ", width)
		}
		if i < len(r) {
			lines[i] += strings.Repeat(" ", gap) + r[i]
		}
	}
	return strings.Join(lines, "\n")
}

// box is content in a rounded box of width with title in its top border.
func box(title, content string, width int, border string) string {
	inner := width - 4
	fill := strings.Repeat("─", max(1, width-5-lipgloss.Width(title)))
	lines := []string{paint(border, "╭─ ") + title + " " + paint(border, fill+"╮")}
	for _, l := range strings.Split(content, "\n") {
		l = ansi.Truncate(l, inner, "…")
		side := paint(border, "│")
		lines = append(lines, side+" "+l+strings.Repeat(" ", max(0, inner-lipgloss.Width(l)))+" "+side)
	}
	return strings.Join(append(lines, paint(border, "╰"+strings.Repeat("─", width-2)+"╯")), "\n")
}

func (m *Model) border(p *pane) string {
	return map[bool]string{true: cyan, false: dim}[m.focused() == p]
}

func spread(left, right string, width int) string {
	room := width - lipgloss.Width(left) - 1
	if right = ansi.Truncate(right, max(0, room), "…"); right == "" {
		return left
	}
	return left + strings.Repeat(" ", max(1, room+1-lipgloss.Width(right))) + right
}

// pipelineBox is the pipeline, height rows at most: the run's label and status, then a row per step.
func (m *Model) pipelineBox(width, height int) string {
	inner := width - 4
	state := m.icon(m.state) + " " + bold(colorOf(m.state), m.state)
	var rows []paneLine
	for i, r := range m.rows {
		if i > 0 && !r.check && m.height >= connectors {
			key := rowKey(r)
			rows = append(rows, paneLine{key: "connector:" + key, parent: key, text: paint(dim, "│")})
		}
		rows = append(rows, paneLine{key: rowKey(r), text: m.row(r, inner)})
	}
	head := spread(paint(dim, ansi.Truncate(m.label, inner-lipgloss.Width(state)-1, "…")), state, inner)
	lines := append([]string{head, ""}, m.pipeline.show(rows, max(1, height-4))...)
	title := "Pipeline"
	if m.focused() == &m.pipeline {
		title += "*"
	}
	return box(bold(cyan, title), strings.Join(lines, "\n"), width, m.border(&m.pipeline))
}

func rowKey(r *row) string {
	if r.check {
		return "check:" + r.step + ":" + r.label
	}
	return "row:" + r.step
}

func colorOf(state string) string {
	if s, ok := states[state]; ok && s[1] != "" {
		return s[1]
	}
	return dim
}

func (m *Model) row(r *row, width int) string {
	prefix := m.icon(r.state) + " "
	if r.check {
		prefix = "  " + prefix
	}
	took := r.took
	if r.state == "running" && !r.start.IsZero() {
		took = m.now.Sub(r.start)
	}
	suffix := ""
	if r.hasDuration || r.state == "running" {
		suffix = "  " + paint(dim, run.Human(took))
	}
	word := r.state
	status := bold(colorOf(word), word)
	if r.note != "" {
		room := width - lipgloss.Width(prefix) - lipgloss.Width(r.label) - lipgloss.Width(suffix) - lipgloss.Width(status) - 4
		if room > 0 {
			status = paint(dim, ansi.Truncate(r.note, room, "…")+" · ") + status
		}
	}
	labelWidth := max(0, width-lipgloss.Width(prefix)-lipgloss.Width(suffix)-lipgloss.Width(status)-1)
	left := prefix + ansi.Truncate(r.label, labelWidth, "…") + suffix
	left = ansi.Truncate(left, max(0, width-lipgloss.Width(status)-1), "…")
	return spread(left, status, width)
}

func (m *Model) right(width, height int) string {
	if m.questionsShow() {
		return clipRows(m.questionsBox(width, max(1, height-boxBorderRows)), height)
	}
	log := m.showLog()
	if m.agentShows() {
		if log {
			logLines := min(len(m.output), max(1, min(height/3-boxBorderRows, height-minimumBoxRows-boxBorderRows)))
			logBox := m.logBox(width, logLines)
			top := m.agentBox(width, max(1, height-lipgloss.Height(logBox)-boxBorderRows))
			return clipRows(strings.Join([]string{top, logBox}, "\n"), height)
		}
		return clipRows(m.agentBox(width, max(1, height-boxBorderRows)), height)
	}
	if len(m.findings) == 0 {
		if !log {
			return ""
		}
		return clipRows(m.logBox(width, min(len(m.output), max(1, height-boxBorderRows))), height)
	}
	findingsHeight := height
	var logBox string
	if log && height >= minimumBoxRows*2 {
		logLines := min(len(m.output), max(1, min(height/3-boxBorderRows, height-minimumBoxRows-boxBorderRows)))
		logBox = m.logBox(width, logLines)
		findingsHeight = height - lipgloss.Height(logBox)
	}
	top := m.findingsBox(width, max(1, findingsHeight))
	if logBox == "" {
		return clipRows(top, height)
	}
	return clipRows(strings.Join([]string{top, logBox}, "\n"), height)
}

func clipRows(s string, height int) string {
	if height < 1 || s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= height {
		return s
	}
	return strings.Join(lines[:height], "\n")
}

func (m *Model) questionsBox(width, lines int) string {
	inner := max(1, width-4)
	var out []paneLine
	if len(m.questions) == 0 {
		out = []paneLine{{key: "q:empty", text: paint(dim, "waiting")}}
	}
	for i, q := range m.questions {
		lead := fmt.Sprintf("%d. ", i+1)
		bodyW := max(1, inner-ansi.StringWidth(lead))
		parts := wrapPlain(q, bodyW)
		if len(parts) == 0 {
			parts = []string{""}
		}
		for j, p := range parts {
			text := strings.Repeat(" ", ansi.StringWidth(lead)) + p
			if j == 0 {
				text = lead + p
			}
			out = append(out, paneLine{key: fmt.Sprintf("q:%d", i), text: text})
		}
	}
	shown := m.history.show(out, lines)
	pad := max(0, lines-len(shown))
	shown = append(shown, make([]string, pad)...)
	title := "Questions"
	if m.focused() == &m.history {
		title += "*"
	}
	result := box(bold(cyan, title), strings.Join(shown, "\n"), width, m.border(&m.history))
	m.historyHeight = lipgloss.Height(result)
	return result
}

func wrapPlain(s string, width int) []string {
	s = displayText(s)
	if width < 1 {
		width = 1
	}
	if s == "" {
		return []string{""}
	}
	lines := strings.Split(ansi.Wrap(s, width, ""), "\n")
	offset := 0
	for _, line := range lines {
		if line == "" {
			continue
		}
		start := strings.Index(s[offset:], line)
		if start < 0 {
			return lines
		}
		offset += start + len(line)
	}
	if offset >= len(s) {
		return lines
	}
	lines[len(lines)-1] += s[offset:]
	if ansi.StringWidth(lines[len(lines)-1]) <= width {
		return lines
	}
	last := strings.Split(ansi.Hardwrap(lines[len(lines)-1], width, true), "\n")
	return append(lines[:len(lines)-1], last...)
}

var listItem = regexp.MustCompile(`^( {0,3})([-*+]|\d+[.)])[ \t]+(.*)$`)

func markdownText(source []byte, root ast.Node) (string, bool) {
	var out strings.Builder
	code := false
	var render func(ast.Node, lipgloss.Style)
	render = func(node ast.Node, style lipgloss.Style) {
		switch n := node.(type) {
		case *ast.Emphasis:
			style = style.Italic(true)
		case *ast.Strong:
			style = style.Bold(true)
		case *ast.AutoLink:
			out.WriteString(style.Render(displayText(string(n.Label.Value(source)))))
			return
		case *ast.CodeBlock:
			code = true
			out.WriteString(paint(yellow, displayText(strings.TrimSuffix(string(n.Value.Str(source)), "\n"))))
			return
		case *ast.HTMLBlock:
			out.WriteString(displayText(strings.TrimSuffix(string(n.Value.Str(source)), "\n")))
			return
		case *ast.Text:
			out.WriteString(style.Render(displayText(string(n.Value.Value(source)))))
			return
		case *ast.CodeSpan:
			code = true
			var value strings.Builder
			for _, index := range n.Value.Indices() {
				value.Write(source[index.Start:index.Stop])
			}
			out.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(yellow)).Render(displayText(value.String())))
			return
		case *ast.RawHTML:
			out.WriteString(displayText(string(n.Value.Value(source))))
			return
		}
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			render(child, style)
		}
	}
	render(root, lipgloss.NewStyle())
	return out.String(), code
}

func markdown(line string) (string, bool, bool) {
	source := []byte(line)
	root := parser.New().Parse(source)
	for block := root.FirstChild(); block != nil; block = block.NextSibling() {
		if _, ok := block.(*ast.CodeBlock); ok {
			return displayText(line), true, true
		}
	}
	text, code := markdownText(source, root)
	return text, code, false
}

func displayText(value string) string {
	var out strings.Builder
	column := 0
	for _, r := range value {
		if r == '\t' {
			spaces := 8 - column%8
			out.WriteString(strings.Repeat(" ", spaces))
			column += spaces
		} else if !unicode.IsControl(r) {
			out.WriteRune(r)
			column += ansi.StringWidth(string(r))
		}
	}
	return out.String()
}

func fence(line string) (byte, int, string, bool) {
	i := 0
	for i < len(line) && line[i] == ' ' && i < 4 {
		i++
	}
	if i > 3 || i == len(line) || line[i] != '`' && line[i] != '~' {
		return 0, 0, "", false
	}
	marker := line[i]
	start := i
	for i < len(line) && line[i] == marker {
		i++
	}
	if i-start < 3 {
		return 0, 0, "", false
	}
	return marker, i - start, strings.TrimSpace(line[i:]), true
}

func wrapMarkdown(text string, width int) string {
	lines := strings.Split(ansi.Wrap(text, width, ""), "\n")
	plain := ansi.Strip(text)
	offset := 0
	var wrapped []string
	for _, line := range lines {
		visible := ansi.Strip(line)
		start := strings.Index(plain[offset:], visible)
		if start < 0 {
			return ansi.Hardwrap(text, width, true)
		}
		line = plain[offset:offset+start] + line
		offset += start + len(visible)
		if ansi.StringWidth(line) > width {
			wrapped = append(wrapped, strings.Split(ansi.Hardwrap(line, width, true), "\n")...)
		} else {
			wrapped = append(wrapped, line)
		}
	}
	if offset < len(plain) {
		wrapped[len(wrapped)-1] += plain[offset:]
		if ansi.StringWidth(wrapped[len(wrapped)-1]) > width {
			last := strings.Split(ansi.Hardwrap(wrapped[len(wrapped)-1], width, true), "\n")
			wrapped = append(wrapped[:len(wrapped)-1], last...)
		}
	}
	return strings.Join(wrapped, "\n")
}

func plainRows(text string, width int, code bool) []string {
	if code {
		text = wrapMarkdown(text, width)
	} else {
		text = ansi.Wrap(text, width, "")
	}
	return strings.Split(text, "\n")
}

func listRows(line string, width int) ([]string, bool) {
	item := listItem.FindStringSubmatch(line)
	if item == nil {
		return nil, false
	}
	lead := item[1] + cmp.Or(strings.Trim(item[2], "-*+"), "•") + " "
	bodyText, bodyHasCode, _ := markdown(item[3])
	bodyWidth := max(1, width-ansi.StringWidth(lead))
	body := plainRows(bodyText, bodyWidth, bodyHasCode)
	wrapped := append([]string{lead + body[0]}, body[1:]...)
	for i := 1; i < len(wrapped); i++ {
		wrapped[i] = strings.Repeat(" ", ansi.StringWidth(lead)) + wrapped[i]
	}
	return wrapped, true
}

// agentTextRows is the Markdown of text in rows of width, the first behind prefix.
func agentTextRows(prefix, text string, width int) []string {
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, ansi.Strip(text))
	w := max(1, width-6)
	var rows []string
	var codeMarker byte
	codeLength := 0
	for _, line := range strings.Split(text, "\n") {
		var wrapped []string
		marker, length, info, isFence := fence(line)
		if codeLength > 0 {
			if isFence && marker == codeMarker && length >= codeLength && info == "" {
				codeMarker, codeLength = 0, 0
				continue
			}
			wrapped = strings.Split(paint(yellow, ansi.Hardwrap(displayText(line), w, true)), "\n")
		} else if isFence {
			codeMarker, codeLength = marker, length
			continue
		} else {
			lineText, hasCode, isBlock := markdown(line)
			if isBlock {
				wrapped = plainRows(lineText, w, hasCode)
			} else if list, ok := listRows(line, w); ok {
				wrapped = list
			} else {
				wrapped = plainRows(lineText, w, hasCode)
			}
		}
		for _, l := range wrapped {
			if rows == nil {
				rows = append(rows, prefix+l)
			} else {
				rows = append(rows, "  "+l)
			}
		}
	}
	return rows
}

func (m *Model) agentBox(width, lines int) string {
	var out []paneLine
	var calls []*activity
	callBase := 0
	for i, a := range m.agent {
		if a.says == "" && a.thinking == "" {
			if len(calls) == 0 {
				callBase = i
			}
			calls = append(calls, a)
			continue
		}
		out, calls = m.toolLines(out, calls, callBase), nil
		prefix := map[bool]string{true: "✎ ", false: "∴ "}[a.says != ""]
		for _, text := range agentTextRows(prefix, cmp.Or(a.says, a.thinking), width) {
			out = append(out, paneLine{key: fmt.Sprintf("act:%d", i), text: text})
		}
	}
	out = m.toolLines(out, calls, callBase)
	if len(out) == 0 {
		out = []paneLine{{key: "starting", text: paint(dim, "starting")}}
	}
	shown := m.history.show(out, lines)
	pad := max(0, lines-len(shown))
	shown = append(make([]string, pad), shown...)
	m.toggles = make([]string, pad, lines)
	for _, l := range out[m.history.top:min(len(out), m.history.top+lines)] {
		m.toggles = append(m.toggles, l.toggle)
	}
	title := "Agent"
	if m.focused() == &m.history {
		title += "*"
	}
	result := box(bold(cyan, title), strings.Join(shown, "\n"), width, m.border(&m.history))
	m.historyHeight = lipgloss.Height(result)
	return result
}

func (m *Model) toolLines(out []paneLine, calls []*activity, base int) []paneLine {
	start := 0
	for i, a := range calls {
		if a.result == "failed" {
			out = m.appendToolGroup(out, calls[start:i], base+start)
			out = append(out, m.callLine(a, "", base+i))
			start = i + 1
		}
	}
	return m.appendToolGroup(out, calls[start:], base+start)
}

func (m *Model) appendToolGroup(out []paneLine, calls []*activity, base int) []paneLine {
	if len(calls) == 0 {
		return out
	}
	var names []string
	counts, running, runningDetail := map[string]int{}, false, ""
	for _, a := range calls {
		if counts[a.tool]++; counts[a.tool] == 1 {
			names = append(names, a.tool)
		}
		if a.result == "" || a.result == "running" {
			running = true
			runningDetail, _, _ = strings.Cut(a.detail, "\n")
		}
	}
	parts := make([]string, len(names))
	for i, tool := range names {
		parts[i] = tool
		if counts[tool] > 1 {
			parts[i] += fmt.Sprintf(" ×%d", counts[tool])
		}
	}
	n := len(calls)
	line := fmt.Sprintf("%d tool%s: %s", n, map[bool]string{true: "s"}[n > 1], strings.Join(parts, ", "))
	key := fmt.Sprintf("tools:%d", base)
	unfolded := m.unfolded(key)
	if running {
		if !unfolded && runningDetail != "" {
			line += " · " + runningDetail
		}
		out = append(out, paneLine{key: key, toggle: key, text: "  " + m.icon("running") + " " + paint(dim, line)})
	} else {
		out = append(out, paneLine{key: key, toggle: key, text: "  " + paint(dim, "· "+line)})
	}
	if unfolded {
		for i, a := range calls {
			out = append(out, m.callLine(a, key, base+i))
		}
	}
	return out
}

func (m *Model) callLine(a *activity, runKey string, index int) paneLine {
	state, word := "running", "running"
	switch a.result {
	case "passed":
		state, word = "passed", "ok"
	case "failed":
		state, word = "failed", "failed"
	}
	line := a.tool + " " + oneLine(a.detail)
	if summary := oneLine(a.summary); summary != "" {
		line += " · " + summary
	}
	return paneLine{key: fmt.Sprintf("act:%d", index), parent: runKey,
		text: "    " + m.icon(state) + " " + paint(colorOf(state), word) + " " + paint(dim, line)}
}

func oneLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return displayText(ansi.Strip(line))
}

func (m *Model) findingsBox(width, height int) string {
	counts := map[string]int{}
	var out []string
	for i, f := range m.findings {
		counts[f.severity]++
		if i > 0 {
			out = append(out, "")
		}
		s := severities[f.severity]
		out = append(out, "  "+paint(s[1], s[0])+" "+f.ref)
		for _, l := range strings.Split(lipgloss.NewStyle().Width(max(10, width-8)).Render(f.text), "\n") {
			out = append(out, "    "+paint(dim, strings.TrimRight(l, " ")))
		}
	}
	if height > boxBorderRows && len(out) > height-boxBorderRows {
		out = out[:height-boxBorderRows]
	}
	title := bold(cyan, "Findings -")
	for _, sev := range []string{"error", "warning"} {
		if counts[sev] > 0 {
			title += " " + paint(severities[sev][1], fmt.Sprintf("%s %d", severities[sev][0], counts[sev]))
		}
	}
	result := box(title, strings.Join(out, "\n"), width, dim)
	if height > 0 {
		return clipRows(result, height)
	}
	return result
}

func (m *Model) logBox(width, lines int) string {
	out := m.output[max(0, len(m.output)-lines):]
	styled := make([]string, len(out))
	for i, l := range out {
		styled[i] = paint(dim, l)
	}
	return box(bold(cyan, "Log"), strings.Join(styled, "\n"), width, dim)
}

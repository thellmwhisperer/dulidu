// Package text reads a model's plain-text answer: blocks at lines that start with "## ", each with its "Key: value"
// fields, "### Name" sub-sections, "- item" and "1. item" lists and prose. What the answer lacks reads as empty.
package text

import (
	"regexp"
	"strings"
)

// Block is one part of an answer: its fields, items and prose, and its sub-sections.
type Block struct {
	Name     string
	RawLines []string          // original lines after the heading, including lines parsed as fields
	Fields   map[string]string // keys lower-cased
	Items    []string
	Prose    string // its other lines, joined by newlines
	Sections []Block
}

var (
	field = regexp.MustCompile(`^([A-Za-z][\w -]*):\s*(.*)$`)
	fence = regexp.MustCompile("^(`{3,}|~{3,})[^`~]*$")
	item  = regexp.MustCompile(`^(?:-|\d+\.)\s+(.*)$`)
)

// Unfence removes a single fence surrounding the whole answer; unfinished fences stay untouched.
// Research uses this without discarding its lead prose, which is the dimension's content.
func Unfence(answer string) string {
	lines := strings.Split(strings.TrimSpace(answer), "\n")
	if len(lines) < 2 {
		return answer
	}
	open := fence.FindStringSubmatch(strings.TrimSpace(lines[0]))
	if open == nil {
		return answer
	}
	close := strings.TrimSpace(lines[len(lines)-1])
	if len(close) < len(open[1]) || strings.Trim(close, open[1][:1]) != "" {
		return answer
	}
	for _, line := range lines[1 : len(lines)-1] {
		if strings.HasPrefix(strings.TrimSpace(line), open[1]) {
			return answer
		}
	}
	return strings.Join(lines[1:len(lines)-1], "\n")
}

// Parse reads form blocks, the first named "". It removes a whole-answer fence and introductory
// prose before the first heading or known field. List-only answers keep their lead items.
func Parse(answer string) []Block {
	lines := strings.Split(Unfence(answer), "\n")
	prose := false
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if item.MatchString(line) {
			break
		}
		start := strings.HasPrefix(line, "## ")
		if m := field.FindStringSubmatch(line); m != nil {
			switch strings.ToLower(m[1]) {
			case "complexity", "epics", "role", "priority", "depends on", "functionalities":
				start = true
			}
		}
		if start {
			if prose {
				lines = lines[i:]
			}
			break
		}
		prose = prose || line != ""
	}
	blocks := []Block{{}}
	cur := &blocks[0]
	inItem := false // the last line was an item or its wrapped line
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		wrapped := inItem && strings.HasPrefix(rawLine, "  ")
		inItem = false
		if name, ok := strings.CutPrefix(line, "## "); ok {
			blocks = append(blocks, Block{Name: strings.TrimSpace(name)})
			cur = &blocks[len(blocks)-1]
			continue
		}
		b := &blocks[len(blocks)-1]
		b.RawLines = append(b.RawLines, rawLine)
		if name, ok := strings.CutPrefix(line, "### "); ok {
			b.Sections = append(b.Sections, Block{Name: strings.TrimSpace(name)})
			cur = &b.Sections[len(b.Sections)-1]
			continue
		}
		if cur != b {
			cur.RawLines = append(cur.RawLines, rawLine)
		}
		if m := item.FindStringSubmatch(line); m != nil {
			cur.Items = append(cur.Items, m[1])
			inItem = true
		} else if m := field.FindStringSubmatch(line); m != nil {
			if cur.Fields == nil {
				cur.Fields = map[string]string{}
			}
			cur.Fields[strings.ToLower(m[1])] = m[2]
		} else if wrapped && line != "" {
			cur.Items[len(cur.Items)-1] += " " + line
			inItem = true
		} else if line != "" {
			cur.Prose = strings.TrimPrefix(cur.Prose+"\n"+line, "\n")
		}
	}
	return blocks
}

// Field is b's value for key, matched without case.
func (b Block) Field(key string) string { return b.Fields[strings.ToLower(key)] }

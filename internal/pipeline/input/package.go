// Package input packages a run's brief and the files that came with it into contexto.md, the one text file every
// later step reads: text as text, images and videos as their metadata, frames, transcript and a cheap model's
// description.
package input

import (
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

// kinds is every attachment extension input reads, by the kind of section it makes.
var kinds = map[string]string{".pdf": "pdf", ".docx": "docx", ".md": "text", ".txt": "text", ".json": "text",
	".png": "image", ".jpg": "image", ".webp": "image", ".heic": "image", ".mp4": "video", ".mov": "video",
	".webm": "video"}

// FrameSeconds is the seconds between video frames when dulidu.yaml leaves video_frame_seconds empty.
const FrameSeconds = 10

// Ask is the input step's agent: its plain-text answer to prompt, run in the run folder.
type Ask func(prompt string) (string, error)

// section is one file in contexto.md: its row in the index and what goes under its heading.
type section struct {
	name, kind, meta string
	size             int
	body             []string
	image            string // what the model describes for it, relative to the run folder; "" for nothing
	video, audio     bool   // a probed video, for frames; with an audio track, for a transcript
}

func (s *section) add(text ...string) { s.body = append(s.body, text...) }

// Load is the run's input: the brief at brief and the files at paths by name. It refuses an unsupported extension
// before reading anything, and two files with one name.
func Load(brief string, paths []string) (pipeline.InputContext, error) {
	in := pipeline.InputContext{Files: map[string]string{}}
	for _, p := range paths {
		if kinds[strings.ToLower(filepath.Ext(p))] == "" {
			return in, fmt.Errorf("unsupported attachment: %s", p)
		}
	}
	b, err := os.ReadFile(brief)
	if err != nil {
		return in, err
	}
	in.Brief.Text = string(b)
	for _, p := range paths {
		name := filepath.Base(p)
		if _, ok := in.Files[name]; ok {
			return in, fmt.Errorf("duplicate attachment name: %s", name)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return in, err
		}
		in.Files[name] = string(b)
	}
	return in, in.Validate()
}

// Package is PackageEvery with a frame every FrameSeconds.
func Package(dir string, in pipeline.InputContext, ask Ask) (string, error) {
	return PackageEvery(dir, in, FrameSeconds, ask)
}

// PackageEvery is contexto.md for in, whose files are already in dir/input: first everything code can do, the same
// bytes every time, a video's frame every seconds under dir/input/frames/<video>/, then one ask per image and per
// video's contact sheet. A video without a required multimedia tool gets a note in its section instead.
func PackageEvery(dir string, in pipeline.InputContext, seconds int, ask Ask) (string, error) {
	files := []*section{{name: "brief.md", kind: "brief", size: len(in.Brief.Text), body: []string{in.Brief.Text}}}
	for _, name := range slices.Sorted(maps.Keys(in.Files)) {
		s := &section{name: name, kind: kinds[strings.ToLower(filepath.Ext(name))], size: len(in.Files[name])}
		path := filepath.Join(dir, "input", name)
		var text string
		var err error
		switch s.kind {
		case "text":
			text = in.Files[name]
		case "pdf":
			text, err = pdf(path)
		case "docx":
			text, err = docx(path)
		case "video":
			err = s.probeVideo(path)
		default:
			err = s.probe(path)
		}
		if err != nil {
			return "", fmt.Errorf("%s: %w", name, err)
		}
		if text != "" {
			s.add(text)
		}
		files = append(files, s)
	}
	for _, s := range files {
		if s.video {
			if err := s.frames(dir, seconds); err != nil {
				return "", fmt.Errorf("%s: %w", s.name, err)
			}
		}
	}
	for _, s := range files {
		if s.image == "" {
			continue
		}
		text, err := ask(prompt(s.image, s.video, seconds))
		if err != nil {
			return "", fmt.Errorf("%s: %w", s.name, err)
		}
		s.add("### Description\n\n" + strings.TrimSpace(text))
	}
	return render(files), nil
}

// render is contexto.md: the index, then each file under its heading, behind an anchor of its position.
func render(files []*section) string {
	var b strings.Builder
	b.WriteString("# Contexto\n\n| File | Type | Size | Dimensions or duration | Section |\n| --- | --- | --- | --- | --- |\n")
	for i, s := range files {
		fmt.Fprintf(&b, "| %s | %s | %d bytes | %s | [#f%d](#f%d) |\n", cell(s.name), s.kind, s.size, s.meta, i+1, i+1)
	}
	for i, s := range files {
		fmt.Fprintf(&b, "\n<a id=\"f%d\"></a>\n\n## %s\n", i+1, s.name)
		for _, text := range s.body {
			b.WriteString("\n" + strings.TrimSpace(text) + "\n")
		}
	}
	return b.String()
}

func cell(s string) string { return strings.ReplaceAll(s, "|", `\|`) }

// missing is the note for a video section whose tool is not installed, or "" when every tool is.
func missing(tools ...string) string {
	for _, t := range tools {
		if _, err := exec.LookPath(t); err != nil {
			return "Not read: " + t + " is not installed; dulidu doctor says how to install it."
		}
	}
	return ""
}

// tool runs name with args and is its stdout; its error ends with its stderr.
func tool(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	if exit, ok := err.(*exec.ExitError); ok {
		err = fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(exit.Stderr)))
	}
	return string(out), err
}

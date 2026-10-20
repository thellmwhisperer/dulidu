//go:build unix

package pipeline_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

func TestBinaryRunKeepsStartOfLongVideo(t *testing.T) {
	bin := binary(t)
	for _, tc := range []struct {
		name, duration  string
		seconds, frames int
	}{
		{"over budget", "1860.5", 1800, 180},
		{"at budget", "1800", 1800, 180},
		{"within budget", "3", 3, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := realTempDir(t)
			t.Setenv("HOME", t.TempDir())
			t.Setenv("VIDEO_DURATION", tc.duration)
			if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("git init: %v\n%s", err, out)
			}
			for name, body := range map[string]string{
				"brief.md": binaryRead(t, filepath.Join(binaryExample, "brief.md")),
				"clip.mp4": "original video",
			} {
				if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			tools := t.TempDir()
			for _, name := range []string{"git", "cat", "wc", "ps"} {
				path, err := exec.LookPath(name)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path, filepath.Join(tools, name)); err != nil {
					t.Fatal(err)
				}
			}
			// The fake media tools expose the selected time range in frames and speech.
			// An uncapped extraction includes the tail; seeking loses the beginning.
			for name, script := range map[string]string{
				"ffprobe": `printf '{"streams":[{"codec_type":"video"},{"codec_type":"audio"}],"format":{"duration":"%s"}}\n' "$VIDEO_DURATION"`,
				"ffmpeg": `duration=${VIDEO_DURATION%.*}
start=0
previous=
for arg; do
  case "$previous" in
    -t) duration=$arg ;;
    -ss) start=$arg ;;
  esac
  previous=$arg
  output=$arg
done
if [ "$duration" -gt "${VIDEO_DURATION%.*}" ]; then duration=${VIDEO_DURATION%.*}; fi
case "$output" in
  *.wav) printf 'Speech from %s to %s seconds.\n' "$start" "$((start + duration))" > "$output" ;;
  *sheet.jpg) : > "$output" ;;
  *) i=1; second=$start
     while [ "$second" -lt "$((start + duration))" ]; do
       printf 'Frame at %s seconds.\n' "$second" > "$(printf "$output" "$i")"
       i=$((i + 1)); second=$((second + 10))
     done ;;
esac`,
				"whisper-cli": `for arg; do wav=$arg; done; cat "$wav"`,
			} {
				if err := os.WriteFile(filepath.Join(tools, name), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", tools)
			answers, _ := filepath.Glob(filepath.Join(binaryExample, "answers", "*"))
			replay := fakeReplay(t, answers)
			for _, args := range [][]string{{"init"}, {"run", "brief.md", "--attach", "clip.mp4"}} {
				cmd := exec.Command(bin, args...)
				cmd.Dir = repo
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("dulidu %s: %v\n%s", strings.Join(args, " "), err, out)
				}
			}
			if n := binaryLines(filepath.Join(replay, "calls")); n != len(answers) {
				t.Errorf("model calls = %d, want %d", n, len(answers))
			}
			dirs, _ := filepath.Glob(filepath.Join(repo, ".dulidu", "runs", "*"))
			if len(dirs) != 1 {
				t.Fatalf("run folders: %v", dirs)
			}
			r, err := pipeline.Read(dirs[0])
			if err != nil || r.Report.Status != "done" {
				t.Fatalf("run should finish: status %q, error %v", r.Report.Status, err)
			}
			context := binaryRead(t, filepath.Join(dirs[0], "contexto.md"))
			_, section, ok := strings.Cut(context, "## clip.mp4\n")
			if !ok {
				t.Fatalf("missing video section:\n%s", context)
			}
			note := "Read first 1800 s of 1860.5 s."
			if tc.name == "over budget" {
				if strings.Count(section, note) != 1 || !strings.Contains(section, "\n"+note+"\n") {
					t.Errorf("want one duration note in video section:\n%s", section)
				}
			} else if strings.Contains(section, "Read first") {
				t.Errorf("in-budget video has a truncation note:\n%s", section)
			}
			transcript := fmt.Sprintf("### Transcript\n\nSpeech from 0 to %d seconds.\n", tc.seconds)
			if !strings.Contains(section, transcript) {
				t.Errorf("want transcript %q:\n%s", transcript, section)
			}
			framesDir := filepath.Join(dirs[0], "input", "frames", "clip.mp4")
			frames, _ := filepath.Glob(filepath.Join(framesDir, "*.jpg"))
			if len(frames) != tc.frames+1 { // includes the contact sheet
				t.Errorf("got %d frame files, want %d plus sheet", len(frames), tc.frames)
			}
			var listed []string
			for i := 1; i <= tc.frames; i++ {
				name := fmt.Sprintf("%03d.jpg", i)
				listed = append(listed, "input/frames/clip.mp4/"+name)
				if got := binaryRead(t, filepath.Join(framesDir, name)); got != fmt.Sprintf("Frame at %d seconds.\n", (i-1)*10) {
					t.Errorf("frame %s: %q", name, got)
				}
			}
			if !strings.Contains(section, "Frames, one every 10 s: "+strings.Join(listed, ", ")+". Contact sheet: input/frames/clip.mp4/sheet.jpg.") {
				t.Errorf("video section does not list exactly the retained frames:\n%s", section)
			}
			if got := binaryRead(t, filepath.Join(dirs[0], "input", "clip.mp4")); got != "original video" {
				t.Error("original attachment changed")
			}
		})
	}
}

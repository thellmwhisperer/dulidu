//go:build unix

package input

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// video is a new test pattern video of seconds at path, with a tone when sound is set.
func video(t *testing.T, path string, seconds string, sound bool) string {
	t.Helper()
	args := []string{"-v", "error", "-f", "lavfi", "-i", "testsrc=size=64x48:rate=5:duration=" + seconds}
	if sound {
		args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:duration="+seconds)
	}
	if out, err := exec.Command("ffmpeg", append(args, "-pix_fmt", "yuv420p", "-bitexact", path)...).CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %v\n%s", err, out)
	}
	return path
}

// files is the names of the files in dir.
func files(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// logged is a fake tool that appends its call to calls.
func logged(calls string) string { return "echo \"${0##*/} $*\" >> " + calls + "\n" }

func TestPackageVideoFramesSheetAndTranscript(t *testing.T) {
	need(t, "ffmpeg", "ffprobe", "pdftotext")
	t.Setenv("WHISPER_MODEL", "")
	calls := filepath.Join(t.TempDir(), "calls")
	fakeBin(t, false, map[string]string{"whisper-cli": logged(calls) + "printf '\\n Hello from the tone.\\n'\n"})
	tmp := t.TempDir()
	long, short := video(t, filepath.Join(tmp, "demo.mp4"), "20", true), video(t, filepath.Join(tmp, "short.mov"), "3", false)
	in := load(t, filepath.Join("testdata", "notes.md"), filepath.Join("testdata", "terms.pdf"),
		filepath.Join("testdata", "pattern.png"), long, short)
	dir, context, asked, err := pack(t, in)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("contexto.md:\n%s", context)
	for name, want := range map[string][]string{"demo.mp4": {"001.jpg", "002.jpg", "sheet.jpg"}, "short.mov": {"001.jpg", "sheet.jpg"}} {
		if got := files(t, filepath.Join(dir, "input", "frames", name)); !slices.Equal(got, want) {
			t.Errorf("input/frames/%s holds %v, want %v", name, got, want)
		}
	}
	for _, want := range []string{"| demo.mp4 | video |", "| 20.0 s | [#f2](#f2) |", "| 3.0 s | [#f5](#f5) |",
		"| pattern.png | image | 469 bytes | 64x48 |", "Sellers list a bike", "Refunds within fourteen days.",
		"### Transcript\n\nHello from the tone."} {
		if !strings.Contains(context, want) {
			t.Errorf("contexto.md lacks %q", want)
		}
	}
	if len(asked) != 3 || !strings.Contains(asked[0], "input/frames/demo.mp4/sheet.jpg") ||
		!strings.Contains(asked[0], "one every 10 seconds") || !strings.Contains(asked[1], "input/pattern.png") ||
		!strings.Contains(asked[2], "input/frames/short.mov/sheet.jpg") {
		t.Errorf("want one ask per image and per contact sheet, got %q", asked)
	}
	for _, name := range []string{"demo.mp4", "short.mov"} {
		section := strings.SplitN(strings.SplitN(context, "## "+name+"\n", 2)[1], "\n## ", 2)[0]
		if !strings.Contains(section, "### Description\n\nA fake description.") {
			t.Errorf("%s has no description under its heading", name)
		}
	}
	if got := read(t, calls); strings.Count(got, "whisper-cli") != 1 || !strings.Contains(got, "-m models/ggml-base.en.bin") {
		t.Errorf("want one transcription, of the video with sound, by the default model, got %q", got)
	}
	for _, p := range []string{long, short} {
		if read(t, filepath.Join(dir, "input", filepath.Base(p))) != read(t, p) {
			t.Errorf("input/%s is not the attachment byte for byte", filepath.Base(p))
		}
	}
	again, context2, _, err := pack(t, in)
	if err != nil || context2 != context {
		t.Errorf("contexto.md differs between two runs (%v):\n%s", err, context2)
	}
	for _, name := range []string{"demo.mp4", "short.mov"} {
		frames := filepath.Join(dir, "input", "frames", name)
		for _, f := range files(t, frames) {
			if read(t, filepath.Join(frames, f)) != read(t, filepath.Join(again, "input", "frames", name, f)) {
				t.Errorf("%s/%s differs between two runs", name, f)
			}
		}
	}
}

// TestPackageTranscribesRealSpeech needs a local whisper model at $WHISPER_MODEL and whisper.cpp's samples/jfk.wav
// at $WHISPER_SPEECH; it downloads nothing.
func TestPackageTranscribesRealSpeech(t *testing.T) {
	speech := os.Getenv("WHISPER_SPEECH")
	if os.Getenv("WHISPER_MODEL") == "" || speech == "" {
		t.Skip("WHISPER_MODEL and WHISPER_SPEECH are not set")
	}
	need(t, "ffmpeg", "ffprobe", "whisper-cli")
	path := filepath.Join(t.TempDir(), "speech.mp4")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=64x48:rate=5:duration=20",
		"-i", speech, "-af", "apad", "-t", "20", "-pix_fmt", "yuv420p", path).CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %v\n%s", err, out)
	}
	_, context, _, err := pack(t, load(t, path))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("contexto.md:\n%s", context)
	if !strings.Contains(strings.ToLower(context), "ask not what your country can do for you") {
		t.Error("the transcript is not the video's speech")
	}
}

func TestPackageReadsStartOfAVideoOver30MinutesAlongsideOtherAttachments(t *testing.T) {
	calls := filepath.Join(t.TempDir(), "calls")
	fakeBin(t, true, map[string]string{
		"ffprobe": `case "$*" in *long.mp4*) d=1860.5 ;; *) d=3 ;; esac
echo '{"streams":[{"codec_type":"video","width":64,"height":48},{"codec_type":"audio"}],"format":{"duration":"'$d'"}}'` + "\n",
		"ffmpeg":      logged(calls) + `for a; do f=$a; done; : > "$(printf "$f" 1)"`,
		"whisper-cli": logged(calls) + "echo 'The beginning of the video.'\n",
	})
	dir := t.TempDir() // the fake ffprobe reads none of them
	for _, name := range []string{"a.png", "a.mp4", "long.mp4"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, context, asked, err := pack(t, load(t, filepath.Join(dir, "a.png"), filepath.Join(dir, "a.mp4"), filepath.Join(dir, "long.mp4")))
	if err != nil {
		t.Fatalf("want the run to retain the start of the long video, got %v", err)
	}
	_, section, _ := strings.Cut(context, "## long.mp4\n")
	if strings.Count(context, "Read first 1800 s of 1860.5 s.") != 1 ||
		!strings.Contains(section, "\nRead first 1800 s of 1860.5 s.\n") ||
		!strings.Contains(section, "### Transcript\n\nThe beginning of the video.") {
		t.Errorf("long video lacks its duration note or transcript:\n%s", context)
	}
	if len(asked) != 3 || strings.Count(context, "### Description\n\nA fake description.") != 3 {
		t.Errorf("want descriptions of both videos and the image, got %d asks:\n%s", len(asked), context)
	}
	if got := read(t, calls); strings.Count(got, "whisper-cli") != 2 || strings.Count(got, "-t 1800") != 4 {
		t.Errorf("want bounded frames and audio for both videos, got %s", got)
	}
}

func TestPackageNotesAMissingMediaToolInTheVideosSection(t *testing.T) {
	for _, c := range []struct {
		name    string
		scripts map[string]string
		want    string
		asks    int
	}{
		{"no ffmpeg", nil, "Not read: ffmpeg is not installed; dulidu doctor says how to install it.", 0},
		{"no whisper-cli", map[string]string{
			"ffprobe": `echo '{"streams":[{"codec_type":"video"},{"codec_type":"audio"}],"format":{"duration":"3"}}'`,
			"ffmpeg":  `for a; do f=$a; done; : > "$(printf "$f" 1)"`, // its last argument, %03d as 001
		}, "### Transcript\n\nNot read: whisper-cli is not installed; dulidu doctor says how to install it.", 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "clip.webm")
			if err := os.WriteFile(path, []byte("webm"), 0o644); err != nil {
				t.Fatal(err)
			}
			in := load(t, path)
			fakeBin(t, true, c.scripts)
			dir, context, asked, err := pack(t, in)
			if err != nil || !strings.Contains(context, c.want) || len(asked) != c.asks {
				t.Fatalf("want %q in the section, got %v, %d asks:\n%s", c.want, err, len(asked), context)
			}
			if read(t, filepath.Join(dir, "input", "clip.webm")) != "webm" {
				t.Error("input/clip.webm is not the attachment")
			}
		})
	}
}

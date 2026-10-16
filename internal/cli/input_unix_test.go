//go:build unix

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRunRejectsAnAudioOnlyAttachmentBeforeAnyAgentCall(t *testing.T) {
	calls := runRepo(t, map[string]string{"talk.mp3": "mp3", "shot.png": "png"})
	code, _, errOut := dulidu(t, "run", "brief.md", "--attach", "shot.png", "--attach", "talk.mp3")
	if code != 1 || !strings.Contains(errOut, "unsupported attachment: talk.mp3") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if _, err := os.Stat(calls); err == nil {
		t.Error("the agent was called")
	}
	if dirs := runs(t); len(dirs) != 0 {
		t.Errorf("a run folder was written: %v", dirs)
	}
}

func TestRunPackagesAVideoWithDocumentsAndImages(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe", "pdftotext"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	files := map[string]string{"notes.md": "Spain first.\n"}
	for _, name := range []string{"terms.pdf", "pattern.png"} {
		files[name] = read(t, filepath.Join("..", "pipeline", "input", "testdata", name))
	}
	calls := runRepo(t, files)
	whisper := filepath.Join(t.TempDir(), "whisper-cli")
	if err := os.WriteFile(whisper, []byte("#!/bin/sh\necho 'Bikes, second hand.'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(whisper)+string(os.PathListSeparator)+os.Getenv("PATH"))
	yml := strings.Replace(read(t, "dulidu.yaml"), "video_frame_seconds:\n", "video_frame_seconds: 5\n", 1)
	if err := os.WriteFile("dulidu.yaml", []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=64x48:rate=5:duration=20",
		"-f", "lavfi", "-i", "sine=duration=20", "-pix_fmt", "yuv420p", "demo.mp4").CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %v\n%s", err, out)
	}
	video := read(t, "demo.mp4")
	if code, out, errOut := dulidu(t, "run", "brief.md", "--attach", "notes.md", "--attach", "terms.pdf",
		"--attach", "pattern.png", "--attach", "demo.mp4"); code != 0 {
		t.Fatalf("dulidu run: exit %d\n%s%s", code, out, errOut)
	}
	dirs := runs(t)
	if len(dirs) != 1 {
		t.Fatalf("want 1 run folder, got %v", dirs)
	}
	context := read(t, filepath.Join(dirs[0], "contexto.md"))
	t.Logf("contexto.md:\n%s", context)
	for _, want := range []string{"| demo.mp4 | video |", "| 20.0 s | [#f2](#f2) |", "<a id=\"f2\"></a>\n\n## demo.mp4\n",
		"Frames, one every 5 s:", "### Transcript\n\nBikes, second hand.", "Spain first.", "Refunds within fourteen days.",
		"| pattern.png | image | 469 bytes | 64x48 |"} {
		if !strings.Contains(context, want) {
			t.Errorf("contexto.md lacks %q", want)
		}
	}
	frames, err := os.ReadDir(filepath.Join(dirs[0], "input", "frames", "demo.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range frames {
		names = append(names, f.Name())
	}
	if !slices.Equal(names, []string{"001.jpg", "002.jpg", "003.jpg", "004.jpg", "sheet.jpg"}) {
		t.Errorf("input/frames/demo.mp4 holds %v, want a frame every 5 s and a contact sheet", names)
	}
	if got := read(t, calls); strings.Count(got, "\n") != 2 {
		t.Errorf("want one call for the image and one for the contact sheet, got %q", got)
	}
	if read(t, filepath.Join(dirs[0], "input", "demo.mp4")) != video {
		t.Error("input/demo.mp4 is not the attachment")
	}
}

func TestDoctorNamesAMissingMediaToolAndFails(t *testing.T) {
	for _, c := range []struct{ missing, want string }{
		{"ffmpeg", "fail ffmpeg: not found in PATH: install it with brew install ffmpeg"},
		{"whisper-cli", "fail whisper-cli: not found in PATH: install it with brew install whisper-cpp"},
	} {
		t.Run(c.missing, func(t *testing.T) {
			repo(t)
			bin := t.TempDir()
			for _, name := range []string{"go", "git"} {
				path, err := exec.LookPath(name)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path, filepath.Join(bin, name)); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range []string{"ffmpeg", "ffprobe", "whisper-cli"} {
				if name == c.missing {
					continue
				}
				if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin)
			code, out, _ := dulidu(t, "doctor")
			if code != 1 || !strings.Contains(out, c.want) || strings.Count(out, "fail ") != 1 || !strings.Contains(out, "ok config:") {
				t.Fatalf("dulidu doctor without %s: exit %d\n%s", c.missing, code, out)
			}
		})
	}
}

func TestDoctorAgentNeedsNoMediaTool(t *testing.T) {
	agentRepo(t, "pi", "ok", "")
	bin := strings.SplitN(os.Getenv("PATH"), string(os.PathListSeparator), 2)[0] // the fake pi alone
	for _, name := range []string{"go", "git"} {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(path, filepath.Join(bin, name)); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	if code, out, errOut := dulidu(t, "doctor", "--agent"); code != 0 {
		t.Fatalf("dulidu doctor --agent with only go, git and the agent: exit %d\n%s%s", code, out, errOut)
	}
}

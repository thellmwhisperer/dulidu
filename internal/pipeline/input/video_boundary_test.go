//go:build unix

package input

import (
	"fmt"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thellmwhisperer/dulidu/internal/pipeline"
)

func TestPackageVideoWithLiteralPatternCharacters(t *testing.T) {
	need(t, "ffmpeg", "ffprobe")
	name := "demo[1]%.mp4"
	source := video(t, filepath.Join(t.TempDir(), name), "3", false)
	in := load(t, source)
	dir := filepath.Join(t.TempDir(), "run[1]%")
	if err := pipeline.Write(dir, pipeline.Run{Report: pipeline.Report{Status: "running", Started: time.Now()}, Input: &in}); err != nil {
		t.Fatal(err)
	}
	var calls int
	context, err := Package(dir, in, func(string) (string, error) {
		calls++
		return "A fake description.", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	frames := filepath.Join(dir, "input", "frames", name)
	if got := files(t, frames); len(got) != 2 || got[0] != "001.jpg" || got[1] != "sheet.jpg" {
		t.Errorf("frames and sheet: %v", got)
	}
	if calls != 1 || !strings.Contains(context, "### Description\n\nA fake description.") || !strings.Contains(context, "input/frames/"+name+"/sheet.jpg") {
		t.Errorf("want one sheet description, got %d calls:\n%s", calls, context)
	}
	if read(t, filepath.Join(dir, "input", name)) != read(t, source) {
		t.Error("source video bytes changed")
	}
}

func TestContactSheetHoldsLastOf1800Frames(t *testing.T) {
	need(t, "ffmpeg")
	dir := t.TempDir()
	name := "long.mp4"
	if err := os.MkdirAll(filepath.Join(dir, "input"), 0o755); err != nil {
		t.Fatal(err)
	}
	video(t, filepath.Join(dir, "input", name), "1", false)
	frames := filepath.Join(dir, "input", "frames", name)
	if err := os.MkdirAll(frames, 0o755); err != nil {
		t.Fatal(err)
	}
	bluePath := filepath.Join(dir, "blue.jpg")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-i", filepath.Join("testdata", "blue.jpg"), "-vf", "scale=64:48", "-frames:v", "1", bluePath).CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %v\n%s", err, out)
	}
	blueFrame, err := os.ReadFile(bluePath)
	if err != nil {
		t.Fatal(err)
	}
	for i := 2; i <= 1800; i++ {
		if err := os.WriteFile(filepath.Join(frames, fmt.Sprintf("%03d.jpg", i)), blueFrame, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := section{name: name, kind: "video"}
	if err := s.frames(dir, 1); err != nil {
		t.Fatal(err)
	}
	want := make([]string, 1800)
	for i := 1; i <= 1800; i++ {
		want[i-1] = fmt.Sprintf("input/frames/%s/%03d.jpg", name, i)
	}
	if !strings.Contains(render([]*section{&s}), "Frames, one every 1 s: "+strings.Join(want, ", ")+". Contact sheet: input/frames/long.mp4/sheet.jpg.") {
		t.Error("contexto.md does not list every frame in numeric reading order")
	}
	f, err := os.Open(filepath.Join(frames, "sheet.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := jpeg.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() > 8192 || b.Dy() > 8192 || b.Dx() == 0 || b.Dy() == 0 {
		t.Fatalf("contact sheet dimensions: %v", b)
	}
	cols := 64
	w, h := b.Dx()/cols, b.Dy()/29
	r, g, blue, _ := img.At((1799%cols)*w+w/2, (1799/cols)*h+h/2).RGBA()
	if r+g+blue < 1000 {
		t.Error("last frame is missing from the contact sheet")
	}
}

package input

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// maxVideo is the longest video input reads, in seconds: a longer one needs its own frame budget.
const maxVideo = 1800

// probeVideo gives s, a video at path, its duration and whether it has sound, then marks it for frames. A video
// over maxVideo is an error, before any frame.
func (s *section) probeVideo(path string) error {
	if note := missing("ffmpeg", "ffprobe"); note != "" {
		s.add(note)
		return nil
	}
	out, err := tool("ffprobe", "-v", "error", "-show_entries", "format=duration:stream=codec_type", "-of", "json", path)
	if err != nil {
		return err
	}
	var p struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
		}
		Format struct{ Duration string }
	}
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		return err
	}
	d, err := strconv.ParseFloat(p.Format.Duration, 64)
	if err != nil {
		return fmt.Errorf("ffprobe returned no duration: %w", err)
	}
	if d > maxVideo {
		return fmt.Errorf("video of %.0f s is over %d s: a longer video needs its own frame budget", d, maxVideo)
	}
	for _, st := range p.Streams {
		s.audio = s.audio || st.CodecType == "audio"
	}
	s.meta, s.video = fmt.Sprintf("%.1f s", d), true
	return nil
}

// frames takes the first frame of s's video and one every seconds after it into dir/input/frames/<video>/, tiles
// them into sheet.jpg for the model to describe, and transcribes its audio track, if it has one.
func (s *section) frames(dir string, seconds int) error {
	rel := filepath.Join("input", "frames", s.name)
	out, video := filepath.Join(dir, rel), filepath.Join(dir, "input", s.name)
	if err := os.RemoveAll(out); err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	every := fmt.Sprintf("select='isnan(prev_selected_t)+gte(t-prev_selected_t,%d)'", seconds)
	pattern := filepath.Join(out, "%03d.jpg")
	if _, err := tool("ffmpeg", "-v", "error", "-i", video, "-vf", every, "-fps_mode", "vfr", "-q:v", "2", pattern); err != nil {
		return err
	}
	frames, _ := filepath.Glob(filepath.Join(out, "[0-9]*.jpg")) // the pattern is well formed
	if len(frames) == 0 {
		return errors.New("ffmpeg extracted no frames")
	}
	cols := min(len(frames), 4)
	tile := fmt.Sprintf("scale=320:-2,tile=%dx%d", cols, (len(frames)+cols-1)/cols)
	sheet := filepath.Join(rel, "sheet.jpg")
	if _, err := tool("ffmpeg", "-v", "error", "-i", pattern, "-vf", tile, "-frames:v", "1", "-q:v", "2",
		filepath.Join(dir, sheet)); err != nil {
		return err
	}
	s.image = sheet
	for i, f := range frames {
		frames[i] = filepath.Join(rel, filepath.Base(f))
	}
	s.add(fmt.Sprintf("Frames, one every %d s: %s. Contact sheet: %s.", seconds, strings.Join(frames, ", "), sheet))
	if !s.audio {
		return nil
	}
	if note := missing("whisper-cli"); note != "" {
		s.add("### Transcript\n\n" + note)
		return nil
	}
	text, err := transcribe(video)
	if err != nil {
		return err
	}
	s.add("### Transcript\n\n" + cmp.Or(text, "No speech."))
	return nil
}

// transcribe is the speech in video, by whisper-cli on its audio as 16 kHz mono WAV. The model is $WHISPER_MODEL,
// else whisper-cli's own default, models/ggml-base.en.bin; neither is downloaded.
func transcribe(video string) (string, error) {
	tmp, err := os.MkdirTemp("", "dulidu-audio-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	wav := filepath.Join(tmp, "audio.wav")
	if _, err := tool("ffmpeg", "-v", "error", "-i", video, "-vn", "-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le", wav); err != nil {
		return "", err
	}
	model := cmp.Or(os.Getenv("WHISPER_MODEL"), "models/ggml-base.en.bin")
	out, err := tool("whisper-cli", "-m", model, "-l", "auto", "-nt", "-np", "-f", wav)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	return strings.Join(lines, "\n"), err
}

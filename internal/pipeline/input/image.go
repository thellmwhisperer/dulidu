package input

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
)

func (s *section) probe(path string) error {
	s.image = filepath.Join("input", s.name)
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return fmt.Errorf("ffprobe is not installed: brew install ffmpeg, or apt install ffmpeg")
	}
	out, err := tool("ffprobe", "-v", "error", "-show_entries", "stream=width,height", "-of", "json", path)
	if err != nil {
		return err
	}
	var p struct{ Streams []struct{ Width, Height int } }
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		return err
	}
	for _, st := range p.Streams {
		if st.Width > 0 && st.Height > 0 {
			s.meta = fmt.Sprintf("%dx%d", st.Width, st.Height)
			return nil
		}
	}
	return fmt.Errorf("ffprobe did not return positive width and height")
}

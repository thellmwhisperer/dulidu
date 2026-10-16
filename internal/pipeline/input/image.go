package input

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
)

// probe marks s, an image at path, for the model to describe and gives it its dimensions, by ffprobe when it is
// installed; without it the section says so.
func (s *section) probe(path string) error {
	s.image = filepath.Join("input", s.name)
	if _, err := exec.LookPath("ffprobe"); err != nil {
		s.add("Dimensions not read: ffprobe is not installed; brew install ffmpeg, or apt install ffmpeg.")
		return nil
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
		if st.Width > 0 {
			s.meta = fmt.Sprintf("%dx%d", st.Width, st.Height)
			break
		}
	}
	return nil
}

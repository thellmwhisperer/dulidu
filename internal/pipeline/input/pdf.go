package input

import (
	"os/exec"
	"strings"
)

// pdf is the text of the PDF at path, by pdftotext. Without pdftotext, or without text, s gets a note instead: its
// text is not lost silently.
func (s *section) pdf(path string) (string, error) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		s.add("Not read: pdftotext is not installed: brew install poppler, or apt install poppler-utils.")
		return "", nil
	}
	text, err := tool("pdftotext", "-enc", "UTF-8", path, "-")
	if err != nil {
		return "", err
	}
	text = strings.ReplaceAll(text, "\f", "\n")
	if strings.TrimSpace(text) == "" {
		s.add("Not read: pdftotext extracted no text from the PDF.")
		return "", nil
	}
	return text, nil
}

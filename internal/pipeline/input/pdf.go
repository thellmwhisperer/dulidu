package input

import (
	"errors"
	"os/exec"
	"strings"
)

// pdf is the text of the PDF at path, by pdftotext. Without pdftotext the run stops: its text is not lost silently.
func pdf(path string) (string, error) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return "", errors.New("pdftotext is not installed: brew install poppler, or apt install poppler-utils")
	}
	text, err := tool("pdftotext", "-enc", "UTF-8", path, "-")
	if err != nil {
		return "", err
	}
	text = strings.ReplaceAll(text, "\f", "\n")
	if strings.TrimSpace(text) == "" {
		return "", errors.New("pdftotext extracted no text from PDF")
	}
	return text, nil
}

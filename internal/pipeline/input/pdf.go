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
	return strings.ReplaceAll(text, "\f", "\n"), err
}

//go:build unix

package input

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageNotesAPDFWithoutPdftotext(t *testing.T) {
	in := load(t, filepath.Join("testdata", "terms.pdf"))
	fakeBin(t, true, nil)
	want := "## terms.pdf\n\nNot read: pdftotext is not installed: brew install poppler, or apt install poppler-utils.\n"
	if _, context, _, err := pack(t, in); err != nil || !strings.HasSuffix(context, want) {
		t.Fatalf("want the PDF noted as not read, got %v and\n%s", err, context)
	}
}

func TestDocxSeparatesBreaksTabsAndParagraphs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan.docx")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	w, err := z.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` +
		`<w:p><w:r><w:t>10</w:t><w:br/><w:t>20</w:t><w:cr/><w:t>30</w:t><w:tab/><w:t>40</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:tabs><w:tab w:val="left" w:pos="720"/></w:tabs></w:pPr><w:r><w:t xml:space="preserve">50 &amp; 60</w:t></w:r></w:p>` +
		`</w:body></w:document>`))
	if err == nil {
		err = z.Close()
	}
	if err == nil {
		err = f.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	_, context, _, err := pack(t, load(t, path))
	if err != nil {
		t.Fatal(err)
	}
	if want := "## plan.docx\n\n10\n20\n30\t40\n50 & 60\n"; !strings.HasSuffix(context, want) {
		t.Fatalf("want %q at the end of:\n%s", want, context)
	}
}

func TestLoadRefusesTwoFilesWithOneNameAndMissingOnes(t *testing.T) {
	brief, other := filepath.Join("testdata", "notes.md"), t.TempDir()
	for _, name := range []string{"notes.md", "brief.md"} {
		if err := os.WriteFile(filepath.Join(other, name), []byte("other\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for paths, want := range map[[2]string]string{
		{brief, filepath.Join(other, "notes.md")}: "duplicate attachment name: notes.md",
		{brief, "missing/foo.xyz"}:                "open missing/foo.xyz",
		{brief, filepath.Join(other, "brief.md")}: "input: colliding names",
	} {
		_, err := Load(brief, paths[:])
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Load %v: got %v, want %q", paths, err, want)
		}
	}
}

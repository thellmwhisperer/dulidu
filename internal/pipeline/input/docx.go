package input

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

// docx is the text of the Word document at path: each paragraph a line, a tab a tab, a break or carriage return a
// new line.
func docx(path string) (string, error) {
	z, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = z.Close() }()
	f, err := z.Open("word/document.xml")
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	var b strings.Builder
	var text, stops bool // inside w:t; inside w:tabs, whose w:tab is a tab stop, not a tab
	for d := xml.NewDecoder(f); ; {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return b.String(), nil
		}
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				text = true
			case "tabs":
				stops = true
			case "tab":
				if !stops {
					b.WriteByte('\t')
				}
			case "br", "cr":
				b.WriteByte('\n')
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				text = false
			case "tabs":
				stops = false
			case "p":
				b.WriteByte('\n')
			}
		case xml.CharData:
			if text {
				b.Write(t)
			}
		}
	}
}

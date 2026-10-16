//go:build unix

package input

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageRejectsImagesWithoutDimensionsBeforeDescription(t *testing.T) {
	for _, tc := range []struct {
		name, script string
	}{
		{"missing ffprobe", ""},
		{"no streams", `printf '{"streams":[]}'`},
		{"width only", `printf '{"streams":[{"width":64}]}'`},
		{"height only", `printf '{"streams":[{"height":48}]}'`},
		{"zero height", `printf '{"streams":[{"width":64,"height":0}]}'`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := load(t, filepath.Join("testdata", "pattern.png"))
			scripts := map[string]string{}
			if tc.script != "" {
				scripts["ffprobe"] = tc.script + "\n"
			}
			fakeBin(t, true, scripts)
			_, context, asked, err := pack(t, in)
			if err == nil || !strings.Contains(err.Error(), "ffprobe") || context != "" || len(asked) != 0 {
				t.Fatalf("want dimensions failure without description, got error %v, context %q, calls %q", err, context, asked)
			}
		})
	}
}

func TestPackageRejectsPDFWithoutTextBeforeDescription(t *testing.T) {
	in := load(t, filepath.Join("testdata", "terms.pdf"), filepath.Join("testdata", "pattern.png"))
	fakeBin(t, true, map[string]string{
		"pdftotext": `printf ' \f\n\t'` + "\n",
		"ffprobe":   `printf '{"streams":[{"width":64,"height":48}]}'` + "\n",
	})
	_, context, asked, err := pack(t, in)
	if err == nil || !strings.Contains(err.Error(), "pdftotext extracted no text") || context != "" || len(asked) != 0 {
		t.Fatalf("want PDF extraction failure without description, got error %v, context %q, calls %q", err, context, asked)
	}
}

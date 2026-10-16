package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeDulidu is a script that answers --version as dulidu at tag would, or an exit 1 when tag is empty.
func fakeDulidu(tag string) string {
	if tag == "" {
		return "#!/bin/sh\nexit 1\n"
	}
	return "#!/bin/sh\necho 'dulidu " + tag + " (abc1234) 2026-10-16T00:00:00Z'\n"
}

// fakeRelease serves tag the way GitHub serves a release: the API, the archive and checksums.txt. A wrong sum is
// checksums.txt naming another archive's sha256.
func fakeRelease(t *testing.T, tag, binary string, wrongSum bool) {
	t.Helper()
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	tw := tar.NewWriter(zw)
	if err := tw.WriteHeader(&tar.Header{Name: "dulidu", Mode: 0o755, Size: int64(len(binary))}); err != nil {
		t.Fatal(err)
	}
	_, _ = tw.Write([]byte(binary))
	_ = tw.Close()
	_ = zw.Close()
	name := fmt.Sprintf("dulidu-%s-%s-%s.tar.gz", tag, runtime.GOOS, runtime.GOARCH)
	sum := sha256.Sum256(gz.Bytes())
	if wrongSum {
		sum = sha256.Sum256([]byte("another archive"))
	}
	files := map[string][]byte{name: gz.Bytes(), "checksums.txt": []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n")}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/thellmwhisperer/dulidu/releases/latest" {
			assets := []map[string]string{}
			for n := range files {
				assets = append(assets, map[string]string{"name": n, "browser_download_url": srv.URL + "/download/" + n})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": tag, "assets": assets})
			return
		}
		if b, ok := files[strings.TrimPrefix(r.URL.Path, "/download/")]; ok {
			_, _ = w.Write(b)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("DULIDU_GITHUB_API", srv.URL)
}

// installed is a dulidu at tag in a new directory.
func installed(t *testing.T, tag string) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "dulidu")
	if err := os.WriteFile(target, []byte(fakeDulidu(tag)), 0o755); err != nil {
		t.Fatal(err)
	}
	return target
}

func TestUpdate(t *testing.T) {
	for _, c := range []struct {
		name, binary string
		wrongSum     bool
		want, err    string
	}{
		{"replaces the binary with the latest release", fakeDulidu("v0.2.0"), false, fakeDulidu("v0.2.0"), ""},
		{"with a wrong checksum touches nothing", fakeDulidu("v0.2.0"), true, fakeDulidu("v0.1.0"), "checksum"},
		{"rolls back a binary that does not answer", fakeDulidu(""), false, fakeDulidu("v0.1.0"), "previous"},
	} {
		t.Run(c.name, func(t *testing.T) {
			target := installed(t, "v0.1.0")
			fakeRelease(t, "v0.2.0", c.binary, c.wrongSum)
			var out bytes.Buffer
			err := update(&out, "v0.1.0", target)
			if (c.err == "") != (err == nil) || err != nil && !strings.Contains(err.Error(), c.err) {
				t.Fatalf("update: %v, want an error with %q", err, c.err)
			}
			if got := read(t, target); got != c.want {
				t.Fatalf("dulidu after update: %q, want %q", got, c.want)
			}
			if entries, _ := os.ReadDir(filepath.Dir(target)); len(entries) != 1 {
				t.Fatalf("update left %v next to dulidu", entries)
			}
		})
	}
}

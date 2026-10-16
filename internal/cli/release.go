package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

// published is a GitHub release of dulidu and its assets.
type published struct {
	Tag    string `json:"tag_name"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// release is "latest" or "tags/<tag>" of dulidu's releases; DULIDU_GITHUB_API stands in for GitHub's API.
func release(which string) (published, error) {
	api := os.Getenv("DULIDU_GITHUB_API")
	if api == "" {
		api = "https://api.github.com"
	}
	var rel published
	b, err := get(api + "/repos/thellmwhisperer/dulidu/releases/" + which)
	if err == nil {
		err = json.Unmarshal(b, &rel)
	}
	return rel, err
}

func get(url string) ([]byte, error) {
	resp, err := http.Get(url) //nolint:gosec // the URL is GitHub's, or the release's own asset
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func (r published) asset(name string) ([]byte, error) {
	for _, a := range r.Assets {
		if a.Name == name {
			return get(a.URL)
		}
	}
	return nil, fmt.Errorf("release %s publishes no %s", r.Tag, name)
}

// binary is the dulidu inside this machine's archive, once its sha256 matches the release's checksums.txt.
func (r published) binary() ([]byte, error) {
	name := fmt.Sprintf("dulidu-%s-%s-%s.tar.gz", r.Tag, runtime.GOOS, runtime.GOARCH)
	archive, err := r.asset(name)
	if err != nil {
		return nil, err
	}
	sums, err := r.asset("checksums.txt")
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(archive)
	if !slices.Contains(strings.Split(string(sums), "\n"), hex.EncodeToString(sum[:])+"  "+name) {
		return nil, fmt.Errorf("the checksum of %s does not match checksums.txt: nothing was touched", name)
	}
	zr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if err != nil {
			return nil, fmt.Errorf("%s carries no dulidu: %w", name, err)
		}
		if path.Base(h.Name) == "dulidu" {
			return io.ReadAll(tr)
		}
	}
}

func swap(target string, bin []byte, tag string) error {
	f, err := os.CreateTemp(filepath.Dir(target), ".dulidu-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(bin)
	err = errors.Join(err, f.Chmod(0o755), f.Close())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.Name(), "--version")
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	if err != nil || !strings.HasPrefix(string(out), "dulidu "+tag+" ") {
		return fmt.Errorf("dulidu %s does not answer --version (%q, %v): the installed dulidu is unchanged", tag, out, err)
	}
	return os.Rename(f.Name(), target)
}

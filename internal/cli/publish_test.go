package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const testCommit = "0123456789abcdef0123456789abcdef01234567"

func releaseFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Chdir(root)
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"git":  "case \"$1\" in rev-parse) printf '%s\\n' \"$FAKE_ROOT\";; -C) printf '%s\\n' \"$FAKE_SHA\";; *) exit 9;; esac",
		"make": "printf '%s\\n' \"$@\" > \"$FAKE_ROOT/make.args\"; [ \"${FAIL_MAKE:-}\" != 1 ]",
		"gh":   "printf '%s\\n' \"$@\" > \"$FAKE_ROOT/gh.args\"; [ \"${FAIL_GH:-}\" != 1 ]",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_ROOT", root)
	t.Setenv("FAKE_SHA", testCommit)
	return root
}

func called(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestReleaseRejectsInvalidTagsBeforeCommands(t *testing.T) {
	for _, args := range [][]string{{}, {"1.2.3"}, {"v1.2"}, {"v1.2.3", "extra"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			root := releaseFixture(t)
			code, out, errOut := dulidu(t, append([]string{"release"}, args...)...)
			if code != 2 || out != "" || !strings.Contains(errOut, "Usage:") ||
				called(filepath.Join(root, "make.args")) || called(filepath.Join(root, "gh.args")) {
				t.Fatalf("release %v: exit %d, stdout %q, stderr %q", args, code, out, errOut)
			}
		})
	}
}

func TestReleaseBuildsAndPublishesExpectedAssets(t *testing.T) {
	root := releaseFixture(t)
	code, out, errOut := dulidu(t, "release", "v1.2.3")
	if code != 0 || !strings.Contains(out, "published dulidu v1.2.3") {
		t.Fatalf("release: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	if got := strings.Fields(read(t, filepath.Join(root, "make.args"))); !slices.Equal(got, []string{"-C", root, "dist", "TAG=v1.2.3"}) {
		t.Fatalf("build args: %v", got)
	}
	want := []string{"release", "create", "v1.2.3", "--target", testCommit}
	for _, platform := range []string{"darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64"} {
		want = append(want, filepath.Join("dist", "dulidu-v1.2.3-"+platform+".tar.gz"))
	}
	want = append(want, filepath.Join("dist", "checksums.txt"))
	if got := strings.Fields(read(t, filepath.Join(root, "gh.args"))); !slices.Equal(got, want) {
		t.Fatalf("publication args: got %v, want %v", got, want)
	}
}

func TestReleaseStopsOnBuildOrPublicationFailure(t *testing.T) {
	for _, failed := range []string{"MAKE", "GH"} {
		t.Run(failed, func(t *testing.T) {
			root := releaseFixture(t)
			t.Setenv("FAIL_"+failed, "1")
			code, out, _ := dulidu(t, "release", "v1.2.3")
			if code != 1 || strings.Contains(out, "published") || !called(filepath.Join(root, "make.args")) ||
				called(filepath.Join(root, "gh.args")) != (failed == "GH") {
				t.Fatalf("%s failure: exit %d, stdout %q", failed, code, out)
			}
		})
	}
}

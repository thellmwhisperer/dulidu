package cli

import (
	"fmt"
	"os"
	"os/exec"
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
		"git": `case "$1:$3" in
  rev-parse:) printf '%s\n' "$FAKE_ROOT";;
  -C:rev-parse) printf '%s\n' "$FAKE_SHA";;
  -C:ls-remote)
    [ "$4:$5:$6:$7" = '--tags:origin:refs/tags/v1.2.3:refs/tags/v1.2.3^{}' ] || exit 9
    [ "${FAIL_LOOKUP:-}" != 1 ] || exit 9
    printf '%s' "${REMOTE_TAG_OUTPUT:-}";;
  -C:remote) [ "$4:$5" = get-url:origin ] && printf '%s\n' "$FAKE_ORIGIN";;
  *) exit 9;;
esac`,
		"make": "printf '%s\\n' \"$@\" > \"$FAKE_ROOT/make.args\"; [ \"${FAIL_MAKE:-}\" != 1 ]",
		"gh":   "printf '%s\\n' \"$@\" > \"$FAKE_ROOT/gh.args\"; printf '%s' \"${GH_REPO:-}\" > \"$FAKE_ROOT/gh.repo\"; [ \"${FAIL_GH:-}\" != 1 ]",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_ROOT", root)
	t.Setenv("FAKE_SHA", testCommit)
	t.Setenv("FAKE_ORIGIN", "git@github.com:fixture-owner/dulidu.git")
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

func TestReleaseChecksExistingRemoteTag(t *testing.T) {
	ref := "refs/tags/v1.2.3"
	other := strings.Repeat("f", 40)
	object := strings.Repeat("a", 40)
	for _, tc := range []struct {
		name, refs            string
		lookupFails, succeeds bool
	}{
		{"absent", "", false, true},
		{"matching lightweight", fmt.Sprintf("%s\t%s\n", testCommit, ref), false, true},
		{"mismatching lightweight", fmt.Sprintf("%s\t%s\n", other, ref), false, false},
		{"matching annotated", fmt.Sprintf("%s\t%s\n%s\t%s^{}\n", object, ref, testCommit, ref), false, true},
		{"mismatching annotated", fmt.Sprintf("%s\t%s\n%s\t%s^{}\n", object, ref, other, ref), false, false},
		{"lookup failure", "", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := releaseFixture(t)
			t.Setenv("REMOTE_TAG_OUTPUT", tc.refs)
			if tc.lookupFails {
				t.Setenv("FAIL_LOOKUP", "1")
			}
			code, out, _ := dulidu(t, "release", "v1.2.3")
			if (code == 0) != tc.succeeds || strings.Contains(out, "published") != tc.succeeds ||
				called(filepath.Join(root, "make.args")) != tc.succeeds ||
				called(filepath.Join(root, "gh.args")) != tc.succeeds {
				t.Fatalf("remote tag %s: exit %d, stdout %q", tc.name, code, out)
			}
			if tc.succeeds {
				args := strings.Fields(read(t, filepath.Join(root, "gh.args")))
				if len(args) != 10 || args[4] != testCommit {
					t.Fatalf("publication args: %v", args)
				}
			}
		})
	}
}

func TestReleasePublishesToLookedUpRepository(t *testing.T) {
	mismatch := fmt.Sprintf("%s\trefs/tags/v1.2.3\n", strings.Repeat("f", 40))
	for _, refs := range []string{"", mismatch} {
		root := releaseFixture(t)
		t.Setenv("GH_REPO", "fixture-owner/different-repository")
		t.Setenv("REMOTE_TAG_OUTPUT", refs)
		code, _, _ := dulidu(t, "release", "v1.2.3")
		if refs == mismatch {
			if code == 0 || called(filepath.Join(root, "make.args")) || called(filepath.Join(root, "gh.args")) {
				t.Fatalf("mismatching origin tag with GH_REPO: exit %d", code)
			}
		} else if code != 0 || read(t, filepath.Join(root, "gh.repo")) != os.Getenv("FAKE_ORIGIN") {
			t.Fatalf("publication repository not bound to lookup remote: exit %d", code)
		}
	}
}

func TestMakeReleasePassesLiteralTag(t *testing.T) {
	makePath, err := exec.LookPath("make")
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	bin := filepath.Join(scratch, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"make", "gh"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\ntouch \"$FAKE_CALLS\"\nexit 9\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(scratch, "marker")
	calls := filepath.Join(scratch, "calls")
	cmd := exec.Command(makePath, "-C", root, "release", "TAG=$(shell touch "+marker+")")
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "FAKE_CALLS="+calls)
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "Usage:") || called(marker) || called(calls) {
		t.Fatalf("make release with invalid literal tag: error %v, output %q, marker %v, external calls %v", err, out, called(marker), called(calls))
	}
}

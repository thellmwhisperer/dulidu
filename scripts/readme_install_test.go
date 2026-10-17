package scripts

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestReadmeInstall: the install section labels the curl line as the public-repository form and tells a
// private-clone reader to run the clone's install.sh with DULIDU_PREFIX.
func TestReadmeInstall(t *testing.T) {
	b, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatal(err)
	}
	readme := string(b)
	start := strings.Index(readme, "## Install\n")
	if start < 0 {
		t.Fatal("README has no ## Install section")
	}
	section := readme[start:]
	section = section[:strings.Index(section, "\n## ")+1]

	curl := "curl -fsSL https://raw.githubusercontent.com/thellmwhisperer/dulidu/main/install.sh | sh"
	at := strings.Index(section, curl)
	if at < 0 {
		t.Fatalf("install section lost the curl line %q", curl)
	}
	// The label is the paragraph right above the curl line's code fence.
	paras := strings.Split(section[:at], "\n\n")
	if before := paras[len(paras)-2]; !strings.Contains(before, "public repository") {
		t.Errorf("no label with \"public repository\" just above the curl line:\n%s", before)
	}

	for _, w := range []string{"clone", "private"} {
		if !strings.Contains(section, w) {
			t.Errorf("install section does not say %q", w)
		}
	}
	local := regexp.MustCompile(`DULIDU_PREFIX=\S+ .*install\.sh`)
	found := false
	for _, line := range strings.Split(section, "\n") {
		if local.MatchString(line) && !strings.Contains(line, "curl") {
			found = true
		}
	}
	if !found {
		t.Errorf("install section has no DULIDU_PREFIX=<dir> ... install.sh line without curl:\n%s", section)
	}
}

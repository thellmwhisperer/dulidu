package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadReadsTheLayersThatExist(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	if _, found, err := Load(root, home); err != nil || len(found) != 0 {
		t.Fatalf("no layer: found %v, %v", found, err)
	}
	layers := Layers(root, home)
	for _, l := range layers {
		write(t, l, "# nothing set\n")
	}
	if _, found, err := Load(root, home); err != nil || !slices.Equal(found, layers) {
		t.Fatalf("both layers: found %v, %v", found, err)
	}
}

func TestLoadRejectsAKeyDuliduDoesNotKnow(t *testing.T) {
	for _, layer := range []int{0, 1} {
		root, home := t.TempDir(), t.TempDir()
		path := Layers(root, home)[layer]
		write(t, path, "install: npm ci\n")
		if _, _, err := Load(root, home); err == nil || !strings.Contains(err.Error(), path) {
			t.Fatalf("an unknown key in %s: %v", path, err)
		}
	}
}

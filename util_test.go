package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEqual(t *testing.T) {
	cases := []struct {
		a        []string
		b        []string
		expected bool
	}{
		{[]string{"a", "b"}, []string{"a", "b"}, true},
		{[]string{"a", "b"}, []string{"a", "c"}, false},
		{[]string{"a"}, []string{"a", "b"}, false},
		{[]string{}, []string{}, true},
	}

	for _, c := range cases {
		if got := equal(c.a, c.b); got != c.expected {
			t.Errorf("equal(%v, %v): expected %v, got %v", c.a, c.b, c.expected, got)
		}
	}
}

func TestStripOptionals(t *testing.T) {
	got := stripOptionals([]string{"[global]", "list", "[options]"})
	if len(got) != 1 || got[0] != "list" {
		t.Errorf("expected [list], got %v", got)
	}
}

func TestWriteJson(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")

	if err := writeJson(path, map[string]string{"key": "value"}); err != nil {
		t.Fatalf("failed to write json: %s", err)
	}

	if !fileExistsMain(path) {
		t.Fatal("expected cache file to exist after writeJson")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read cache file: %s", err)
	}
	if string(content) != `{"key":"value"}`+"\n" {
		t.Errorf("unexpected cache file content: %q", string(content))
	}
}

func fileExistsMain(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

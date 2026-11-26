package drive

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFormatSize(t *testing.T) {
	cases := []struct {
		bytes    int64
		force    bool
		expected string
	}{
		{0, false, ""},
		{999, false, "999.0 B"},
		{1500, false, "1.5 KB"},
		{999 * 1000 * 1000, false, "999.0 MB"},
		{1234, true, "1234 B"},
	}

	for _, c := range cases {
		if got := formatSize(c.bytes, c.force); got != c.expected {
			t.Errorf("formatSize(%d, %v): expected %q, got %q", c.bytes, c.force, c.expected, got)
		}
	}
}

func TestTruncateString(t *testing.T) {
	// Strings shorter than the max length are returned unchanged
	if got := truncateString("abc", 10); got != "abc" {
		t.Errorf("expected %q, got %q", "abc", got)
	}

	// Max length below 9 is not supported and returns the input unchanged
	if got := truncateString("abcdefghijklmnopqrstuvwxyz", 5); got != "abcdefghijklmnopqrstuvwxyz" {
		t.Errorf("expected input string returned for max length below 9, got %q", got)
	}

	// Long strings are truncated to the max length with an ellipsis in the middle
	got := truncateString("abcdefghijklmnopqrstuvwxyz", 10)
	if got != "abcd...xyz" {
		t.Errorf("expected %q, got %q", "abcd...xyz", got)
	}
	if utf8.RuneCountInString(got) != 10 {
		t.Errorf("expected truncated string of 10 runes, got %d", utf8.RuneCountInString(got))
	}
}

func TestFormatBool(t *testing.T) {
	if got := formatBool(true); !strings.EqualFold(got, "true") {
		t.Errorf("expected true, got %s", got)
	}
	if got := formatBool(false); !strings.EqualFold(got, "false") {
		t.Errorf("expected false, got %s", got)
	}
}

package shrt

import (
	"regexp"
	"strings"
	"testing"
)

var codePattern = regexp.MustCompile(`^[a-zA-Z0-9]{6}$`)

func TestNewCode(t *testing.T) {
	seen := make(map[rune]bool)
	for range 2000 {
		code := newCode()
		if !codePattern.MatchString(code) {
			t.Fatalf("code %q, want 6 characters of a-z A-Z 0-9", code)
		}
		for _, r := range code {
			seen[r] = true
		}
	}
	// 12000 draws over 62 characters miss one only by a broken generator.
	if len(seen) != len(codeAlphabet) {
		var missing []string
		for _, r := range codeAlphabet {
			if !seen[r] {
				missing = append(missing, string(r))
			}
		}
		t.Errorf("never drew %s", strings.Join(missing, " "))
	}
}

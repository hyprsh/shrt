package main

import (
	"slices"
	"testing"
)

// The commits of a release: a feat, a fix, a docs, a breaking feat with
// a footer, an unscoped fix, a subject that isn't Conventional, and the
// release's own commit.
func fixture() []Commit {
	return []Commit{
		{"a1b2c3d", "feat(api): add the links endpoint", "Explains it.\n\nCloses #1\n"},
		{"b2c3d4e", "fix(store): keep links across restarts", "Refs #2\nRefs hyprsh/nixos-config#2\n"},
		{"c3d4e5f", "docs(docs): describe the API", "Closes hyprsh/shrt#3\nCloses #3\n"},
		{"d4e5f6a", "feat(redirect)!: move redirects", "Why.\n\nBREAKING CHANGE: redirects live under /r/\n  now, not /x/.\nBookmarks break.\n\nRefs #5, #4\nCo-Authored-By: someone\n"},
		{"e5f6a7b", "fix: an old unscoped fix", ""},
		{"f6a7b8c", "Update the readme", ""},
	}
}

const wantNotes = `### Breaking changes

- **redirect**: move redirects (#4, #5, d4e5f6a)

  redirects live under /r/
    now, not /x/.
  Bookmarks break.

### Features

- **api**: add the links endpoint (#1, a1b2c3d)

### Fixes

- **store**: keep links across restarts (#2, b2c3d4e)
- an old unscoped fix (e5f6a7b)

<details>
<summary>Internal</summary>

- **docs**: describe the API (#3, c3d4e5f)
- Update the readme (f6a7b8c)

</details>

**Full diff**: https://github.com/hyprsh/shrt/compare/v0.1.0...v0.2.0
`

func TestReleaseNotesInTheSharedFormat(t *testing.T) {
	got := ReleaseNotes(fixture(), Span{Repo: "hyprsh/shrt", From: "v0.1.0", To: "v0.2.0"})
	if got != wantNotes {
		t.Errorf("notes:\n%s\nwant:\n%s", got, wantNotes)
	}
}

func TestFirstReleaseHasNoFullDiff(t *testing.T) {
	got := ReleaseNotes(fixture()[:1], Span{Repo: "hyprsh/shrt", To: "v0.1.0"})
	want := "### Features\n\n- **api**: add the links endpoint (#1, a1b2c3d)\n"
	if got != want {
		t.Errorf("notes %q, want %q", got, want)
	}
}

func TestNoIssuesLeavesTheSHAAlone(t *testing.T) {
	got := ReleaseNotes([]Commit{{"abc1234", "refactor(store): tidy", ""}}, Span{Repo: "hyprsh/shrt", To: "v0.1.0"})
	want := "<details>\n<summary>Internal</summary>\n\n- **store**: tidy (abc1234)\n\n</details>\n"
	if got != want {
		t.Errorf("notes %q, want %q", got, want)
	}
}

func TestNextVersion(t *testing.T) {
	feat := Commit{Subject: "feat(api): x"}
	fix := Commit{Subject: "fix(api): x"}
	docs := Commit{Subject: "docs(docs): x"}
	bang := Commit{Subject: "refactor(api)!: x"}
	footer := Commit{Subject: "fix(api): x", Body: "BREAKING CHANGE: y\n"}
	plain := Commit{Subject: "Update things"}
	for _, tc := range []struct {
		name    string
		last    string
		commits []Commit
		want    string
	}{
		{"no tag yet", "", []Commit{fix}, "v0.1.0"},
		{"no tag, feat", "", []Commit{feat}, "v0.1.0"},
		{"feat bumps the minor on 0.x", "v0.3.2", []Commit{fix, feat}, "v0.4.0"},
		{"bang bumps the minor on 0.x", "v0.3.2", []Commit{bang}, "v0.4.0"},
		{"footer bumps the minor on 0.x", "v0.3.2", []Commit{footer}, "v0.4.0"},
		{"fix bumps the patch", "v0.3.2", []Commit{fix}, "v0.3.3"},
		{"docs bump the patch", "v0.3.2", []Commit{docs, plain}, "v0.3.3"},
		{"breaking bumps the major from 1.0", "v1.2.3", []Commit{feat, bang}, "v2.0.0"},
		{"feat bumps the minor from 1.0", "v1.2.3", []Commit{feat}, "v1.3.0"},
	} {
		got, err := NextVersion(tc.last, tc.commits)
		if err != nil || got != tc.want {
			t.Errorf("%s: %q, %v; want %q", tc.name, got, err, tc.want)
		}
	}
	if _, err := NextVersion("v1.0.0-rc1", []Commit{fix}); err == nil {
		t.Error("a tag that isn't a version should be an error")
	}
}

func TestReferredIssues(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       []int
	}{
		{"bare after a space and a comma", "Refs #5, #3,#4", []int{3, 4, 5}},
		{"bare at the line start", "Closes: #7", []int{7}},
		{"bare after a word is no issue", "Refs foo#5 and a#6", nil},
		{"bare needs the digits to end", "Refs #5abc", nil},
		{"qualified", "Refs hyprsh/shrt#5", []int{5}},
		{"qualified in brackets", "Refs (hyprsh/shrt#5)", []int{5}},
		{"qualified, any case", "Refs HyprSh/Shrt#5", []int{5}},
		{"qualified after a word", "Refs xhyprsh/shrt#5", nil},
		{"qualified after a path", "Refs foo/hyprsh/shrt#5", nil},
		{"another repo", "Refs hyprsh/nixos-config#2", nil},
		{"only referring lines", "See #9\nCloses #1\nFixes #1", []int{1}},
	} {
		got := referred(tc.body, "hyprsh/shrt")
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}

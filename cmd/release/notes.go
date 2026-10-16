package main

// What a release of shrt is: the version its commits call for, and its
// notes, in the format every hyprsh repo's releases share (a port of
// hyprsh/autopilot's generator, without Specs).

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Commit is a commit since the last tag: its short SHA, subject and body.
type Commit struct {
	SHA     string
	Subject string
	Body    string
}

// Span is what a release covers: the repo's owner/name, the tag it
// starts from (empty for the first release) and the tag it makes.
type Span struct {
	Repo string
	From string
	To   string
}

// The patterns the regexes below are built from. referringPattern is a
// line that closes tickets, as GitHub reads them, or a `Refs` line.
const (
	referringPattern = `^(?:(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)|refs)[: ]`
	breakingPattern  = `BREAKING[ -]CHANGE:`
)

var (
	// conventional matches a Conventional Commit's subject: type, scope if
	// any, `!` if breaking, and what it says.
	conventional = regexp.MustCompile(`^(\w+)(?:\(([^)]*)\))?(!?):\s*(.*)$`)
	// breakingFooterStart matches the line a BREAKING CHANGE: footer starts on.
	breakingFooterStart = regexp.MustCompile(`^` + breakingPattern + `\s*(.*)$`)
	// breakingInBody finds a BREAKING CHANGE: footer in a body.
	breakingInBody = regexp.MustCompile(`(?m)^` + breakingPattern)
	// referring matches the lines that refer to issues.
	referring = regexp.MustCompile(`(?i)` + referringPattern)
	// footerEnd matches a line that ends a BREAKING CHANGE footer: one that
	// refers to issues, a trailer like `Co-Authored-By:`, or another footer.
	footerEnd = regexp.MustCompile(`(?i)` + referringPattern + `|^[a-z]+(?:-[a-z]+)+:|^` + breakingPattern)
	// bareIssue finds `#12` at the start of a line or after a space or a comma.
	bareIssue = regexp.MustCompile(`(?:^|[ ,])#(\d+)\b`)
	// versionTag matches a release tag, capturing major, minor and patch.
	versionTag = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)
	// firstChar finds the first character of every non-empty line.
	firstChar = regexp.MustCompile(`(?m)^(.)`)
)

// commitType is the commit's type, lowercase, or empty if its subject
// isn't Conventional.
func commitType(c Commit) string {
	if m := conventional.FindStringSubmatch(c.Subject); m != nil {
		return strings.ToLower(m[1])
	}
	return ""
}

// breaking reports whether the commit is a breaking change: a `!` before
// its subject's colon, or a BREAKING CHANGE: footer.
func breaking(c Commit) bool {
	m := conventional.FindStringSubmatch(c.Subject)
	return (m != nil && m[3] == "!") || breakingInBody.MatchString(c.Body)
}

// NextVersion is the tag to make: v0.1.0 when there is no last tag, else
// the last one bumped by the commits. A breaking change bumps the major
// from 1.0 on; a breaking change or a feat bumps the minor (on 0.x, a
// breaking change is only a minor bump); anything else bumps the patch.
func NextVersion(last string, commits []Commit) (string, error) {
	if last == "" {
		return "v0.1.0", nil
	}
	m := versionTag.FindStringSubmatch(last)
	if m == nil {
		return "", fmt.Errorf("not a version tag: %s", last)
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	isBreaking := slices.ContainsFunc(commits, breaking)
	isFeat := slices.ContainsFunc(commits, func(c Commit) bool { return commitType(c) == "feat" })
	switch {
	case isBreaking && major > 0:
		return fmt.Sprintf("v%d.0.0", major+1), nil
	case isBreaking || isFeat:
		return fmt.Sprintf("v%d.%d.0", major, minor+1), nil
	}
	return fmt.Sprintf("v%d.%d.%d", major, minor, patch+1), nil
}

// ReleaseNotes is the notes of a release of the commits, oldest first:
// Breaking changes, Features, Fixes, a collapsed Internal for every other
// type, and the compare link unless it is the first release. A section
// with no commits is left out.
func ReleaseNotes(commits []Commit, span Span) string {
	var breakers, features, fixes, internal []Commit
	for _, c := range commits {
		switch {
		case breaking(c):
			breakers = append(breakers, c)
		case commitType(c) == "feat":
			features = append(features, c)
		case commitType(c) == "fix":
			fixes = append(fixes, c)
		default:
			internal = append(internal, c)
		}
	}
	list := func(cs []Commit) string {
		var b strings.Builder
		for _, c := range cs {
			b.WriteString(entry(c, span.Repo))
		}
		return b.String()
	}
	var notes []string
	for _, s := range []struct {
		head    string
		commits []Commit
	}{{"### Breaking changes", breakers}, {"### Features", features}, {"### Fixes", fixes}} {
		if len(s.commits) > 0 {
			notes = append(notes, s.head+"\n\n"+list(s.commits))
		}
	}
	if len(internal) > 0 {
		notes = append(notes, "<details>\n<summary>Internal</summary>\n\n"+list(internal)+"\n</details>\n")
	}
	if span.From != "" {
		notes = append(notes, fmt.Sprintf("**Full diff**: https://github.com/%s/compare/%s...%s\n", span.Repo, span.From, span.To))
	}
	return strings.Join(notes, "\n")
}

// entry is a commit's line: its subject without the type, the scope bold,
// then the issues it refers to and its SHA; a breaking change's footer
// sits indented beneath it.
func entry(c Commit, repo string) string {
	summary := c.Subject
	if m := conventional.FindStringSubmatch(c.Subject); m != nil {
		summary = m[4]
		if m[2] != "" {
			summary = "**" + m[2] + "**: " + summary
		}
	}
	var refs []string
	for _, n := range referred(c.Body, repo) {
		refs = append(refs, "#"+strconv.Itoa(n))
	}
	refs = append(refs, c.SHA)
	out := fmt.Sprintf("- %s (%s)\n", summary, strings.Join(refs, ", "))
	if footer := breakingFooter(c.Body); footer != "" {
		out += "\n" + firstChar.ReplaceAllString(footer, "  $1") + "\n"
	}
	return out
}

// referred is the issues of repo that a body's closing and `Refs` lines
// name, lowest first, each once: `#12` at the start of a line or after a
// space or a comma, and `owner/name#12` anywhere but after a word
// character, `.` or `-`, or a `/` that follows one, so not another repo's,
// as in `hyprsh/nixos-config#2`.
func referred(body, repo string) []int {
	qualified := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(repo) + `#(\d+)\b`)
	var found []int
	add := func(digits string) {
		n, _ := strconv.Atoi(digits)
		found = append(found, n)
	}
	for _, line := range strings.Split(body, "\n") {
		if !referring.MatchString(line) {
			continue
		}
		for _, m := range bareIssue.FindAllStringSubmatch(line, -1) {
			add(m[1])
		}
		for _, m := range qualified.FindAllStringSubmatchIndex(line, -1) {
			if !afterWord(line, m[0]) {
				add(line[m[2]:m[3]])
			}
		}
	}
	slices.Sort(found)
	return slices.Compact(found)
}

// afterWord reports whether line has a word character, `.` or `-` before
// index i, or a `/` that follows one.
func afterWord(line string, i int) bool {
	isWord := func(b byte) bool {
		return b == '.' || b == '-' || b == '_' || ('0' <= b && b <= '9') || ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z')
	}
	if i >= 1 && isWord(line[i-1]) {
		return true
	}
	return i >= 2 && line[i-1] == '/' && isWord(line[i-2])
}

// breakingFooter is what the body's BREAKING CHANGE: footers say, each up
// to the line that ends it; empty if it has none.
func breakingFooter(body string) string {
	var footers []string
	open := false
	for _, line := range strings.Split(body, "\n") {
		start := breakingFooterStart.FindStringSubmatch(line)
		open = start != nil || (open && !footerEnd.MatchString(line))
		if start != nil {
			footers = append(footers, start[1])
		} else if open {
			footers[len(footers)-1] += "\n" + line
		}
	}
	var kept []string
	for _, f := range footers {
		if f = strings.TrimSpace(f); f != "" {
			kept = append(kept, f)
		}
	}
	return strings.Join(kept, "\n\n")
}

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// git runs git in dir, isolated from the machine's own configuration.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// A checkout of a local bare remote, at a detached HEAD on main, where
// releases are cut from.
func newCheckout(t *testing.T) (dir, remote string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	base := t.TempDir()
	remote = filepath.Join(base, "remote.git")
	dir = filepath.Join(base, "work")
	git(t, base, "init", "-q", "--bare", "-b", "main", remote)
	git(t, base, "init", "-q", "-b", "main", dir)
	git(t, dir, "config", "user.name", "Test")
	git(t, dir, "config", "user.email", "test@example.com")
	git(t, dir, "remote", "add", "origin", remote)
	commit(t, dir, "feat(api): first", "Closes #1")
	git(t, dir, "push", "-q", "origin", "main")
	git(t, dir, "checkout", "-q", "--detach", "origin/main")
	return dir, remote
}

func commit(t *testing.T, dir, subject, body string) {
	t.Helper()
	git(t, dir, "commit", "-q", "--allow-empty", "-m", subject, "-m", body)
}

// A stand-in for gh that records its calls, in order.
type fakeGH struct{ calls [][]string }

func (g *fakeGH) run(args ...string) (string, error) {
	g.calls = append(g.calls, args)
	if args[0] == "repo" {
		return "hyprsh/shrt\n", nil
	}
	return "", nil
}

func TestReleaseTagsHeadPushesTheTagAndMakesTheRelease(t *testing.T) {
	dir, remote := newCheckout(t)
	gh := &fakeGH{}
	var out bytes.Buffer
	if err := (Releaser{Dir: dir, GH: gh.run, Out: &out}).Run(); err != nil {
		t.Fatal(err)
	}
	if got := git(t, remote, "tag", "--list"); got != "v0.1.0" {
		t.Errorf("remote tags %q, want v0.1.0", got)
	}
	if got, want := git(t, dir, "rev-parse", "v0.1.0"), git(t, dir, "rev-parse", "HEAD"); got != want {
		t.Errorf("v0.1.0 at %s, HEAD at %s", got, want)
	}
	if len(gh.calls) != 2 || gh.calls[0][0] != "repo" {
		t.Fatalf("gh calls %v, want repo view then release create", gh.calls)
	}
	create := gh.calls[1]
	if strings.Join(create[:5], " ") != "release create v0.1.0 --title v0.1.0" || create[5] != "--notes" {
		t.Errorf("gh release create args %v", create)
	}
	if notes := create[6]; !strings.Contains(notes, "- **api**: first (#1, ") || strings.Contains(notes, "Full diff") {
		t.Errorf("notes %q", notes)
	}
}

func TestSecondReleaseBumpsFromTheLastTag(t *testing.T) {
	dir, _ := newCheckout(t)
	if err := (Releaser{Dir: dir, GH: (&fakeGH{}).run, Out: &bytes.Buffer{}}).Run(); err != nil {
		t.Fatal(err)
	}
	commit(t, dir, "fix(store): mend", "Refs #2")
	gh := &fakeGH{}
	if err := (Releaser{Dir: dir, GH: gh.run, Out: &bytes.Buffer{}}).Run(); err != nil {
		t.Fatal(err)
	}
	create := gh.calls[len(gh.calls)-1]
	if create[2] != "v0.1.1" || !strings.Contains(create[6], "compare/v0.1.0...v0.1.1") || strings.Contains(create[6], "first") {
		t.Errorf("gh release create args %v", create)
	}
}

func TestRejectedPushLeavesNoTag(t *testing.T) {
	dir, remote := newCheckout(t)
	hook := filepath.Join(remote, "hooks", "pre-receive")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	gh := &fakeGH{}
	err := Releaser{Dir: dir, GH: gh.run, Out: &bytes.Buffer{}}.Run()
	if err == nil {
		t.Fatal("a rejected push should be an error")
	}
	if got := git(t, dir, "tag", "--points-at", "HEAD"); got != "" {
		t.Errorf("tags at HEAD %q, want none", got)
	}
	if got := git(t, remote, "tag", "--list"); got != "" {
		t.Errorf("remote tags %q, want none", got)
	}
	for _, c := range gh.calls {
		if c[0] == "release" {
			t.Errorf("made a release after a rejected push: %v", c)
		}
	}
}

func TestNothingSinceTheLastTagDoesNothing(t *testing.T) {
	dir, remote := newCheckout(t)
	if err := (Releaser{Dir: dir, GH: (&fakeGH{}).run, Out: &bytes.Buffer{}}).Run(); err != nil {
		t.Fatal(err)
	}
	gh := &fakeGH{}
	var out bytes.Buffer
	if err := (Releaser{Dir: dir, GH: gh.run, Out: &out}).Run(); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "release: nothing since v0.1.0\n" {
		t.Errorf("output %q", got)
	}
	if len(gh.calls) != 0 {
		t.Errorf("gh called: %v", gh.calls)
	}
	if got := git(t, remote, "tag", "--list"); got != "v0.1.0" {
		t.Errorf("remote tags %q, want just v0.1.0", got)
	}
}

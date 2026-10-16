package main

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Releaser releases the checkout in Dir: tags HEAD, pushes the tag and
// makes the GitHub release. GH runs gh with its arguments and returns its
// output, so tests can stand in for it.
type Releaser struct {
	Dir string
	GH  func(args ...string) (string, error)
	Out io.Writer
}

// run runs the command in dir; its output. A failure names the command,
// with long arguments such as release notes cut short, and carries what
// it wrote to stderr.
func run(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		named := []string{name}
		for _, arg := range args {
			arg, _, _ = strings.Cut(arg, "\n")
			if len(arg) > 40 {
				arg = arg[:40] + "..."
			}
			named = append(named, arg)
		}
		return "", fmt.Errorf("%s: %w: %s", strings.Join(named, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// git runs git in the checkout; its output.
func (r Releaser) git(args ...string) (string, error) {
	return run(r.Dir, "git", args...)
}

// lastTag is the latest v* tag at or before HEAD, empty if there is none.
func (r Releaser) lastTag() string {
	out, err := r.git("describe", "--tags", "--abbrev=0", "--match", "v*", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// since is the commits since tag, or all of them before the first
// release, oldest first.
func (r Releaser) since(tag string) ([]Commit, error) {
	rev := "HEAD"
	if tag != "" {
		rev = tag + "..HEAD"
	}
	out, err := r.git("log", "--reverse", "--format=%h%x00%s%x00%b%x1e", rev)
	if err != nil {
		return nil, err
	}
	var commits []Commit
	for _, chunk := range strings.Split(out, "\x1e") {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		parts := strings.SplitN(chunk, "\x00", 3)
		for len(parts) < 3 {
			parts = append(parts, "")
		}
		commits = append(commits, Commit{SHA: parts[0], Subject: parts[1], Body: parts[2]})
	}
	return commits, nil
}

// Run cuts the release. A push that's rejected leaves no tag at HEAD.
func (r Releaser) Run() error {
	last := r.lastTag()
	commits, err := r.since(last)
	if err != nil {
		return err
	}
	if last != "" && len(commits) == 0 {
		fmt.Fprintf(r.Out, "release: nothing since %s\n", last)
		return nil
	}
	// The repo on GitHub, for the compare link, asked before anything is pushed.
	repo, err := r.GH("repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
	if err != nil {
		return fmt.Errorf("gh can't tell this checkout's repo: %w", err)
	}
	version, err := NextVersion(last, commits)
	if err != nil {
		return err
	}
	notes := ReleaseNotes(commits, Span{Repo: strings.TrimSpace(repo), From: last, To: version})
	if _, err := r.git("tag", version); err != nil {
		return err
	}
	if _, err := r.git("push", "-q", "origin", "refs/tags/"+version); err != nil {
		if _, delErr := r.git("tag", "-d", version); delErr != nil {
			return fmt.Errorf("couldn't push %s: %w (and couldn't delete it: %v)", version, err, delErr)
		}
		return fmt.Errorf("couldn't push %s: %w", version, err)
	}
	if _, err := r.GH("release", "create", version, "--title", version, "--notes", notes); err != nil {
		return fmt.Errorf("%s is pushed but its release isn't made: %w", version, err)
	}
	fmt.Fprintf(r.Out, "release: %s\n", version)
	return nil
}

// Command release cuts a release of shrt, as Autopilot's Release section
// says: it works the version out from the Conventional Commits since the
// last v* tag, tags HEAD, pushes the tag and makes the GitHub release with
// notes in the format every hyprsh repo's releases share. Run it from a
// detached HEAD at origin/main. It writes no version file: the tag is the
// version.
package main

import (
	"fmt"
	"os"
)

// gh runs the GitHub CLI; its output.
func gh(args ...string) (string, error) {
	return run(".", "gh", args...)
}

func main() {
	if err := (Releaser{Dir: ".", GH: gh, Out: os.Stdout}).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "release: %v\n", err)
		os.Exit(1)
	}
}

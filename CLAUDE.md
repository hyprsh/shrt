# shrt

URL shortener for the hypr.sh homelab, served under https://hypr.sh/x/. The design lives in `docs/design.md`, the decisions in `docs/adr/`.

## Agent skills

### Issue tracker

Issues are tracked as GitHub issues on `hyprsh/shrt`, via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

The five default labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.

## Check, commits and releases

Check with `scripts/check.sh`, which runs `test -z "$(gofmt -l .)"`, `go vet ./...` and `go test ./...`; CI and the release workflow run it too.

Commits are Conventional, `type(scope)!: subject`, and the scope is required: one of `api`, `redirect`, `store`, `config`, `codes`, `image` (the Dockerfile and `shrt health`), `ci` (the workflows), `docs`, `deps` or `release`, since the release notes are built from them. A breaking change gets `!` and a `BREAKING CHANGE:` footer saying what upgrading needs. A ticket's last commit has `Closes #N` in its footer, and each of its commits before that `Refs #N`.

Release with `go run ./cmd/release` from a detached HEAD at `origin/main`. It tags the next `vX.Y.Z` (on 0.x a `feat` or a breaking change bumps the minor, anything else the patch; the first is `v0.1.0`), pushes the tag and makes the GitHub release. The tag is the version: there is no version file. Releasing deploys nothing here: the tag makes `release.yml` publish the image, and Renovate in hyprsh/cloud picks it up. Deploys happen in hyprsh/cloud.

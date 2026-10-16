# Autopilot

How `/hyprsh-skills:autopilot` works this repo's `ready-for-agent` tickets: how it prepares, checks and releases the repo, and what its ticket sessions follow. Under Install, Check and Release, the runner reads the first code block as the command, one line or several; a Triage heading turns triage prep on, and a line under it promotion; everything else is for people and agents. `/hyprsh-skills:setup` wrote this file; edit it freely.

## Install

Runs in every fresh ticket worktree, and again after a rebase.

```sh
go mod download
```

## Check

A ticket lands only when this is green. Autopilot runs one check at a time, its own and its sessions', which run it through Autopilot.

```sh
test -z "$(gofmt -l .)"
go vet ./...
go test ./...
```

## Release

```sh
go run ./cmd/release
```

SemVer on 0.x: a `feat` or a breaking change bumps the minor, anything else the patch; the first release is `v0.1.0`. Autopilot releases after a Spec's last ticket lands, and at the end of a run, from a detached `origin/main`. The command tags HEAD, pushes the tag and makes the GitHub release, with notes in the format every hyprsh repo's releases share, built from the Conventional Commits since the last `v*` tag. A push that's rejected leaves no tag. There is no version file: the tag is the version. Deploys stay in nixos-config.

## Commits

Conventional Commits: `type(scope)!: subject`, with the scope required, one of `api`, `redirect`, `store`, `config`, `codes`, `nix`, `docs`, `deps`, or `release` for the release command, and the type `feat`, `fix`, `docs`, `refactor`, `test`, `chore` and so on; the release notes are built from these; a breaking change gets `!` and a `BREAKING CHANGE:` footer saying what upgrading needs. A ticket's last commit has `Closes #N` in its footer, and each of its commits before that `Refs #N`.

## Tiers

A ticket's model label sets the model and effort its session runs at. A ticket without one runs at the default tier below; one with several, at the highest. `At once` caps how many of a tier's sessions a run holds at once, empty for no cap. Give every ticket you file one of these:

| Label | Model | Effort | At once | For |
| --- | --- | --- | --- | --- |
| `opus-xhigh` | opus | xhigh | 1 | hard or wide-reaching tickets |
| `opus-high` | opus | high | | most tickets |
| `sonnet-high` | sonnet | high | | mechanical tickets: renames, docs, small fixes |

Default tier: `opus-high`

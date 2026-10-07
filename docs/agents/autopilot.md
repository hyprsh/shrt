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

## Commits

Conventional Commits: `type(scope)!: subject`, with `feat`, `fix`, `docs`, `refactor`, `test`, `chore` and so on; a breaking change gets `!` and a `BREAKING CHANGE:` footer saying what upgrading needs. A ticket's last commit has `Closes #N` in its footer, and each of its commits before that `Refs #N`.

## Tiers

A ticket's model label sets the model and effort its session runs at. A ticket without one runs at the default tier below; one with several, at the highest. `At once` caps how many of a tier's sessions a run holds at once, empty for no cap. Give every ticket you file one of these:

| Label | Model | Effort | At once | For |
| --- | --- | --- | --- | --- |
| `opus-xhigh` | opus | xhigh | 1 | hard or wide-reaching tickets |
| `opus-high` | opus | high | | most tickets |
| `sonnet-high` | sonnet | high | | mechanical tickets: renames, docs, small fixes |

Default tier: `opus-high`

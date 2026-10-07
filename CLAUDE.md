# shrt

URL shortener for the hypr.sh homelab, served under https://hypr.sh/x/. The design lives in `docs/design.md`, the decisions in `docs/adr/`.

## Agent skills

### Issue tracker

Issues are tracked as GitHub issues on `hyprsh/shrt`, via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

The five default labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.

### Autopilot

Install with `go mod download`, check with gofmt, `go vet` and `go test`; Autopilot doesn't release (deploys happen in nixos-config). Every ticket filed for Autopilot gets exactly one tier label, which sets the model and effort its session runs at: `opus-xhigh` for hard or wide-reaching tickets, `opus-high` for most, `sonnet-high` for mechanical ones (renames, docs, small fixes). After filing `ready-for-agent` tickets, the next step is `/hyprsh-skills:autopilot`. See `docs/agents/autopilot.md`.

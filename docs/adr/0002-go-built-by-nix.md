# 2. Go, built and run by Nix

Date: 2026-10-07
Status: accepted

## Context

shrt is a small HTTP service on the NixOS host `server`. Two stacks were
weighed: Go, built from source by Nix like the Go tools in nixos-config's
`health.nix`, or TypeScript on Bun like danshari, which ships as a container
image to GHCR through GitHub Actions and is digest-pinned in
`media-images.json`. Andy left the choice to the agent.

## Decision

shrt is written in Go, using the standard library for HTTP and
`modernc.org/sqlite` (pure Go, no cgo) for storage if SQLite is chosen.
nixos-config takes `hyprsh/shrt` as a flake input, builds it with
`buildGoModule`, and runs it as a native systemd service.

## Consequences

- No container image, registry or publish workflow. A deploy is a flake input
  update plus `./scripts/rebuild.sh server`.
- One static binary with few dependencies.
- Install is `go mod download`; Check is gofmt, `go vet` and `go test`.
- Differs from danshari's Bun stack, so code isn't shared with it or with navi.

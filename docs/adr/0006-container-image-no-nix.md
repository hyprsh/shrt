# 6. A container image on GHCR, run as a Compose Stack, no Nix

Date: 2026-10-17
Status: accepted

## Context

ADR 0002 had Nix build shrt as a flake input of nixos-config and run it as a
native systemd service on `server`, with no container image. The homelab is
leaving NixOS: hosts are rebuilt on Debian with Docker (hyprsh/cloud ADR
0001), and `server` is wiped. shrt runs on `cloud` as a Compose Stack of
hyprsh/cloud. It is pinned there by tag and digest, updated through
Renovate, and deployed by `hyprsh apply`. tnl already ships its images this
way.

## Decision

Supersedes [ADR 0002](0002-go-built-by-nix.md), except that shrt stays Go,
with `modernc.org/sqlite` and no cgo.

- **The image:** `ghcr.io/hyprsh/shrt`, built by a multi-stage Dockerfile.
  - A `golang` stage on `$BUILDPLATFORM` cross-compiles for each target, so no
    QEMU is needed.
  - The final image is `gcr.io/distroless/static-debian12:nonroot` (uid
    65532), the same as tnl.
  - It is built for linux/amd64 and linux/arm64.
  - GoReleaser isn't used: `cmd/release` already makes the GitHub release, and
    shrt ships no binary archives.
- **Tags:** a `vX.Y.Z` tag, and only a plain one, publishes `X.Y.Z`, `X.Y`, `X`
  and `latest`. The image carries `org.opencontainers.image.source` for
  Renovate, and GitHub artifact attestations record its build provenance.
- **Running in a container:**
  - The image sets `SHRT_ADDRESS=0.0.0.0` and
    `SHRT_DB_PATH=/var/lib/shrt/shrt.db`, with that directory owned by 65532.
  - It has no `HEALTHCHECK` and no `VOLUME`. Compose runs `shrt health`.
  - The binary knows its version from the tag.
- **The token:** it comes from `SHRT_TOKEN_FILE` (Compose `secrets:`) or
  `SHRT_TOKEN`, never both, following tnl's convention.
- **CI:**
  - `ci.yml` runs the Check and builds the image without pushing, on every
    push to `main` and every PR.
  - `release.yml` runs the Check, then builds, pushes and attests the image,
    on `vX.Y.Z` tags.
  - CI deploys nothing.
- **Nix:** `flake.nix` and `flake.lock` are deleted.

## Consequences

- A deploy goes: release, then a Renovate PR in hyprsh/cloud, then `hyprsh
  apply`. It is no longer a flake input update.
- Autopilot's Check on the Mac never builds the image. `ci.yml` catches a
  broken Dockerfile before a release does.
- The host directory bind-mounted at `/var/lib/shrt`
  (`/srv/state/shrt/shrt` on `cloud`) must be owned by uid 65532.
- nixos-config's lock still points at a commit with `flake.nix`. `nix flake
  update` there fails until the input is removed, which doesn't matter once
  `server` is wiped.

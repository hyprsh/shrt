# shrt design

Status: agreed with Andy on 2026-10-07.

## Goal

A URL shortener for the hypr.sh homelab. Short links are served at
`https://sh.hypr.sh/<code>` and redirect to the original URL. navi, the
household AI assistant, creates them programmatically when Andy asks it to
shorten a URL.

## Deployment

Runs on `cloud` as the `shrt` Stack of hyprsh/cloud: a Compose service from the
image `ghcr.io/hyprsh/shrt` behind tnl's Edge, which carries both public and
internal traffic. hyprsh/cloud pins the image by tag and digest and updates it
through Renovate; `hyprsh apply` deploys.

## Decisions

| # | Decision | ADR |
|---|----------|-----|
| 1 | The only way to create links is an HTTP API that navi calls with a token. No web UI, no CLI. | [0001](adr/0001-api-only-interface.md) |
| 2 | Short links are shared with anyone, so the redirect `https://sh.hypr.sh/<code>` is reachable from the internet. | [0005](adr/0005-links-at-the-root-public-api.md) |
| 3 | Go, shipped as a container image on GHCR (`ghcr.io/hyprsh/shrt`, distroless, linux/amd64 and linux/arm64) and run as a Compose service on `cloud`. No Nix. | [0006](adr/0006-container-image-no-nix.md) |
| 4 | The API is public at `sh.hypr.sh/api/`, guarded only by the bearer token. | [0005](adr/0005-links-at-the-root-public-api.md) |
| 5 | Codes are 6 random base62 characters by default; navi may pass a chosen name (`a-z 0-9 -`) instead, and a taken name is an error. | [0004](adr/0004-random-codes-optional-names.md) |
| 6 | Links live in one SQLite file (`modernc.org/sqlite`, no cgo) at `/var/lib/shrt/shrt.db` in the image, bind-mounted from `/srv/state/shrt/shrt` on `cloud` and owned by uid 65532. | [0006](adr/0006-container-image-no-nix.md) |
| 7 | shrt does no backups of its own. Losing `/persist` loses the links, which is accepted. Snapshots or backups of all of `/persist` belong in nixos-config, as separate work. | |
| 8 | One API token: 32 random bytes in hyprsh/cloud's secrets, read by shrt as a file through Compose `secrets:` (`SHRT_TOKEN_FILE`; `SHRT_TOKEN` also works, never both), held by navi as `SHRT_TOKEN` and sent as `Authorization: Bearer`. shrt compares it in constant time and refuses every API call when no token is configured. Rotation means updating both places. | |
| 9 | Links live forever, until navi deletes them. No expiry. | |
| 10 | Shortening a URL that already has a generated code returns that code; a chosen name always makes its own link. | |
| 11 | Only `http://` and `https://` URLs of at most 2048 characters; a URL anywhere on `sh.hypr.sh`, shrt's own host, is rejected. | [0005](adr/0005-links-at-the-root-public-api.md) |
| 12 | The redirect is a `302`, so a deleted link stops redirecting even in browsers that opened it before. Unknown and deleted codes get a plain `404`. | |
| 13 | A deleted chosen name can be used again. | |
| 14 | No statistics: no click counter, no record of who opened a link beyond nginx's access log. | |
| 15 | shrt ships only the image. How it runs on `cloud` lives in hyprsh/cloud as the `shrt` Stack (#39 there): the Compose service (healthcheck `shrt health`), the secret, the state directory and the Edge routes for the public and internal hostnames. | [0006](adr/0006-container-image-no-nix.md) |
| 16 | Deploys happen in hyprsh/cloud, not here: tagging a release (`go run ./cmd/release`) makes `release.yml` publish the image, Renovate in hyprsh/cloud opens a PR pinning it, and `hyprsh apply` deploys. CI deploys nothing. | [0006](adr/0006-container-image-no-nix.md) |
| 17 | shrt owns the whole host `sh.hypr.sh`: `GET /{code}` redirects, `/api/` and `/healthz` are shrt's own paths, and the names `api` and `healthz` can't be chosen. `GET /` and unknown codes get a plain `404`. | [0005](adr/0005-links-at-the-root-public-api.md) |
| 18 | The API, redirect and health endpoints are as described under [API](#api). | |
| 19 | navi learns shrt through a tracked skill `skills/shrt/` in hyprsh/navi (`SKILL.md` plus a script for create, list and delete, calling `https://shrt.internal.hypr.sh` with `$NAVI_SECRET_SHRT_TOKEN`). It is a `ready-for-agent` ticket in hyprsh/navi, worked by Autopilot there, blocked until shrt runs. | |
| 20 | The navi skill is for everyone in the household (no `navi-roles`), not Owner-only. | |
| 21 | The Check on the Mac covers only the Go code and never builds the image. `ci.yml` runs the Check and builds the image for both platforms on every PR and push to `main`, so a broken Dockerfile shows up before a release. | [0006](adr/0006-container-image-no-nix.md) |

## API

Served publicly at `https://sh.hypr.sh/api/` (decision 4). Every call needs
`Authorization: Bearer <token>`; a missing or wrong token gets `401`. JSON in
and out; errors are `{"error": "..."}`.

| Call | Does | Answers |
| --- | --- | --- |
| `POST /api/links` with `{"url": "...", "name": "ferien"}` (`name` optional) | Creates a link; without `name`, a random code | `201` created, `200` existing generated code for the same URL, `400` invalid URL or name, `409` name taken |
| `GET /api/links` | Every link, newest first | `200` |
| `GET /api/links/{code}` | One link | `200`, `404` |
| `DELETE /api/links/{code}` | Deletes a link | `204`, `404` |

A link is:

```json
{"code": "k3P9qa", "short_url": "https://sh.hypr.sh/k3P9qa", "url": "https://example.com/...", "created_at": "2026-10-07T08:00:00Z"}
```

shrt also serves, without a token:

- `GET /{code}`: `302` to the link's URL, `404` if unknown.
- `GET /`: a plain `404`.
- `GET /healthz`: `200` when the database is reachable, for monitoring.

The listen address, port, base URL for `short_url`, the path of the token
file and the database path come from environment variables, set by
nixos-config; the README lists them.

## Out of scope

- Backups of `/persist` on `server`. Today it is a ZFS mirror (`data/persist`) with no snapshots or off-host backup in nixos-config. That covers a failed disk, not a deletion or a lost pool. To be solved for every service in nixos-config (e.g. sanoid snapshots), tracked in [hyprsh/nixos-config#1](https://github.com/hyprsh/nixos-config/issues/1).


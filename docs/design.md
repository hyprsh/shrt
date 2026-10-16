# shrt design

Status: agreed with Andy on 2026-10-07.

## Goal

A URL shortener for the hypr.sh homelab. Short links are served at
`https://sh.hypr.sh/<code>` and redirect to the original URL. navi, the
household AI assistant, creates them programmatically when Andy asks it to
shorten a URL.

## Deployment

Runs on the NixOS host `server` (`server.home`), configured declaratively in
`~/Code/nixos-config`. Public traffic reaches it through the nginx reverse
proxy in `modules/services/proxy-external.nix`.

## Decisions

| # | Decision | ADR |
|---|----------|-----|
| 1 | The only way to create links is an HTTP API that navi calls with a token. No web UI, no CLI. | [0001](adr/0001-api-only-interface.md) |
| 2 | Short links are shared with anyone, so the redirect `https://sh.hypr.sh/<code>` is reachable from the internet. | [0005](adr/0005-links-at-the-root-public-api.md) |
| 3 | Go, built by Nix (`buildGoModule`) as a flake input of nixos-config, run as a native systemd service. | [0002](adr/0002-go-built-by-nix.md) |
| 4 | The API is public at `sh.hypr.sh/api/`, guarded only by the bearer token. | [0005](adr/0005-links-at-the-root-public-api.md) |
| 5 | Codes are 6 random base62 characters by default; navi may pass a chosen name (`a-z 0-9 -`) instead, and a taken name is an error. | [0004](adr/0004-random-codes-optional-names.md) |
| 6 | Links live in one SQLite file (`modernc.org/sqlite`, no cgo) in the service's state directory on `server`, persisted under `/persist`. | [0002](adr/0002-go-built-by-nix.md) |
| 7 | shrt does no backups of its own. Losing `/persist` loses the links, which is accepted. Snapshots or backups of all of `/persist` belong in nixos-config, as separate work. | |
| 8 | One API token: 32 random bytes in sops (`secrets/server.yaml`, `shrt/api-token`), read by shrt via systemd `LoadCredential`, held by navi as `SHRT_TOKEN` and sent as `Authorization: Bearer`. shrt compares it in constant time and refuses every API call when no token is configured. Rotation means updating both places. | |
| 9 | Links live forever, until navi deletes them. No expiry. | |
| 10 | Shortening a URL that already has a generated code returns that code; a chosen name always makes its own link. | |
| 11 | Only `http://` and `https://` URLs of at most 2048 characters; a URL anywhere on `sh.hypr.sh`, shrt's own host, is rejected. | [0005](adr/0005-links-at-the-root-public-api.md) |
| 12 | The redirect is a `302`, so a deleted link stops redirecting even in browsers that opened it before. Unknown and deleted codes get a plain `404`. | |
| 13 | A deleted chosen name can be used again. | |
| 14 | No statistics: no click counter, no record of who opened a link beyond nginx's access log. | |
| 15 | shrt's `flake.nix` exports only the package (`packages.default`, `buildGoModule`). How it runs on `server` lives in nixos-config as `modules/services/shrt.nix`: the systemd unit (`DynamicUser`, `StateDirectory`, `LoadCredential`), the public `hypr.sh` vhost forwarding only `GET /x/`, the internal `shrt.internal.hypr.sh` vhost, the sops secret and persistence. The nixos-config side is a ticket in nixos-config that an agent may work too. | [0002](adr/0002-go-built-by-nix.md) |
| 16 | The nixos-config ticket is worked in an attended session, not by Autopilot: build with `./scripts/rebuild.sh server flake-check` / `build`, show the diff, push to `main` only on Andy's go. comin deploys `main` to `server` within a minute, so the push is the deploy. | |
| 17 | shrt owns the whole host `sh.hypr.sh`: `GET /{code}` redirects, `/api/` and `/healthz` are shrt's own paths, and the names `api` and `healthz` can't be chosen. `GET /` and unknown codes get a plain `404`. | [0005](adr/0005-links-at-the-root-public-api.md) |
| 18 | The API, redirect and health endpoints are as described under [API](#api). | |
| 19 | navi learns shrt through a tracked skill `skills/shrt/` in hyprsh/navi (`SKILL.md` plus a script for create, list and delete, calling `https://shrt.internal.hypr.sh` with `$NAVI_SECRET_SHRT_TOKEN`). It is a `ready-for-agent` ticket in hyprsh/navi, worked by Autopilot there, blocked until shrt runs. | |
| 20 | The navi skill is for everyone in the household (no `navi-roles`), not Owner-only. | |
| 21 | shrt's `flake.nix` is written and verified in the attended nixos-config work (decision 16), with `./scripts/rebuild.sh server build`. Autopilot's Check in shrt covers only the Go code, since the Mac it runs on has no Nix. A dependency update that breaks `vendorHash` shows up at the next nixos-config build. | |

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


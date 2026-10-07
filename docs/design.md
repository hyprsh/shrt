# shrt design

Status: in progress (being worked out with Andy, one decision at a time).

## Goal

A URL shortener for the hypr.sh homelab. Short links are served at
`https://hypr.sh/x/<code>` and redirect to the original URL. navi, the
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
| 2 | Short links are shared with anyone, so the redirect `https://hypr.sh/x/<code>` is reachable from the internet. | [0003](adr/0003-public-redirect-lan-only-api.md) |
| 3 | Go, built by Nix (`buildGoModule`) as a flake input of nixos-config, run as a native systemd service. | [0002](adr/0002-go-built-by-nix.md) |
| 4 | The API is LAN-only at `shrt.internal.hypr.sh` (`proxyInternal`) plus a bearer token. The public `hypr.sh` vhost forwards only `GET /x/<code>`. | [0003](adr/0003-public-redirect-lan-only-api.md) |
| 5 | Codes are 6 random base62 characters by default; navi may pass a chosen name (`a-z 0-9 -`) instead, and a taken name is an error. | [0004](adr/0004-random-codes-optional-names.md) |
| 6 | Links live in one SQLite file (`modernc.org/sqlite`, no cgo) in the service's state directory on `server`, persisted under `/persist`. | [0002](adr/0002-go-built-by-nix.md) |
| 7 | shrt does no backups of its own. Losing `/persist` loses the links, which is accepted. Snapshots or backups of all of `/persist` belong in nixos-config, as separate work. | |
| 8 | One API token: 32 random bytes in sops (`secrets/server.yaml`, `shrt/api-token`), read by shrt via systemd `LoadCredential`, held by navi as `SHRT_TOKEN` and sent as `Authorization: Bearer`. shrt compares it in constant time and refuses every API call when no token is configured. Rotation means updating both places. | |
| 9 | Links live forever, until navi deletes them. No expiry. | |
| 10 | Shortening a URL that already has a generated code returns that code; a chosen name always makes its own link. | |
| 11 | Only `http://` and `https://` URLs of at most 2048 characters; URLs pointing back to `hypr.sh/x/` are rejected. | |
| 12 | The redirect is a `302`, so a deleted link stops redirecting even in browsers that opened it before. Unknown and deleted codes get a plain `404`. | |
| 13 | A deleted chosen name can be used again. | |
| 14 | No statistics: no click counter, no record of who opened a link beyond nginx's access log. | |
| 15 | shrt's `flake.nix` exports only the package (`packages.default`, `buildGoModule`). How it runs on `server` lives in nixos-config as `modules/services/shrt.nix`: the systemd unit (`DynamicUser`, `StateDirectory`, `LoadCredential`), the public `hypr.sh` vhost forwarding only `GET /x/`, the internal `shrt.internal.hypr.sh` vhost, the sops secret and persistence. The nixos-config side is a ticket in nixos-config that an agent may work too. | [0002](adr/0002-go-built-by-nix.md) |
| 16 | The nixos-config ticket is worked in an attended session, not by Autopilot: build with `./scripts/rebuild.sh server flake-check` / `build`, show the diff, push to `main` only on Andy's go. comin deploys `main` to `server` within a minute, so the push is the deploy. | |
| 17 | On the public `hypr.sh` vhost only `GET /x/<code>` reaches shrt; nginx answers every other path, `/` and `/x/` included, with `404`, and redirects `http://` to `https://`. | [0003](adr/0003-public-redirect-lan-only-api.md) |

## Out of scope

- Backups of `/persist` on `server`. Today it is a ZFS mirror (`data/persist`) with no snapshots or off-host backup in nixos-config. That covers a failed disk, not a deletion or a lost pool. To be solved for every service in nixos-config (e.g. sanoid snapshots), tracked in [hyprsh/nixos-config#1](https://github.com/hyprsh/nixos-config/issues/1).

## Open questions


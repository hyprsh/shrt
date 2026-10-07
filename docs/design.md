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

## Out of scope

- Backups of `/persist` on `server`. Today it is a ZFS mirror (`data/persist`) with no snapshots or off-host backup in nixos-config. That covers a failed disk, not a deletion or a lost pool. To be solved for every service in nixos-config (e.g. sanoid snapshots).

## Open questions

- What `hypr.sh/` itself serves (today: nothing, catch-all 404)
- Link lifetime, duplicates, URL validation

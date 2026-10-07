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

## Open questions

- Code format (generated vs. chosen, length, alphabet)
- Storage, auth token handling
- What `hypr.sh/` itself serves (today: nothing, catch-all 404)
- Link lifetime, duplicates, URL validation

# 3. Public redirect, LAN-only API

Date: 2026-10-07
Status: accepted

## Context

Short links are shared with anyone, so the redirect must be reachable from
the internet. Only navi creates and manages links, from the Mac mini on the
LAN. A token that leaked would let anyone create links under `hypr.sh`,
for spam or phishing, if the API were public too.

## Decision

- The public vhost `hypr.sh` (via `proxyExternal`, which has no vhost for the
  bare domain yet) forwards only `GET /x/<code>` to shrt. Every other path
  returns 404, as cliproxyapi's public vhost does.
- The API is reachable only at `shrt.internal.hypr.sh` via `proxyInternal`,
  which allows only private networks, and it also requires a bearer token.

## Consequences

- The public attack surface is one GET route.
- navi can create links only from the LAN (or over Tailscale). The mini is
  stationary, so that is no loss.
- shrt must not rely on nginx alone: the API checks the token itself.

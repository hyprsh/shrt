# 5. Links at the root of sh.hypr.sh, API public behind its token

Date: 2026-10-17
Status: accepted

## Context

shrt moves off `server` and onto `cloud`, as the `shrt` Stack of
hyprsh/cloud (hyprsh/cloud#25). There, tnl's Edge is the only way in, and it
sends a whole hostname to one service. It can't route by path (hyprsh/cloud
ADR 0004). ADR 0003 depended on an nginx vhost that forwarded only
`GET /x/<code>` and on a LAN-only `shrt.internal.hypr.sh` for the API.
Neither exists on `cloud`.

## Decision

Supersedes [ADR 0003](0003-public-redirect-lan-only-api.md).

- shrt owns the host `sh.hypr.sh`. The redirect is `GET /{code}`, a link's
  `short_url` is `<base URL>/<code>`, and `SHRT_BASE_URL` defaults to
  `https://sh.hypr.sh`.
- The API is public at `sh.hypr.sh/api/`. shrt's own bearer token check is
  the only guard. `shrt.internal.hypr.sh` goes away.
- Chosen names that would shadow shrt's own paths (`api`, `healthz`) are
  refused. A URL anywhere on shrt's own host is rejected as a link to shrt
  itself.
- `GET /` and unknown codes get a plain `404`.

## Consequences

- The public attack surface is the whole API, not one GET route. A leaked
  token lets anyone create links under `sh.hypr.sh` until it is rotated.
- navi can create links from anywhere, not only the LAN. Its `shrt` skill has
  to move to `https://sh.hypr.sh` (hyprsh/navi).
- `/healthz` is public too. It reveals only whether the database is
  reachable.
- `hypr.sh/x/<code>` links stop working. Today's links are lost anyway, with
  `cloud`'s fresh state (hyprsh/cloud#8).
- Any new top-level path shrt serves must also become a refused name.

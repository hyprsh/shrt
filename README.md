# shrt

URL shortener for the hypr.sh homelab, served at https://sh.hypr.sh/<code>.
The design is in `docs/design.md`.

## Running

```sh
go run ./cmd/shrt
```

The image is `ghcr.io/hyprsh/shrt` (linux/amd64 and linux/arm64), tagged `X.Y.Z`,
`X.Y`, `X` and `latest`. It runs as uid 65532, so the directory bind-mounted at
`/var/lib/shrt` must be owned by 65532:

```sh
docker run -d -p 8080:8080 -e SHRT_TOKEN=... -v /srv/state/shrt/shrt:/var/lib/shrt ghcr.io/hyprsh/shrt
```

`shrt health` calls `/healthz` on the configured address and exits 0 on a `200`
and 1 otherwise, so a container health check can run it without a shell:
`test: ["CMD", "shrt", "health"]`. Deploys happen in hyprsh/cloud.

shrt is configured only through environment variables, and refuses to start on
any other `SHRT_*` variable, so a typo can't go unnoticed. The image sets
`SHRT_ADDRESS=0.0.0.0` and `SHRT_DB_PATH=/var/lib/shrt/shrt.db`.

| Variable | Default | Meaning |
| --- | --- | --- |
| `SHRT_ADDRESS` | `127.0.0.1` | Address to listen on |
| `SHRT_PORT` | `8080` | Port to listen on |
| `SHRT_BASE_URL` | `https://sh.hypr.sh` | Prefix of a link's `short_url`, before `/<code>` |
| `SHRT_TOKEN` | none | The API bearer token. Without a token every API call answers `401`. |
| `SHRT_TOKEN_FILE` | none | File holding the API bearer token, such as a Compose secret at `/run/secrets/shrt-token`. Give the token as this or `SHRT_TOKEN`, not both; a missing or empty file stops shrt. |
| `SHRT_DB_PATH` | `shrt.db` | SQLite database file, created if missing |

# shrt

URL shortener for the hypr.sh homelab, served under https://hypr.sh/x/.
The design is in `docs/design.md`.

## Running

```sh
go run ./cmd/shrt
```

`flake.nix` builds the package (`packages.default`) with `buildGoModule`.
nixos-config takes it as a flake input and runs it on `server`. When `go.mod`
or `go.sum` change, update `vendorHash` in `flake.nix`: the next build prints
the right hash.

shrt is configured only through environment variables:

| Variable | Default | Meaning |
| --- | --- | --- |
| `SHRT_ADDRESS` | `127.0.0.1` | Address to listen on |
| `SHRT_PORT` | `8080` | Port to listen on |
| `SHRT_BASE_URL` | `https://hypr.sh` | Prefix of a link's `short_url`, before `/x/<code>` |
| `SHRT_TOKEN_FILE` | none | File holding the API bearer token, such as `%d/api-token` from systemd's `LoadCredential`. Without a token every API call answers `401`. |
| `SHRT_DB_PATH` | `shrt.db` | SQLite database file, created if missing |

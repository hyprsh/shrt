#!/bin/sh
# The Check: what CI and the release workflow run; exits non-zero on the first
# failure. Run it from the repo root. Extra arguments go to `go test`, e.g.
# -count=1.
set -eu

unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
  echo "::error::not gofmt-formatted (run gofmt -w):" >&2
  echo "$unformatted" >&2
  exit 1
fi

go vet ./...
go test "$@" ./...
echo "ok: every check passed"

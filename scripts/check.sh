#!/bin/sh
# The Check: what CI and the release workflow run. Run it from the repo root.
set -eu

test -z "$(gofmt -l .)"
go vet ./...
go test ./...

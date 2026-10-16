# The Go stage runs on the build platform and cross-compiles for the target, so
# building for arm64 needs no QEMU. VERSION is the release tag without its v.
FROM --platform=$BUILDPLATFORM golang:1.26 AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o /out/shrt ./cmd/shrt

# The state directory is made on the build platform and owned by 65532, so a
# fresh volume inherits that ownership. There is no VOLUME, HEALTHCHECK or
# EXPOSE: Compose runs `shrt health`.
FROM --platform=$BUILDPLATFORM busybox:stable AS state
RUN mkdir -m 0700 /state

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/shrt /usr/local/bin/shrt
COPY --from=state --chown=65532:65532 --chmod=0700 /state /var/lib/shrt
ENV SHRT_ADDRESS=0.0.0.0 \
    SHRT_DB_PATH=/var/lib/shrt/shrt.db
ENTRYPOINT ["shrt"]

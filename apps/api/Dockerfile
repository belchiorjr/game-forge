# syntax=docker/dockerfile:1
# Build context: repository root (the image also needs templates/).

FROM golang:1.25-alpine AS build
WORKDIR /src/apps/api
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/forge-api ./cmd/api

# Pre-warm the template's module cache so agent builds inside the container work offline.
FROM golang:1.25-alpine AS modcache
WORKDIR /tpl
COPY templates/ebiten-game/go.mod templates/ebiten-game/go.sum ./
RUN go mod download

# Runtime keeps the Go toolchain and git: the agent builds and tests the games it writes.
FROM golang:1.25-alpine
RUN apk add --no-cache git ca-certificates \
 && adduser -D -u 10001 forge \
 && mkdir -p /data /workspaces \
 && chown forge:forge /data /workspaces
COPY --from=modcache --chown=forge:forge /go/pkg/mod /go/pkg/mod
COPY --from=build /out/forge-api /usr/local/bin/forge-api
COPY templates /app/templates
USER forge
ENV FORGE_ADDR=:8787 \
    FORGE_DATA_DIR=/data \
    FORGE_WORKSPACES_DIR=/workspaces \
    FORGE_TEMPLATES_DIR=/app/templates \
    GOFLAGS=-mod=mod \
    GOCACHE=/data/gocache
EXPOSE 8787
HEALTHCHECK --interval=15s --timeout=3s CMD wget -qO- http://localhost:8787/healthz || exit 1
ENTRYPOINT ["forge-api"]

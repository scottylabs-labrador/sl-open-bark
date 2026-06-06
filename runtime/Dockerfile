# Monorepo build for the runtime service (cmd/runtime-server). Build CONTEXT is the repo root
# (Railway uploads the git root, pointed here via RAILWAY_DOCKERFILE_PATH), so paths are prefixed
# with runtime/.
FROM golang:1.26 AS build
WORKDIR /src
COPY runtime/go.mod runtime/go.sum ./
RUN go mod download
COPY runtime/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/runtime-server ./cmd/runtime-server

# The final image bundles the Goose CLI so the runtime can drive the agent loop. Goose is a Rust
# binary that needs glibc, so this uses debian-slim (not distroless/static). The install is
# best-effort: if it fails the service still serves /healthz and accepts config; tasks then return a
# clear "goose not installed" error until it is fixed.
FROM debian:stable-slim
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates bzip2 \
    && rm -rf /var/lib/apt/lists/*
RUN (curl -fsSL https://github.com/block/goose/releases/download/stable/download_cli.sh \
      | CONFIGURE=false bash) \
    || echo "WARN: goose CLI install failed during build; runtime will report it at task time"
ENV PATH="/root/.local/bin:/usr/local/bin:${PATH}"
COPY --from=build /out/runtime-server /usr/local/bin/runtime-server
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/runtime-server"]

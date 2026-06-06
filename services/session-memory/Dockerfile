# Monorepo build for the Memory MCP server (cmd/memory-mcp). Build CONTEXT is the repo root (Railway
# uploads the git root, pointed here via RAILWAY_DOCKERFILE_PATH), so paths are prefixed with
# services/session-memory/. The retention job (cmd/retention) ships in the same module and runs as a
# Railway cron command.
FROM golang:1.26 AS build
WORKDIR /src
COPY services/session-memory/go.mod services/session-memory/go.sum ./
RUN go mod download
COPY services/session-memory/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/memory-mcp ./cmd/memory-mcp

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/memory-mcp /memory-mcp
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/memory-mcp"]

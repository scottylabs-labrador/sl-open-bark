# Monorepo build for the Finance Rules MCP. Build CONTEXT is the repo root (Railway uploads the git
# root and is pointed here via the RAILWAY_DOCKERFILE_PATH service variable), so paths are prefixed
# with mcp-servers/finance-rules/. For a standalone (Path B) repo, build from the module dir instead.
FROM golang:1.26 AS build
WORKDIR /src
COPY mcp-servers/finance-rules/go.mod mcp-servers/finance-rules/go.sum ./
RUN go mod download
COPY mcp-servers/finance-rules/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/server /server
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]

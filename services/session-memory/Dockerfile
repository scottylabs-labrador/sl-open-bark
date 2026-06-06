# Builds the Memory MCP server (cmd/memory-mcp). The retention job (cmd/retention) ships in the
# same module and runs as a Railway cron command.
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/memory-mcp ./cmd/memory-mcp

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/memory-mcp /memory-mcp
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/memory-mcp"]

# Monorepo build for the dashboard. Build CONTEXT is the repo root (Railway uploads the git root,
# pointed here via RAILWAY_DOCKERFILE_PATH); it replaces ../session-memory for the store. The web UI
# is compiled into the binary via go:embed, so the distroless/static image needs nothing else.
FROM golang:1.26 AS build
WORKDIR /src
COPY services/session-memory/ ./services/session-memory/
COPY services/dashboard/ ./services/dashboard/
WORKDIR /src/services/dashboard
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/dashboard ./cmd/dashboard

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/dashboard /dashboard
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/dashboard"]

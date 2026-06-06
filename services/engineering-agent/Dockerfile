# Monorepo build for the Engineering Agent orchestrator. CONTEXT is the repo root
# (RAILWAY_DOCKERFILE_PATH); self-contained module. Isolated trust domain — no gateway/Google/
# finance/production-secret access.
FROM golang:1.26 AS build
WORKDIR /src
COPY services/engineering-agent/go.mod services/engineering-agent/go.sum ./
RUN go mod download
COPY services/engineering-agent/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/engineering-agent ./cmd/engineering-agent

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/engineering-agent /engineering-agent
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/engineering-agent"]

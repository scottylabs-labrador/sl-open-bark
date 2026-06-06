# Multi-stage build for the gateway policy service.
#
# IMPORTANT: the build context MUST be the repo root, because this module replaces
# ../services/session-memory (the shared WP-01 store). Build from the repo root with:
#
#   docker build -f gateway/Dockerfile -t scottylabs-gateway .
#
# On Railway, set the service's build to use this Dockerfile with the repo root as context.

FROM golang:1.26 AS build
WORKDIR /src
# Copy the shared store first (the replace target), then the gateway module.
COPY services/session-memory/ ./services/session-memory/
COPY gateway/ ./gateway/
WORKDIR /src/gateway
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gateway ./cmd/gateway

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/gateway /gateway
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/gateway"]

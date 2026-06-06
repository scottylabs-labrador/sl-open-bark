# Monorepo build for the Slack gateway. Build CONTEXT is the repo root (Railway uploads the git
# root, pointed here via RAILWAY_DOCKERFILE_PATH), so paths are prefixed with services/slack-gateway/.
FROM golang:1.26 AS build
WORKDIR /src
COPY services/slack-gateway/go.mod services/slack-gateway/go.sum ./
RUN go mod download
COPY services/slack-gateway/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/slack-gateway ./cmd/slack-gateway

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/slack-gateway /slack-gateway
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/slack-gateway"]

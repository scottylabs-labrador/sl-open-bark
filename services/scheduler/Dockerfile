# Monorepo build for the scheduler. CONTEXT is the repo root (RAILWAY_DOCKERFILE_PATH); self-
# contained module. Default command is the long-running scheduler (ticks every minute, serves
# /healthz); `scheduler run` is a one-shot for Railway cron.
FROM golang:1.26 AS build
WORKDIR /src
COPY services/scheduler/go.mod services/scheduler/go.sum ./
RUN go mod download
COPY services/scheduler/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/scheduler ./cmd/scheduler

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/scheduler /scheduler
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/scheduler"]

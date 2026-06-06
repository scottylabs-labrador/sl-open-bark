# Multi-stage build -> a tiny static binary on a distroless base. This is the Go advantage:
# fast cold start and low memory, which keeps always-on Railway cost down.

FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/server /server
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]

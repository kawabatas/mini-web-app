## Build stage
FROM golang:1.25.3-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY frontend/dist/ ./frontend/dist
COPY / ./
ENV CGO_ENABLED=0 GOOS=linux
RUN go build -o /out/server ./cmd/server

## Run stage
FROM debian:bookworm-slim
RUN set -x && apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y \
    ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /out/server .
COPY --from=builder /app/frontend/dist ./frontend/dist
ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]

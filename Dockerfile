# Build stage - compile both server and worker binaries
FROM golang:1.25 AS builder

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build server binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./server

# Build worker binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /worker ./worker

# Server image - minimal image with just the server binary
FROM debian:bookworm-slim AS server

WORKDIR /app

COPY --from=builder /server /app/server

EXPOSE 8080

ENTRYPOINT ["/app/server"]

# Worker image - minimal image with just the worker binary
FROM debian:bookworm-slim AS worker

WORKDIR /app

COPY --from=builder /worker /app/worker

ENTRYPOINT ["/app/worker"]
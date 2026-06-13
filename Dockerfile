# Stage 1: Build the Go binary
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Copy go.mod and go.sum for caching dependency downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ws-reachability main.go

# Stage 2: Minimal runtime image
FROM alpine:latest

RUN apk update && apk add --no-cache ca-certificates && rm -rf /var/cache/apk/*

WORKDIR /app

# Copy binary from builder
COPY --from=builder /src/ws-reachability .

# Copy default config template
COPY docker/data/config.json ./data/config.json

# Declare data volume for custom configuration file
VOLUME /app/data

CMD ["./ws-reachability"]

# --- Build stage ---
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
COPY pkg/socketio/go.mod pkg/socketio/
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /hwarr-server .

# --- Runtime stage ---
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /hwarr-server .
COPY --from=builder /app/data/ ./data/ 2>/dev/null || true

ENV HOST=0.0.0.0
ENV PORT=8000
ENV GIN_MODE=release

EXPOSE 8000

ENTRYPOINT ["./hwarr-server"]

# --- Build stage ---
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates curl

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
COPY pkg/socketio/go.mod pkg/socketio/
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /hwarr-server .

# Admin-dong GeoJSON for daily ranking (see internal/geodata/loader.go).
# Overridable via build-arg if upstream publishes a newer version.
ARG GEOJSON_URL=https://raw.githubusercontent.com/vuski/admdongkor/master/ver20230701/HangJeongDong_ver20230701.geojson
RUN mkdir -p data && curl -fsSL -o data/admin_dong.geojson "$GEOJSON_URL"

# --- Runtime stage ---
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata curl && \
    adduser -D -u 1001 appuser

WORKDIR /app

COPY --from=builder /hwarr-server .
COPY --from=builder /app/data/ ./data/

ENV HOST=0.0.0.0
ENV PORT=8000
ENV GIN_MODE=release

USER appuser
EXPOSE 8000

HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
  CMD curl -f http://localhost:8000/health || exit 1

ENTRYPOINT ["./hwarr-server"]

# syntax=docker/dockerfile:1

# ---- build ----
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
# Static binaries: the API and goose (migrations run from the same image).
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/goose github.com/pressly/goose/v3/cmd/goose

# ---- runtime ----
FROM alpine:3.22
# ca-certificates: Google OAuth/Calendar over HTTPS. tzdata is embedded in the binary.
RUN apk add --no-cache ca-certificates wget && \
    addgroup -S -g 10001 app && adduser -S -u 10001 -G app app

WORKDIR /app
COPY --from=build /out/api /out/goose /usr/local/bin/
COPY migrations ./migrations

ENV APP_ENV=production \
    APP_PORT=8081 \
    UPLOAD_DIR=/app/uploads \
    LOG_DIR=/app/logs

RUN mkdir -p /app/uploads /app/logs && chown -R app:app /app
USER app

EXPOSE 8081
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=5 \
    CMD wget -qO- "http://127.0.0.1:${APP_PORT}/health/live" >/dev/null || exit 1

CMD ["api"]

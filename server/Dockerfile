# ---- Build stage ----
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/server ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/temporal-worker ./cmd/temporal-worker
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/reconcile-git-pr-status ./cmd/reconcile-git-pr-status
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/recover-missed-support-emails ./cmd/recover-missed-support-emails
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/reindex-helpcenter-search ./cmd/reindex-helpcenter-search
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/repair-helpcenter-artifacts ./cmd/repair-helpcenter-artifacts

# ---- Minimal runtime base ----
FROM alpine:3.19 AS runtime-base

RUN apk add --no-cache \
        ca-certificates \
        tzdata \
    && addgroup -S app -g 10001 \
    && adduser -S app -u 10001 -G app

WORKDIR /app

# ---- API runtime ----
FROM runtime-base AS api

COPY --from=builder /app/server .
COPY --from=builder /app/reconcile-git-pr-status .
COPY --from=builder /app/recover-missed-support-emails .
COPY --from=builder /app/reindex-helpcenter-search .
COPY --from=builder /app/repair-helpcenter-artifacts .
RUN chown -R app:app /app

USER app:app

EXPOSE 8080

ENTRYPOINT ["./server"]

# ---- Migration runtime ----
FROM runtime-base AS migrate

COPY --from=builder /app/migrate .
COPY --from=builder /app/reindex-helpcenter-search .
RUN chown -R app:app /app

USER app:app

ENTRYPOINT ["./migrate"]

# ---- Temporal product-automation worker runtime ----
FROM runtime-base AS temporal-worker

COPY --from=builder /app/temporal-worker .
RUN chown -R app:app /app

USER app:app

ENTRYPOINT ["./temporal-worker"]

# Keep plain `docker build server/` as the minimal API image.
FROM api AS final

# syntax=docker/dockerfile:1.7

# ---- Build stage ----
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

ENV GOPRIVATE=github.com/helpin-ai

# Cache dependencies
COPY go.mod go.sum ./
RUN --mount=type=secret,id=github_token,required=true \
    GITHUB_TOKEN="$(cat /run/secrets/github_token)" \
    && git config --global url."https://x-access-token:${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/" \
    && go mod download \
    && git config --global --unset-all url."https://x-access-token:${GITHUB_TOKEN}@github.com/".insteadOf

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/server ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/temporal-worker ./cmd/temporal-worker
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/helpin-mcp-bridge ./cmd/helpin-mcp-bridge
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/reconcile-git-pr-status ./cmd/reconcile-git-pr-status
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/recover-missed-support-emails ./cmd/recover-missed-support-emails
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/reindex-helpcenter-search ./cmd/reindex-helpcenter-search

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

RUN apk add --no-cache \
        nodejs \
        npm \
    && npm i -g @openai/codex

COPY --from=builder /app/server .
COPY --from=builder /app/helpin-mcp-bridge /usr/local/bin/helpin-mcp-bridge
COPY --from=builder /app/reconcile-git-pr-status .
COPY --from=builder /app/recover-missed-support-emails .
COPY --from=builder /app/reindex-helpcenter-search .
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

# ---- Temporal worker runtime ----
FROM alpine:3.19 AS temporal-worker

ARG TRIVY_VERSION=0.69.3
ARG GITLEAKS_VERSION=8.30.1

ENV XDG_CACHE_HOME=/app/.cache \
    TRIVY_CACHE_DIR=/app/.cache/trivy \
    SEMGREP_SETTINGS_FILE=/app/.cache/semgrep/settings.yml

RUN apk add --no-cache \
        ca-certificates \
        tzdata \
        curl \
        ripgrep \
        git \
        nodejs \
        npm \
        go \
        python3 \
        py3-pip \
        rust \
        cargo \
    && mkdir -p /app/.cache/trivy /app/.cache/semgrep /app/security-rules \
    && ln -sf /usr/bin/python3 /usr/bin/python \
    && pip3 install --no-cache-dir --break-system-packages pytest uv poetry semgrep \
    && npm i -g @openai/codex opencode-ai pnpm yarn \
    && curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin "v${TRIVY_VERSION}" \
    && curl -sfL "https://github.com/gitleaks/gitleaks/releases/download/v${GITLEAKS_VERSION}/gitleaks_${GITLEAKS_VERSION}_linux_x64.tar.gz" | tar -xz -C /usr/local/bin gitleaks \
    && trivy image --download-db-only --cache-dir "$TRIVY_CACHE_DIR" \
    && trivy image --download-java-db-only --cache-dir "$TRIVY_CACHE_DIR" \
    && printf '%s\n' 'apiVersion: v1' 'kind: Pod' 'metadata:' '  name: trivy-prewarm' 'spec:' '  containers:' '    - name: app' '      image: alpine:3.19' > /tmp/trivy-prewarm.yml \
    && trivy fs --cache-dir "$TRIVY_CACHE_DIR" --scanners misconfig --format json --skip-version-check /tmp/trivy-prewarm.yml >/dev/null \
    && rm -f /tmp/trivy-prewarm.yml \
    && rm -rf /root/go/pkg /root/.cache/go-build \
    && addgroup -S app -g 10001 \
    && adduser -S app -u 10001 -G app

WORKDIR /app

COPY --from=builder /app/temporal-worker .
COPY --from=builder /app/helpin-mcp-bridge /usr/local/bin/helpin-mcp-bridge
COPY skills/system/security_triage/semgrep /app/security-rules/semgrep
RUN chown -R app:app /app

USER app:app

ENTRYPOINT ["./temporal-worker"]

# Keep plain `docker build server/` as the minimal API image.
FROM api AS final

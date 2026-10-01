# Build from the Helpin repository root. Never copy an operator .env into images.
ARG BUILDPLATFORM
FROM node:24.21.0-bookworm-slim@sha256:2fe369e969550cde8e867afc3fe370b260140cab4a23d467074295b42163d553 AS sdk-build
ENV CI=true NODE_OPTIONS=--max-old-space-size=4096
WORKDIR /workspace
RUN corepack enable && corepack prepare pnpm@10.32.1 --activate
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.base.json ./
COPY packages/ ./packages/
COPY frontend/ ./frontend/
COPY help-center/ ./help-center/
RUN pnpm install --frozen-lockfile
RUN pnpm --filter @helpin-ai/sdk-js... build
FROM sdk-build AS frontend-build
RUN VITE_API_URL=/api VITE_EDITION=community pnpm --dir frontend run build
FROM sdk-build AS helpcenter-build
RUN pnpm --dir help-center run build && pnpm --filter help-center deploy --prod --legacy /out/help-center

FROM --platform=$BUILDPLATFORM golang:1.26.7-bookworm AS go-build
ARG TARGETARCH
ENV GOWORK=off
WORKDIR /workspace/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -trimpath -ldflags='-s -w' -o /out/worker ./cmd/temporal-worker && \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -trimpath -ldflags='-s -w' -o /out/migrate ./cmd/migrate

FROM debian:bookworm-slim AS server-base
# CI passes the build date so OS security updates refresh daily while the
# application layers stay cached.
ARG OS_PACKAGES_DATE=unset
COPY LICENSE LICENSE-AGPL-3.0 LICENSE-APACHE-2.0 /usr/share/licenses/helpin/
COPY ee/LICENSE /usr/share/licenses/helpin/ee/LICENSE
RUN echo "OS packages as of $OS_PACKAGES_DATE" && apt-get update && apt-get upgrade -y && apt-get install -y --no-install-recommends ca-certificates tzdata wget && rm -rf /var/lib/apt/lists/* && useradd --uid 10001 --create-home app
WORKDIR /app
USER app
FROM server-base AS api
COPY --from=go-build /out/api /app/api
COPY --from=sdk-build /workspace/packages/sdk-js/dist /app/sdk
ENV SDK_DIST_DIR=/app/sdk
EXPOSE 8080
ENTRYPOINT ["/app/api"]
FROM server-base AS worker
COPY --from=go-build /out/worker /app/worker
ENTRYPOINT ["/app/worker"]
FROM server-base AS migrate
COPY --from=go-build /out/migrate /app/migrate
ENTRYPOINT ["/app/migrate"]
FROM nginx:1.28.0-alpine AS frontend
ARG OS_PACKAGES_DATE=unset
COPY LICENSE LICENSE-AGPL-3.0 LICENSE-APACHE-2.0 /usr/share/licenses/helpin/
COPY ee/LICENSE /usr/share/licenses/helpin/ee/LICENSE
RUN echo "OS packages as of $OS_PACKAGES_DATE" && apk upgrade --no-cache
ENV COMMUNITY_TRUSTED_PROXY_CIDR=127.0.0.1/32
COPY --from=frontend-build /workspace/frontend/dist /usr/share/nginx/html
COPY community/images/nginx.conf /etc/nginx/templates/default.conf.template
EXPOSE 80
FROM node:24.21.0-bookworm-slim@sha256:2fe369e969550cde8e867afc3fe370b260140cab4a23d467074295b42163d553 AS helpcenter
ARG OS_PACKAGES_DATE=unset
COPY LICENSE LICENSE-AGPL-3.0 LICENSE-APACHE-2.0 /usr/share/licenses/helpin/
COPY ee/LICENSE /usr/share/licenses/helpin/ee/LICENSE
ENV NODE_ENV=production PORT=3000
RUN echo "OS packages as of $OS_PACKAGES_DATE" && apt-get update && apt-get upgrade -y && rm -rf /var/lib/apt/lists/* && \
    rm -rf /usr/local/lib/node_modules/npm /usr/local/bin/npm /usr/local/bin/npx /opt/yarn* /usr/local/bin/yarn*
WORKDIR /workspace/help-center
COPY --from=helpcenter-build /out/help-center/node_modules ./node_modules
COPY --from=helpcenter-build /workspace/help-center/dist ./dist
COPY help-center/package.json help-center/*.mjs ./
USER node
# Check SSR imports against the production-only tree before shipping the image.
RUN node --input-type=module -e "await import('./dist/server/server.js')"
EXPOSE 3000
CMD ["node", "serve.mjs"]

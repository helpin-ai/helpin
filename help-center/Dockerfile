# The customer portal build, served by this server at /requests
# (see portalMount.mjs). It is built from the app's portal pages.
FROM node:24.21.0-alpine@sha256:be80f76cf40ec8e42b9bec49f60a55e0660f30af58d3e5a25530785b30ea67e2 AS portal
ENV CI=true
WORKDIR /workspace
RUN corepack enable
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.base.json ./
COPY packages/ ./packages/
COPY frontend/ ./frontend/
# support-core and shared are aliased to source, so install their dependencies too.
RUN pnpm install --frozen-lockfile --filter frontend... --filter @helpin-ai/support-core... --filter @helpin-ai/shared...
RUN NODE_OPTIONS=--max-old-space-size=4096 pnpm --dir frontend run build:portal

FROM node:24.21.0-alpine@sha256:be80f76cf40ec8e42b9bec49f60a55e0660f30af58d3e5a25530785b30ea67e2 AS build
ENV CI=true
WORKDIR /workspace/help-center
RUN corepack enable
COPY help-center/package.json ./
COPY help-center/pnpm-lock.yaml ./
COPY help-center/ ./
COPY packages/shared/src/docs-blocks.css /workspace/packages/shared/src/docs-blocks.css
RUN pnpm install --frozen-lockfile
RUN pnpm run build
RUN pnpm prune --prod

FROM node:24.21.0-alpine@sha256:be80f76cf40ec8e42b9bec49f60a55e0660f30af58d3e5a25530785b30ea67e2
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000
COPY --from=build /workspace/help-center/package.json ./package.json
COPY --from=build /workspace/help-center/node_modules ./node_modules
COPY --from=build /workspace/help-center/dist ./dist
COPY --from=build /workspace/help-center/serve.mjs ./serve.mjs
COPY --from=build /workspace/help-center/assetUrls.mjs ./assetUrls.mjs
COPY --from=build /workspace/help-center/serverCompression.mjs ./serverCompression.mjs
COPY --from=build /workspace/help-center/serverRenderCache.mjs ./serverRenderCache.mjs
COPY --from=build /workspace/help-center/serverRedirects.mjs ./serverRedirects.mjs
COPY --from=build /workspace/help-center/serverSeo.mjs ./serverSeo.mjs
COPY --from=build /workspace/help-center/portalMount.mjs ./portalMount.mjs
COPY --from=portal /workspace/frontend/dist-portal ./portal
RUN node --input-type=module -e "await import('./dist/server/server.js')"
EXPOSE 3000
CMD ["node", "serve.mjs"]

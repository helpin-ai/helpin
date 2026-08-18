FROM node:22-alpine AS build
ENV CI=true
WORKDIR /workspace/help-center
RUN corepack enable
COPY help-center/package.json ./
COPY help-center/pnpm-lock.yaml ./
COPY help-center/ ./
COPY packages/shared/src/docs-blocks.css /workspace/packages/shared/src/docs-blocks.css
RUN pnpm install --frozen-lockfile
RUN pnpm run build

FROM node:22-alpine
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
EXPOSE 3000
CMD ["node", "serve.mjs"]

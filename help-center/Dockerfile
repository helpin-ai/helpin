FROM node:22-alpine AS build
WORKDIR /app
RUN corepack enable
COPY package.json ./
COPY pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY . .
RUN pnpm run build

FROM node:22-alpine
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000
COPY --from=build /app/package.json ./package.json
COPY --from=build /app/node_modules ./node_modules
COPY --from=build /app/dist ./dist
COPY --from=build /app/serve.mjs ./serve.mjs
COPY --from=build /app/serverRedirects.mjs ./serverRedirects.mjs
COPY --from=build /app/serverSeo.mjs ./serverSeo.mjs
EXPOSE 3000
CMD ["node", "serve.mjs"]

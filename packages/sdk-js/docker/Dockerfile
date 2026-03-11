FROM node:20-alpine AS builder
WORKDIR /usr/src
RUN npm install -g pnpm

COPY . .
# Clear npm cache and remove existing node_modules
RUN npm cache clean --force && \
    rm -rf node_modules
RUN pnpm install && pnpm add typescript@latest -w
# RUN pnpm add -D -w rollup @rollup/plugin-node-resolve @rollup/plugin-commonjs 
RUN pnpm build
RUN ls -R
FROM nginx:latest
COPY --from=builder /usr/src/packages/javascript-sdk/dist/lib.js /usr/share/nginx/html
RUN rm /etc/nginx/nginx.conf && rm /etc/nginx/conf.d/default.conf
RUN ls -al
COPY packages/javascript-sdk/docker/nginx.conf /etc/nginx/nginx.conf
COPY packages/javascript-sdk/docker/default.conf /etc/nginx/conf.d/default.conf

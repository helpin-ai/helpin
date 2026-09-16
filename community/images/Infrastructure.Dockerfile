# Small security rebuilds: keep upstream services and protocols unchanged.
FROM --platform=$BUILDPLATFORM golang:1.26.7-bookworm AS gosu-build
ARG TARGETARCH
WORKDIR /src
RUN go mod init community-gosu && go get github.com/tianon/gosu@v0.0.0-20260606051551-40506998e34a
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o /out/gosu github.com/tianon/gosu
FROM pgvector/pgvector:0.8.6-pg17-bookworm AS postgres
RUN apt-get update && apt-get upgrade -y && rm -rf /var/lib/apt/lists/*
COPY --from=gosu-build /out/gosu /usr/local/bin/gosu
COPY --from=gosu-build /go/pkg/mod/github.com/tianon/gosu@v0.0.0-20260606051551-40506998e34a/LICENSE /usr/share/doc/gosu/LICENSE
LABEL org.opencontainers.image.source="https://github.com/pgvector/pgvector"

FROM --platform=$BUILDPLATFORM golang:1.26.7-bookworm AS nats-build
ARG TARGETARCH
WORKDIR /src
RUN go mod init community-nats && go get github.com/nats-io/nats-server/v2@v2.14.7
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o /out/nats-server github.com/nats-io/nats-server/v2
FROM alpine:3.23 AS nats
RUN apk upgrade --no-cache && apk add --no-cache ca-certificates && adduser -D -u 10001 nats && mkdir /data && chown nats:nats /data
COPY --from=nats-build /out/nats-server /usr/local/bin/nats-server
COPY --from=nats-build /go/pkg/mod/github.com/nats-io/nats-server/v2@v2.14.7/LICENSE /usr/share/doc/nats/LICENSE
USER nats
ENTRYPOINT ["nats-server"]
LABEL org.opencontainers.image.source="https://github.com/nats-io/nats-server"

FROM --platform=$BUILDPLATFORM golang:1.26.7-bookworm AS mc-build
ARG TARGETARCH
RUN go mod download github.com/minio/mc@v0.0.0-20251106162529-77f82e18b540
RUN mkdir /source && cp -a /go/pkg/mod/github.com/minio/mc@* /source/mc && chmod -R u+w /source/mc
WORKDIR /source/mc
RUN go get github.com/prometheus/prometheus@v0.311.3 golang.org/x/crypto@v0.55.0 google.golang.org/grpc@v1.83.2
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -mod=mod -o /out/mc .
FROM debian:bookworm-slim AS mc
RUN apt-get update && apt-get upgrade -y && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/* && useradd -u 10001 --create-home mc
COPY --from=mc-build /out/mc /usr/local/bin/mc
COPY --from=mc-build /source/mc /usr/share/mc-source
USER mc
ENTRYPOINT ["mc"]
LABEL org.opencontainers.image.source="https://github.com/minio/mc" org.opencontainers.image.licenses="AGPL-3.0-only"

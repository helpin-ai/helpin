# Upstream's final security release is source-only. Pin through Go's checksum
# database and retain the corresponding source/license for redistribution.
FROM --platform=$BUILDPLATFORM golang:1.26.7-bookworm AS build
ARG TARGETARCH
ENV CGO_ENABLED=0
RUN go mod download github.com/minio/minio@RELEASE.2025-10-15T17-29-55Z
RUN mkdir /source && cp -a /go/pkg/mod/github.com/minio/minio@* /source/minio && chmod -R u+w /source/minio
WORKDIR /source/minio
# Upstream's archived release predates these dependency security fixes. Ship
# the modified corresponding source, including its exact module checksums.
RUN go get github.com/apache/thrift@v0.24.0 github.com/buger/jsonparser@v1.1.2 \
    github.com/go-jose/go-jose/v4@v4.1.4 github.com/prometheus/prometheus@v0.311.3 \
    github.com/rabbitmq/amqp091-go@v1.13.0 go.opentelemetry.io/otel/sdk@v1.44.0 \
    golang.org/x/crypto@v0.55.0 google.golang.org/grpc@v1.83.2
RUN GOOS=linux GOARCH=$TARGETARCH go build -mod=mod -o /out/minio .
FROM debian:bookworm-slim
RUN apt-get update && apt-get upgrade -y && apt-get install -y --no-install-recommends ca-certificates curl && rm -rf /var/lib/apt/lists/* && useradd -u 10001 --create-home minio && mkdir /data && chown minio:minio /data
COPY --from=build /out/minio /usr/local/bin/minio
COPY --from=build /source/minio /usr/share/minio-source
LABEL org.opencontainers.image.source="https://github.com/minio/minio" org.opencontainers.image.licenses="AGPL-3.0-only" org.opencontainers.image.version="RELEASE.2025-10-15T17-29-55Z"
USER minio
EXPOSE 9000
ENTRYPOINT ["minio"]

FROM rust:1.88-bookworm AS builder

RUN apt-get update \
    && apt-get install -y --no-install-recommends pkg-config \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY . .
RUN cargo build --release --bins

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl libssl3 \
    && rm -rf /var/lib/apt/lists/*

ENV TZ=UTC
WORKDIR /app

COPY --from=builder /app/target/release/events-pipeline /usr/local/bin/events-pipeline
COPY --from=builder /app/target/release/replay-worker /usr/local/bin/replay-worker
COPY --from=builder /app/target/release/nats-bootstrap /usr/local/bin/nats-bootstrap
COPY --from=builder /app/target/release/session-writer /usr/local/bin/session-writer
COPY --from=builder /app/data ./data

CMD ["events-pipeline"]

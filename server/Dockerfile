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

# ---- Run stage ----
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata ripgrep \
    && addgroup -S app -g 10001 \
    && adduser -S app -u 10001 -G app

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/temporal-worker .
COPY --from=builder /app/migrate .
RUN chown -R app:app /app

USER app:app

EXPOSE 8080

ENTRYPOINT ["./server"]

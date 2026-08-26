#!/usr/bin/env bash

prepare_e2e_nats_tls() {
  tls_dir=$1
  mkdir -p "$tls_dir"

  openssl req -x509 -newkey rsa:2048 -sha256 -days 1 -nodes \
    -subj '/CN=helpin-e2e-ca' \
    -keyout "$tls_dir/ca.key" \
    -out "$tls_dir/ca.crt" >/dev/null 2>&1

  openssl req -new -newkey rsa:2048 -sha256 -nodes \
    -subj '/CN=helpin-e2e-nats' \
    -addext 'subjectAltName=DNS:nats-0,DNS:nats-1,DNS:nats-2,DNS:localhost,IP:127.0.0.1' \
    -keyout "$tls_dir/server.key" \
    -out "$tls_dir/server.csr" >/dev/null 2>&1
  openssl x509 -req -sha256 -days 1 -copy_extensions copy \
    -in "$tls_dir/server.csr" \
    -CA "$tls_dir/ca.crt" \
    -CAkey "$tls_dir/ca.key" \
    -CAcreateserial \
    -out "$tls_dir/server.crt" >/dev/null 2>&1

  openssl req -new -newkey rsa:2048 -sha256 -nodes \
    -subj '/CN=helpin-e2e-client' \
    -addext 'extendedKeyUsage=clientAuth' \
    -keyout "$tls_dir/client.key" \
    -out "$tls_dir/client.csr" >/dev/null 2>&1
  openssl x509 -req -sha256 -days 1 -copy_extensions copy \
    -in "$tls_dir/client.csr" \
    -CA "$tls_dir/ca.crt" \
    -CAkey "$tls_dir/ca.key" \
    -CAcreateserial \
    -out "$tls_dir/client.crt" >/dev/null 2>&1

  chmod 0644 "$tls_dir/ca.crt" "$tls_dir/server.crt" "$tls_dir/server.key" \
    "$tls_dir/client.crt" "$tls_dir/client.key"
}

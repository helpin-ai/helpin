#!/usr/bin/env bash

run_e2e_openssl() {
  local error_log=$1
  shift
  if openssl "$@" >/dev/null 2>"$error_log"; then
    rm -f "$error_log"
    return 0
  fi

  echo "OpenSSL failed while generating E2E NATS certificates:" >&2
  sed 's/^/  /' "$error_log" >&2
  return 1
}

prepare_e2e_nats_tls() {
  local tls_dir=$1
  local openssl_error_log
  local openssl_version
  mkdir -p "$tls_dir"
  openssl_error_log="$tls_dir/openssl-error.log"

  if ! command -v openssl >/dev/null 2>&1; then
    echo "OpenSSL 3 is required to generate E2E NATS certificates." >&2
    return 1
  fi
  if ! openssl x509 -help 2>&1 | grep -q -- '-copy_extensions'; then
    openssl_version=$(openssl version 2>&1 || true)
    echo "OpenSSL 3 with x509 -copy_extensions is required; found: $openssl_version" >&2
    echo "macOS: brew install openssl@3, then prepend its bin directory to PATH (see brew --prefix openssl@3)." >&2
    return 1
  fi

  run_e2e_openssl "$openssl_error_log" req -x509 -newkey rsa:2048 -sha256 -days 1 -nodes \
    -subj '/CN=helpin-e2e-ca' \
    -keyout "$tls_dir/ca.key" \
    -out "$tls_dir/ca.crt"

  run_e2e_openssl "$openssl_error_log" req -new -newkey rsa:2048 -sha256 -nodes \
    -subj '/CN=helpin-e2e-nats' \
    -addext 'subjectAltName=DNS:nats-0,DNS:nats-1,DNS:nats-2,DNS:localhost,IP:127.0.0.1' \
    -keyout "$tls_dir/server.key" \
    -out "$tls_dir/server.csr"
  run_e2e_openssl "$openssl_error_log" x509 -req -sha256 -days 1 -copy_extensions copy \
    -in "$tls_dir/server.csr" \
    -CA "$tls_dir/ca.crt" \
    -CAkey "$tls_dir/ca.key" \
    -CAcreateserial \
    -out "$tls_dir/server.crt"

  run_e2e_openssl "$openssl_error_log" req -new -newkey rsa:2048 -sha256 -nodes \
    -subj '/CN=helpin-e2e-client' \
    -addext 'extendedKeyUsage=clientAuth' \
    -keyout "$tls_dir/client.key" \
    -out "$tls_dir/client.csr"
  run_e2e_openssl "$openssl_error_log" x509 -req -sha256 -days 1 -copy_extensions copy \
    -in "$tls_dir/client.csr" \
    -CA "$tls_dir/ca.crt" \
    -CAkey "$tls_dir/ca.key" \
    -CAcreateserial \
    -out "$tls_dir/client.crt"

  chmod 0644 "$tls_dir/ca.crt" "$tls_dir/server.crt" "$tls_dir/server.key" \
    "$tls_dir/client.crt" "$tls_dir/client.key"
}

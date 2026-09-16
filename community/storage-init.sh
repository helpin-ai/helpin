#!/bin/sh
set -eu
mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null
mc mb --ignore-existing local/helpin >/dev/null
# Public documentation assets only. Never grant anonymous bucket listing or writes.
mc anonymous set-json /setup/public-policy.json local/helpin >/dev/null

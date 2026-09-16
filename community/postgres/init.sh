#!/usr/bin/env bash
set -euo pipefail
# Passwords are passed through psql variables and quoted as SQL literals, never
# interpolated into SQL or printed. This runs once on an empty Postgres volume.
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
  --set=helpin_password="$HELPIN_DB_PASSWORD" \
  --set=runtime_password="$RUNTIME_DB_PASSWORD" \
  --set=temporal_password="$TEMPORAL_DB_PASSWORD" \
  --set=visibility_password="$VISIBILITY_DB_PASSWORD" <<'SQL'
CREATE ROLE helpin LOGIN PASSWORD :'helpin_password';
CREATE ROLE agent_runtime LOGIN PASSWORD :'runtime_password';
CREATE ROLE temporal LOGIN PASSWORD :'temporal_password';
CREATE ROLE temporal_visibility LOGIN PASSWORD :'visibility_password';
CREATE DATABASE helpin OWNER helpin;
CREATE DATABASE agent_runtime OWNER agent_runtime;
CREATE DATABASE temporal OWNER temporal;
CREATE DATABASE temporal_visibility OWNER temporal_visibility;
REVOKE CONNECT ON DATABASE helpin FROM PUBLIC;
REVOKE CONNECT ON DATABASE agent_runtime FROM PUBLIC;
REVOKE CONNECT ON DATABASE temporal FROM PUBLIC;
REVOKE CONNECT ON DATABASE temporal_visibility FROM PUBLIC;
\connect helpin
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pgcrypto;
SQL

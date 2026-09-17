#!/bin/sh
set -eu
export SQL_HOST=postgres SQL_PORT=5432 SQL_PLUGIN=postgres12
export SQL_USER=temporal SQL_DATABASE=temporal SQL_PASSWORD="$TEMPORAL_DB_PASSWORD"
temporal-sql-tool setup-schema -v 0.0
temporal-sql-tool update-schema -d /etc/temporal/schema/postgresql/v12/temporal/versioned
export SQL_USER=temporal_visibility SQL_DATABASE=temporal_visibility SQL_PASSWORD="$VISIBILITY_DB_PASSWORD"
temporal-sql-tool setup-schema -v 0.0
temporal-sql-tool update-schema -d /etc/temporal/schema/postgresql/v12/visibility/versioned

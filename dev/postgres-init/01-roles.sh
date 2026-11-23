#!/usr/bin/env bash
# Runs once, when the local PostgreSQL volume is first created (docker-entrypoint-initdb.d).
# Creates the two roles from ADR 0008:
#   saas_owner — owns the database and schema, runs migrations
#   saas_app   — used by the API at runtime; no ownership and no BYPASSRLS, so row-level security applies
set -euo pipefail

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
  -v owner_password="$POSTGRES_OWNER_PASSWORD" \
  -v app_password="$POSTGRES_APP_PASSWORD" \
  -v db="$POSTGRES_DB" <<'SQL'
CREATE ROLE saas_owner LOGIN PASSWORD :'owner_password';
CREATE ROLE saas_app LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS PASSWORD :'app_password';
ALTER DATABASE :"db" OWNER TO saas_owner;
REVOKE ALL ON DATABASE :"db" FROM PUBLIC;
GRANT CONNECT ON DATABASE :"db" TO saas_app;
SQL

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<'SQL'
-- The public schema belongs to the database owner (saas_owner) since PostgreSQL 15.
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO saas_app;
-- Tables and sequences created by migrations become usable by the app role, nothing more.
ALTER DEFAULT PRIVILEGES FOR ROLE saas_owner IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO saas_app;
ALTER DEFAULT PRIVILEGES FOR ROLE saas_owner IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO saas_app;
SQL

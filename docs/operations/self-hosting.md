# Self-hosting and operations

This document defines the initial self-hosting baseline for Oke Gaas.

## Scope

The baseline is intentionally small:

- one Oke Gaas API container
- one PostgreSQL database
- one explicit migration job
- persistent PostgreSQL storage
- health checks
- documented backup and restore procedures

It does not introduce a message broker, async worker, managed cloud dependency, or automatic production schema mutation.

## Required configuration

Create a local `.env` file for Docker Compose. Do not commit it.

```dotenv
POSTGRES_DB=oke_gaas
POSTGRES_USER=oke_gaas
POSTGRES_PASSWORD=
OKE_GAAS_ADMIN_API_KEY=
OKE_GAAS_HTTP_PORT=8080
```

Both secret values are intentionally blank in `.env.example`. Docker Compose uses required-variable expansion and refuses to render/start the stack while either value is empty, preventing the checked-in example from becoming a usable credential. Generate secrets with an appropriate cryptographically secure password/secret generator. Because the baseline constructs PostgreSQL connection URIs from `POSTGRES_PASSWORD`, keep that value to URL-unreserved characters (`A-Z`, `a-z`, `0-9`, `-`, `.`, `_`, `~`) and use at least 32 random characters. `OKE_GAAS_ADMIN_API_KEY` must also contain at least 32 non-whitespace characters. Project API keys are provisioned by the API and are returned only at Project creation.

## Start the stack

```bash
docker compose up --build
```

Startup order is explicit:

```text
PostgreSQL healthy
      |
      v
migrate/migrate applies /migrations in order
      |
      v
migration job exits successfully
      |
      v
Oke Gaas API starts
```

The application itself does not run GORM `AutoMigrate` in production. Ordered SQL files under `/migrations` remain the production schema source of truth.

Health endpoint:

```text
GET http://localhost:8080/health
```

Override the host port with `OKE_GAAS_HTTP_PORT`.

## Migration operations

The Compose baseline pins the migration runner to `migrate/migrate:v4.19.1` and pins all self-hosting/base images by immutable digest. Human-readable version tags remain beside the digests so upgrades are explicit and reviewable; the digest is the reproducibility boundary actually selected by the container runtime.

Normal deployment order:

1. make a database backup when the schema/data change warrants one
2. start PostgreSQL
3. apply all pending ordered migrations
4. start application code that depends on the new schema
5. verify `/health` and a representative authenticated API read/write path

Do not edit migration files already applied to a shared environment. Add a new numbered migration instead.

If a migration fails, do not force the recorded migration version forward merely to unblock startup. Diagnose the migration, database state, and migration tool status first.

## Backup

The PostgreSQL volume is persistent, but a volume is not a backup.

Create a logical backup:

```bash
docker compose exec -T postgres sh -c \
  'pg_dump --username="$POSTGRES_USER" --dbname="$POSTGRES_DB" --format=custom --no-owner --no-privileges' \
  > oke-gaas.dump
```

Store backups outside the Docker volume and protect them according to the sensitivity of the data they contain.

Periodically verify that backups can actually be restored into an isolated database.

## Restore

> **Warning:** restoring over the active Oke Gaas database is destructive. Stop application writes first and keep a verified backup of the current state before replacing data.

For a clean replacement database in the Compose environment:

```bash
docker compose stop api
docker compose exec -T postgres sh -c \
  'dropdb --username="$POSTGRES_USER" --if-exists "$POSTGRES_DB" && createdb --username="$POSTGRES_USER" "$POSTGRES_DB"'
docker compose exec -T postgres sh -c \
  'pg_restore --username="$POSTGRES_USER" --dbname="$POSTGRES_DB" --no-owner --no-privileges' \
  < oke-gaas.dump
```

After restoring:

1. run the migration job again so the restored database reaches the schema version required by the deployed application
2. start the API
3. verify `/health`
4. verify one known Project-scoped read and, when safe, one write path
5. inspect logs for migration, authentication, or persistence errors

Run the migration job explicitly:

```bash
docker compose run --rm migrate
```

Then restart the API:

```bash
docker compose up -d api
```

## Production notes

The Compose file is a self-hosting baseline, not a complete internet-facing production platform.

A production deployment should additionally provide, as appropriate:

- TLS termination
- firewall/network policy
- secret management
- centralized logs
- monitored backups
- resource limits
- alerting and observability
- a controlled deployment/rollback process

Those capabilities should be added only through explicit operational decisions and concrete requirements.

## Self-hosting verification

The gated CI job runs `scripts/verify-self-hosting.sh`. The script validates Compose rendering, builds the API image, starts the complete stack, waits for `/health`, verifies the API process is non-root, and creates a Project through the authenticated REST API. This exercises the database health -> migrations -> API dependency chain on the reviewed revision.

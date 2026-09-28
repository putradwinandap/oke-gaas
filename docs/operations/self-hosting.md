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
# Optional; leave empty to disable telemetry export.
OTEL_EXPORTER_OTLP_ENDPOINT=
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

## Observability

The API creates OpenTelemetry server spans and request metrics for every HTTP route. Event ingestion adds a child span and records processing duration, outcome, duplicate status, and reward grant count. Telemetry does not include Project, Player, Event, or API-key identifiers, event properties, or raw error messages.

Export is disabled unless an OTLP endpoint is configured. Set `OTEL_EXPORTER_OTLP_ENDPOINT` to an OTLP/HTTP collector base URL (for example `http://otel-collector:4318`); the trace and metric exporters append their standard signal paths. Signal-specific `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` and `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` may be used instead. The Compose API service passes these settings through. The collector must be reachable from the API container, and production deployments should configure an authenticated, encrypted endpoint where the network requires it.

The API exports `http.server.requests`, `http.server.request.duration`, `oke_gaas.event.processing.count`, `oke_gaas.event.processing.duration`, and `oke_gaas.event.processing.reward_grants`. HTTP metrics use method, route, and status attributes; processing metrics use outcome and duplicate attributes, while grant count is a measurement. Keep route templates low-cardinality and do not add caller-controlled identifiers or Event properties to telemetry attributes.

## Migration operations

The Compose baseline pins the migration runner to `migrate/migrate:v4.19.1` and pins all self-hosting/base images by immutable digest. Human-readable version tags remain beside the digests so upgrades are explicit and reviewable; the digest is the reproducibility boundary actually selected by the container runtime. The final API image also avoids package-manager network installs and uses the BusyBox tooling already contained in the pinned Alpine base for its health check.

Normal deployment order:

1. make a database backup when the schema/data change warrants one
2. start PostgreSQL
3. apply all pending ordered migrations
4. start application code that depends on the new schema
5. verify `/health` and a representative authenticated API read/write path

Do not edit migration files already applied to a shared environment. Add a new numbered migration instead.

If a migration fails, do not force the recorded migration version forward merely to unblock startup. Diagnose the migration, database state, and migration tool status first.

### Aggregate-rule rollback boundary

Migration `000008_rule_match_counts` is backward-compatible for existing immediate rules because `match_every` defaults to `1`. Once any Rule with `match_every > 1` has been created, however, **do not roll the application binary back to a pre-000008 version**: older binaries do not understand aggregate thresholds and would evaluate those Rules as immediate XP Rules.

The `000008` down migration therefore fails closed while aggregate Rules exist. If aggregate behavior has been activated, recover with a forward-fix or restore a verified backup from before aggregate Rules were created. Do not delete aggregate Rules or their counters merely to force a rollback.

### Once-per-UTC-day rollback boundary

Migration `000009_rule_daily_claims` adds `once_per_utc_day` and the `rule_daily_claims` state that older binaries do not understand. Once any Rule with `once_per_utc_day = true` has been created, **do not roll the application binary back to a pre-000009 version**: an older binary would ignore the daily gate and could grant XP repeatedly for same-day Events.

The `000009` down migration therefore fails closed while daily Rules exist. If daily behavior has been activated, recover with a forward-fix or restore a verified backup from before daily Rules were created. Schema downgrades must be applied in reverse migration order: `000009` must be removed before attempting to remove `000008`.

### XP-level rollback boundary

Migration `000010_level_thresholds` adds Project-scoped Level configuration. Level resolution is derived from XP and does not alter Reward processing, so an older application binary cannot over-grant XP merely by ignoring this table. However, dropping the schema would destroy configured Level ladders and break clients that depend on Level responses.

The `000010` down migration therefore fails closed while any Level thresholds are configured. Prefer a forward-fix; otherwise restore a verified backup from before Level thresholds were created. Do not delete configured thresholds merely to force a rollback. Schema downgrades must remain in reverse migration order: `000010` before `000009`.

Migration `000011_counters` adds immutable Project Counter definitions and materialized Player Counter values. Once a Project has a Counter definition, a database trigger rejects Event-processing claims from binaries that do not declare Counter-aware processing; older binaries therefore fail closed for that Project instead of silently losing progress. The `000011` down migration also fails closed while any Counter definitions exist. Use a forward-fix or restore a verified backup from before Counters were configured. Downgrade in reverse migration order, with `000011` before `000010`.

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

Migration `000012_achievements` adds immutable Counter-linked Achievement definitions and auditable Player unlocks. Once a Project has any Achievement definition, the Event-processing trigger requires an Achievement-aware binary. The `000012` down migration fails closed while Achievement definitions exist. Use a forward-fix or restore a verified backup from before Achievements were configured. Downgrade in reverse migration order, with `000012` before `000011`.

Migration `000013_badge_rewards` adds Badge reward Rules and audit records after the Achievement schema. A Project with Badge Rules requires a Badge-aware binary. The `000013` down migration fails closed while Badge definitions, rules, or grants exist. Downgrade in reverse order: `000013` before `000012`.

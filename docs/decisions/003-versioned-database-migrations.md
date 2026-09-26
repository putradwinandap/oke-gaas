# ADR-003: Versioned Database Migrations

- **Status:** Accepted
- **Date:** 2026-09-26

## Context

Oke Gaas uses GORM with PostgreSQL. GORM `AutoMigrate` is convenient while the schema is small, but it is not an adequate production migration history because production changes must be ordered, reviewable, reproducible, and safe to roll forward across environments.

Persistence error mapping also depends on stable PostgreSQL constraint names. Schema evolution therefore needs explicit control over constraint naming.

## Decision

Production schema evolution uses ordered, versioned SQL migrations stored in `/migrations`.

The initial convention is:

```text
migrations/
  000001_core.up.sql
  000001_core.down.sql
```

Future changes increment the numeric prefix and are applied in order.

The repository does not yet lock a migration runner. A deployment may use a compatible SQL migration tool, but the migration files in this repository are the production schema source of truth.

GORM `AutoMigrate` is limited to local development and automated tests through `AutoMigrateCoreForDevelopment`. Production application startup must not call it.

When application behavior maps a database error by constraint name, the relevant migration must define that constraint or index name explicitly and tests must protect the mapping.

## Consequences

### Positive

- production schema changes are reviewable and reproducible
- migration ordering is explicit
- constraint names remain stable
- deploys do not depend on implicit ORM diff behavior
- rollback or forward-fix planning is visible in code review

### Trade-offs

- schema changes require maintaining SQL alongside GORM mapping metadata
- contributors must keep ORM records and migrations consistent
- a concrete migration runner still needs to be selected when deployment automation is introduced

## Operational rule

Apply migrations as an explicit deployment step before starting application code that depends on the new schema. Do not edit a migration after it has been applied to a shared environment; create a new migration instead.

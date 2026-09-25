# ADR-001: Initial Technology Stack

- **Status:** Accepted
- **Date:** 2026-09-25

## Context

Oke Gaas needs an initial implementation stack that is productive for API development, suitable for self-hosting, and capable of growing into a high-throughput gamification backend without forcing distributed complexity from day one.

## Decision

Use:

```text
Go
Fiber v3
GORM
PostgreSQL
go-playground/validator
log/slog
Go testing + testify
OpenAPI / Swagger
OpenTelemetry
Docker
```

## Boundaries

Fiber is limited to transport/delivery concerns.

GORM is limited to persistence/infrastructure concerns.

Neither Fiber nor GORM should define the domain model.

The domain/application layers should remain independently testable.

Raw SQL is permitted where it is clearer or more appropriate than ORM chaining.

## Consequences

### Positive

- productive HTTP development
- straightforward self-hosting
- strong Go tooling
- simple single-binary deployment path
- mature PostgreSQL ecosystem
- fast development with ORM support
- room for explicit SQL where needed

### Trade-offs

- Fiber-specific behavior must be contained at the HTTP boundary
- GORM convenience can tempt persistence concerns into domain code
- explicit architectural discipline is required to preserve module boundaries

## Follow-up

Pin concrete dependency versions in implementation files.

Changes to this stack require updating `AGENTS.md` and recording a new ADR or superseding this one.

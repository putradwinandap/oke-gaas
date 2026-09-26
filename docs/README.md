# Oke Gaas Documentation

This directory contains durable technical documentation.

Authority remains split intentionally:

- `AGENTS.md` — authoritative engineering rules, architecture constraints, coding standards, testing, and workflow.
- `docs/architecture/` — detailed architectural explanations.
- `docs/decisions/` — Architecture Decision Records (ADRs).
- `docs/concepts/` — domain concepts and semantics.
- GitHub Issues — product direction, roadmap, acceptance criteria, and actionable implementation work.

The umbrella product-direction issue is [#1 — define Oke Gaas product direction and boundaries](https://github.com/putradwinandap/oke-gaas/issues/1).

## Architecture

- [Architecture Overview](./architecture/overview.md)

## Decisions

- [ADR-001: Initial Technology Stack](./decisions/001-initial-technology-stack.md)
- [ADR-002: Event-Driven Modular Monolith](./decisions/002-modular-monolith.md)
- [ADR-003: Versioned Database Migrations](./decisions/003-versioned-database-migrations.md)

## Concepts

- [Events](./concepts/events.md)
- [Projects and Players](./concepts/projects-and-players.md)
- [Rules and Rewards](./concepts/rules-and-rewards.md)

When an architectural or engineering rule changes, update `AGENTS.md` in the same logical change and update the relevant document here when deeper explanation is useful.

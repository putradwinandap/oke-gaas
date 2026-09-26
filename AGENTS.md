# AGENTS.md — Oke Gaas Engineering Source of Truth

> **Project:** Oke Gaas — Gamification as a Service  
> **Repository:** `putradwinandap/oke-gaas`  
> **Role of this file:** Authoritative engineering, architecture, consistency, and contribution rules for humans and AI agents.

## 1. Authority and Maintenance

This file is the engineering source of truth for Oke Gaas.

Every contributor and AI agent must read this file before making architectural, structural, or implementation changes.

When a change affects any of the following, update this file in the same logical change:

- architecture
- module boundaries
- domain terminology
- public API conventions
- repository structure
- coding standards
- testing requirements
- consistency rules
- deployment assumptions
- persistence strategy
- event-processing semantics
- engineering workflow

Do not allow implementation and this document to drift apart.

If code, tests, documentation, and this file disagree, resolve the inconsistency explicitly rather than silently choosing one.

Detailed explanations may live under `/docs`, especially:

```text
docs/
├── architecture/
├── decisions/
└── concepts/
```

Use ADR-style documents under `docs/decisions/` for significant architectural decisions. This file should retain the current authoritative rule or direction and link to detailed documentation when necessary.

Product direction, roadmap, and actionable implementation work live in GitHub Issues. Issue #1 is the umbrella product-direction issue; subsequent issues break the roadmap into implementable slices.

---

## 2. Product Engineering Context

Oke Gaas provides reusable gamification infrastructure.

The core conceptual flow is:

```text
Event -> Rule -> Reward -> Player State
```

Client applications own their business domain. Oke Gaas owns gamification behavior and state.

Canonical domain terminology:

- **Project** — tenant / integration boundary.
- **Player** — end user receiving gamification state and rewards.
- **Event** — something that happened in the integrating application.
- **Rule** — logic that evaluates events and/or state.
- **Reward** — outcome produced by a matching rule.
- **Reward Grant** — immutable/auditable record of a reward being granted.
- **Player State** — current materialized gamification state.
- **Domain Event** — meaningful event produced internally by Oke Gaas.

Use **Player**, not `User`, for the gamified end user.

Reserve **User** for Oke Gaas account users, administrators, operators, or dashboard members when such concepts are introduced.

---

## 3. Architecture Baseline

Oke Gaas starts as an:

```text
Event-Driven Modular Monolith
+ Lightweight Domain-Driven Design
+ Synchronous Processing First
+ PostgreSQL Preferred Initially
```

The initial architecture is intentionally not microservices.

### 3.1 Architectural dimensions

These concepts are not interchangeable:

- **Event-driven architecture** describes how behavior is initiated and propagated.
- **Modular monolith** describes the initial deployment model.
- **DDD** provides domain language and boundaries.
- **Distributed systems** arise when components communicate across processes, nodes, queues, or services.
- **Microservices** are a possible later deployment strategy, not an initial objective.

Guiding principle:

> Design strong boundaries now; distribute them later only when a concrete requirement justifies the cost.

### 3.2 Initial domain modules

Likely modules include:

```text
Project
Player
Event
Rule
Reward
Progression
Achievement
Leaderboard
Webhook
```

Not every module belongs in the MVP.

Modules must have explicit responsibilities and dependencies.

Do not create circular dependencies.

Do not bypass module boundaries by directly reaching into another module's persistence internals.

Prefer explicit interfaces, application services, or domain/application events between boundaries.

Current concrete module layout:

```text
internal/project/            Project domain + application service + repository interface
internal/player/             Player domain + application service + repository interface
internal/platform/database/  GORM/PostgreSQL records, queries, and migrations
```

Repository interfaces belong at the domain/application consumer boundary. GORM record types and query construction belong in infrastructure.

---

## 4. Initial Technology Stack

The initial implementation stack is locked to:

```text
Language              Go
HTTP framework        Fiber v3
ORM                   GORM
Database              PostgreSQL
Validation            go-playground/validator
Logging               log/slog
Testing               Go testing + testify
API documentation     OpenAPI / Swagger
Observability         OpenTelemetry
Containerization      Docker
```

Pin concrete dependency versions in implementation files such as `go.mod`; do not rely on floating "latest" behavior.

### 4.1 Framework boundary

Fiber is a delivery-layer concern.

Do not allow Fiber-specific types, request contexts, middleware contracts, or response objects to leak into the domain model.

Preferred dependency direction:

```text
Fiber Handler
     |
     v
Application Service
     |
     v
Domain
     |
     v
Repository Interface
     |
     v
GORM / PostgreSQL
```

Domain logic must remain testable without starting an HTTP server.

### 4.2 ORM boundary

GORM is a persistence/infrastructure detail, not the domain model.

Prefer keeping persistence mapping concerns separate from domain behavior when doing so protects the domain boundary.

Do not design aggregates around GORM convenience.

Do not expose `*gorm.DB` across domain/application boundaries.

Transactions should be owned by an application/infrastructure boundary with explicit intent.

### 4.3 ORM does not prohibit SQL

Use GORM when it keeps persistence simple and readable.

Raw SQL is allowed when it is clearer, safer, or materially better suited to the query.

Examples include:

- leaderboard ranking/window functions
- aggregate analytics
- performance-sensitive reporting
- complex joins that become harder to understand through ORM chaining

Do not write ORM gymnastics merely to avoid SQL.

### 4.4 Persistence migrations

Production schema evolution must use ordered, versioned SQL migrations under `/migrations`.

Rules:

- migration files are immutable after they have been applied to a shared environment
- use explicit constraint and index names when application error mapping depends on them
- schema changes must be reviewable independently from ORM model tags
- local development and tests may use `AutoMigrateCoreForDevelopment` for convenience
- production startup must not call GORM `AutoMigrate`
- production deployments must apply versioned migrations as an explicit deployment step before running code that depends on the new schema
- destructive or irreversible migrations require an explicit rollback/forward-fix plan

GORM model tags remain useful mapping metadata, but they are not the production migration source of truth.

See `docs/decisions/003-versioned-database-migrations.md`.

### 4.5 Go-specific baseline

All Go code must follow idiomatic Go conventions.

At minimum:

- format with `gofmt`
- use `go vet`
- keep package responsibilities narrow
- return and wrap errors intentionally
- avoid global mutable state
- pass `context.Context` explicitly where request/deadline/cancellation propagation is required
- keep exported APIs documented
- prefer standard-library solutions unless a dependency provides clear value
- avoid unnecessary interfaces; define interfaces at useful consumer boundaries
- keep constructors explicit
- avoid panic for ordinary runtime/application errors

Additional linting may be added later through an explicit repository decision.

See:

- `docs/architecture/overview.md`
- `docs/decisions/001-initial-technology-stack.md`
- `docs/decisions/002-modular-monolith.md`

---

## 5. Product Boundary

Keep these responsibilities conceptually distinct.

### Oke Gaas Core

Contains framework-agnostic domain logic and core gamification behavior.

It should not depend directly on React, Next.js, Laravel, WordPress, or another client framework.

### Oke Gaas Server / API

Responsible for application orchestration such as:

- HTTP/API transport
- authentication
- transaction boundaries
- persistence integration
- event ingestion
- calling the core domain
- exposing player state

### SDKs

SDKs are integration clients.

They should make Oke Gaas easy to consume without duplicating business logic that belongs in Core or the Server.

### Oke Gaas Cloud

The managed SaaS layer provides operational convenience such as:

- hosted infrastructure
- project/environment management
- dashboard
- API key management
- analytics
- monitoring
- backups
- scaling
- usage metering
- collaboration

The open-source core must remain genuinely useful without Oke Gaas Cloud.

---

## 6. Project and Tenant Isolation

`Project` is a security and data-isolation boundary.

Data owned by one project must not be readable or mutable by another project.

Domain entities that are project-scoped should carry or derive an unambiguous `project_id`.

This includes, where applicable:

- players
- events
- rules
- reward grants
- leaderboards
- achievements
- webhooks

Do not rely solely on caller-provided identifiers without enforcing project ownership.

Player repositories and application behavior must require Project scope for Player lookup. A Player external identifier is unique within a Project, not globally.

Tenant isolation must be covered by automated tests when persistence and authorization are implemented.

---

## 7. Event Model

### 7.1 External events

External/application events describe what happened in the integrating application.

Examples:

```text
lesson_completed
purchase_completed
article_published
daily_login
```

Canonical conceptual envelope:

```json
{
  "event_id": "evt_xxx",
  "type": "lesson_completed",
  "project_id": "proj_xxx",
  "player_id": "player_xxx",
  "occurred_at": "2026-09-25T10:00:00Z",
  "properties": {
    "lesson_id": "lesson_5"
  }
}
```

The server may record metadata such as `received_at`.

### 7.2 Domain events

Domain events describe meaningful outcomes inside Oke Gaas.

Examples:

```text
XpGranted
PlayerLeveledUp
AchievementUnlocked
StreakAdvanced
```

Keep external events and domain events conceptually distinct.

### 7.3 Event identity and idempotency

Stable event identity and idempotent ingestion are required from the initial vertical slice.

Retries must not accidentally grant the same logical reward twice.

Event identity supports:

- deduplication
- retry safety
- debugging
- auditability
- reward tracing
- abuse prevention

The exact public API shape is not yet permanently locked, but implementation must preserve these semantics.

---

## 8. Rule Model and Versioning

Rules must be version-aware.

Changing a rule must not rewrite the historical meaning of previously processed events.

Example:

```text
lesson_completed -> +100 XP   rule version 1
lesson_completed -> +50 XP    rule version 2
```

Historical reward records must be traceable to the specific rule version that produced them.

At minimum, reward history should be able to identify:

```text
rule_id
rule_version
```

Do not introduce a sophisticated DSL before concrete use cases require it.

Prefer the smallest representation that supports the current vertical slice.

---

## 9. Reward Ledger and Player State

Do not model rewards only as direct mutations such as:

```text
player.xp += 100
```

Maintain an auditable reward record.

Conceptual reward grant:

```text
RewardGrant
├── id
├── project_id
├── player_id
├── event_id
├── rule_id
├── rule_version
├── reward_type
├── amount / payload
├── created_at
└── reversed_at (optional)
```

The desired model is:

```text
Event
  |
  v
Rule Version
  |
  v
Reward Grant
  |
  v
Player State
```

`Player State` is a current materialized representation for efficient reads.

Reward history provides auditability.

This supports:

- debugging
- disputes
- analytics
- anti-cheat controls
- reversals
- historical explanation

---

## 10. Transaction Boundary

Initial event processing is synchronous.

Conceptually:

```text
receive event

BEGIN TRANSACTION

validate project/player ownership
persist or deduplicate event
evaluate applicable rules
create reward grants
update player state

COMMIT
```

Invariant:

> An accepted event must not leave reward history and player state in an unintentionally inconsistent partial state.

External side effects such as webhook delivery must not compromise the core transaction.

When asynchronous side effects are introduced, use an appropriate reliable-delivery pattern.

---

## 11. Event-Driven Does Not Mean Broker-Driven

Do not introduce Kafka, RabbitMQ, NATS, Redis Streams, or another broker merely because the architecture is event-driven.

V1 may remain synchronous:

```text
POST /events
     |
     v
validate + persist
     |
     v
evaluate rules
     |
     v
create reward grants
     |
     v
update player state
```

Introduce queues/workers only when justified by requirements such as:

- burst handling
- expensive workloads
- independent retries
- webhook delivery
- analytics fan-out
- operational isolation
- throughput limits

---

## 12. Event Sourcing Stance

Do not begin with full Event Sourcing.

Preserve the useful properties instead:

```text
append-oriented event history
+
reward ledger
+
current materialized player state
```

Full Event Sourcing requires an explicit architectural decision backed by concrete requirements.

---

## 13. Future Async / Distributed Evolution

Before splitting the system into services, prefer extracting asynchronous work only where it provides a clear benefit.

If reliable asynchronous publishing is introduced, consider the Transactional Outbox pattern:

```text
BEGIN

update player state
insert reward grant
insert outbox event

COMMIT

      |
      v
background worker
      |
      v
publish domain event
```

Microservices or independently deployed components should only be introduced when supported by evidence such as:

- independent scaling needs
- materially different workload characteristics
- stable domain boundaries
- different reliability requirements
- independent deployment cadence
- operational isolation
- different storage or compute needs

Do not split a module into a service merely because it has a name.

---

## 14. Privacy and Historical Data

The system intentionally keeps event and reward history for auditability.

Privacy/deletion behavior must therefore be designed explicitly.

Before implementing permanent player deletion, define behavior for:

- player state
- external event history
- reward grants
- analytics references
- personally identifying properties
- anonymization or pseudonymization
- legal/operational retention requirements

Do not silently hard-delete or retain historical data without an explicit documented policy.

---

## 15. Coding Standards

All production code must follow established international software engineering conventions for the selected language/runtime.

When a stack is chosen, adopt and automate the ecosystem-standard formatter, linter, static analysis, and compiler/type-checking rules where applicable.

Examples of expected practice:

- descriptive names
- clear and small units of behavior
- explicit error handling
- predictable control flow
- minimal hidden side effects
- safe resource handling
- consistent formatting
- type safety where the language supports it
- documented public interfaces
- no dead code
- no commented-out code committed as history
- no unexplained magic values
- no duplicated business rules across modules
- no swallowing errors without deliberate handling
- no secrets or credentials committed to the repository

Prefer readability and maintainability over cleverness.

Code should be understandable by an experienced engineer unfamiliar with the feature.

---

## 16. Engineering Principles

The following principles are mandatory defaults.

### SOLID

Apply SOLID where it improves boundaries and maintainability.

Do not turn SOLID into unnecessary interface proliferation or abstract every class/function prematurely.

### DRY — Don't Repeat Yourself

Avoid duplicating knowledge and business rules.

Do not eliminate harmless repetition if the abstraction would couple unrelated concepts.

### KISS — Keep It Simple

Choose the simplest design that correctly satisfies the current requirement.

Simple does not mean careless.

### YAGNI — You Aren't Gonna Need It

Do not build speculative features, abstractions, infrastructure, or scaling mechanisms without a current requirement.

### Separation of Concerns

Keep domain logic, transport, persistence, and external integrations appropriately separated.

### Explicit Over Implicit

Prefer code whose behavior, dependencies, mutations, and failure modes are easy to see.

### Composition Over Unnecessary Inheritance

Prefer composition when it produces clearer and more flexible designs.

### Fail Clearly

Errors should provide useful context and should not leave persistent state inconsistent.

---

## 17. Consistency Rules

Consistency is a project requirement.

Before introducing a new pattern, inspect the existing codebase.

Match existing conventions for:

- naming
- directory structure
- module boundaries
- API responses
- error shapes
- test organization
- dependency injection
- persistence access
- logging
- formatting
- documentation

Do not introduce a second style for the same problem without a documented reason.

If an existing convention is harmful, change it deliberately and consistently across the affected area rather than adding another competing pattern.

Whenever a consistency rule changes materially, update this file.

---

## 18. Testing Policy

Unit tests are mandatory.

A feature is not complete when only production code exists.

At minimum:

- new domain behavior requires unit tests
- bug fixes require a regression test whenever reasonably possible
- rules and reward calculations require deterministic tests
- idempotency requires tests
- transaction-sensitive behavior requires tests
- project/tenant isolation requires tests
- public API behavior should be covered at the appropriate integration level

Tests must verify behavior rather than merely execute lines.

Do not weaken, skip, or delete meaningful tests simply to make CI pass.

Use the testing pyramid pragmatically:

```text
many focused unit tests
some integration tests
few end-to-end tests
```

Prefer fast deterministic tests.

---

## 19. Definition of Done

A logical change is complete only when applicable items are satisfied:

- implementation follows current architecture and conventions
- unit tests are present and passing
- relevant integration tests are passing
- formatter/linter/type checks pass
- no known CI failure is introduced
- documentation is updated when behavior or architecture changed
- `AGENTS.md` is updated when architecture or consistency rules changed
- public API changes are intentional and documented
- migrations are safe and documented when persistence changes
- security and tenant-boundary implications are considered
- issue acceptance criteria/checklist is updated

Do not normalize red CI.

Fix failures or explicitly revert the offending change.

---

## 20. Git and Branch Workflow

Keep commits scoped to logical changes.

Commit messages should describe the logical change clearly.

Prefer short-lived branches.

Do not mix unrelated refactors, formatting changes, and feature behavior in one commit unless they are inseparable.

### 20.1 Manual review before CI

Pull-request CI must not consume runner time before a human/manual code review has completed.

Required sequence:

1. Open implementation pull requests as **Draft**.
2. Perform manual code review while the PR remains Draft.
3. Resolve review findings and push any required fixes.
4. When the reviewed revision is ready, mark the PR **Ready for review**.
5. That transition triggers CI for the reviewed revision.
6. Merge only after that CI run is green.

Repository automation must enforce this policy where practical:

- ordinary pull-request open, reopen, and synchronize events must not automatically start the full CI job
- the full PR CI should run on the `ready_for_review` transition
- a deliberate manual `workflow_dispatch` run is allowed as an explicit fallback
- pushes to the default branch may run CI as post-merge verification
- if new commits are pushed after a successful review/CI cycle, return the PR to Draft, review the new revision, then mark it Ready for review again before merge

Do not trigger expensive CI merely to discover problems that should have been caught during manual review. Review first, CI second.

Before merge:

- tests must pass
- CI must be green
- review feedback must be resolved
- relevant documentation must be current
- issue acceptance criteria must be satisfied

After a branch is merged:

1. Delete the merged branch if it is no longer needed.
2. Update/check off the related issue checklist.
3. Close the related issue when its acceptance criteria are fully satisfied.
4. If the issue is only partially satisfied, keep it open and clearly document remaining work.
5. Confirm the default branch remains green after merge.

Do not leave stale completed branches or completed issues open without a reason.

---

## 21. Documentation Rules

Documentation is part of the implementation.

Repository roles:

```text
AGENTS.md
    engineering source of truth, architecture, standards, consistency rules

docs/architecture/
    detailed architecture documentation

docs/decisions/
    ADRs / significant engineering decisions

docs/concepts/
    detailed domain concepts and semantics

GitHub Issues
    product direction, roadmap, and small actionable units of planned work
```

Do not allow the same authoritative engineering rule to diverge across multiple files.

When product detail becomes durable technical knowledge, move the technical explanation into `/docs` and keep the related issue focused on decisions, acceptance criteria, and work status.

---

## 22. Current Architecture Summary

When context is limited, preserve at least this:

> Oke Gaas uses Go with Fiber v3, GORM, and PostgreSQL in an event-driven modular monolith with lightweight DDD and synchronous processing first. The canonical domain flow is **Event -> Rule -> Reward -> Player State**. Use **Player** for gamified end users. Project is the tenant/isolation boundary. Events require stable identity and idempotent ingestion. Rewards must produce an auditable reward ledger and rules are version-aware. Preserve transactional consistency between event processing, reward grants, and player state. Do not introduce microservices, a message broker, or full Event Sourcing without a concrete requirement. Unit tests are mandatory. Maintain code/style consistency, update AGENTS.md whenever architecture or engineering consistency changes, and after merge delete completed branches and update/close the related issue.

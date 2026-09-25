# Oke Gaas — Gamification as a Service

> **Status:** Concept / Source of Truth  
> **Repository:** `putradwinandap/oke-gaas`  
> **Project name:** Oke Gaas  
> **Meaning:** Gamification as a Service (GaaS)

## 1. Overview

**Oke Gaas** is an open-source gamification library/core with an optional hosted SaaS.

The goal is simple:

> Let developers add gamification to Web2 applications without rebuilding XP, levels, achievements, streaks, leaderboards, challenges, rewards, and event processing from scratch.

Oke Gaas should work as infrastructure rather than as a collection of UI gimmicks.

A product should be able to send application events to Oke Gaas, define rules, and let Oke Gaas calculate and persist gamification state.

Example:

```ts
gaas.track("lesson_completed", {
  userId: "user_123",
  lessonId: "lesson_5",
});
```

A rule could conceptually say:

```text
WHEN lesson_completed
GIVE 100 XP

WHEN lesson_completed >= 10
UNLOCK "Rajin Belajar"

WHEN user is active for 7 consecutive days
UNLOCK "7 Day Streak"
```

The application owns its business domain. Oke Gaas owns the gamification logic.

---

## 2. Product Philosophy

Oke Gaas is planned as two complementary products.

### Oke Gaas Core

An open-source engine that can be used and self-hosted independently.

Core responsibilities may include:

- event ingestion
- rule evaluation
- rewards
- user gamification state
- XP
- levels
- achievements
- badges
- streaks
- leaderboards
- challenges / quests
- webhooks
- persistence interfaces

The open-source version should be genuinely useful by itself.

### Oke Gaas Cloud

A managed SaaS built on top of the same core concepts.

Possible SaaS value:

- hosted infrastructure
- managed database
- dashboard
- analytics
- project/environment management
- API keys
- team management
- monitoring
- backups
- scaling
- webhook management
- easier configuration
- usage metering

The SaaS should primarily sell **convenience and operations**, not artificially cripple the open-source core.

---

## 3. Core Abstraction

The initial architecture should stay small.

The four fundamental primitives are:

```text
Event -> Rule -> Reward -> User State
```

### Event

Something that happened in the client application.

Examples:

- `lesson_completed`
- `purchase_completed`
- `article_published`
- `daily_login`
- `comment_created`

Example:

```json
{
  "type": "lesson_completed",
  "userId": "user_123",
  "properties": {
    "lessonId": "lesson_5"
  }
}
```

### Rule

A condition that evaluates events and/or user state.

Conceptual example:

```text
WHEN event.type == "lesson_completed"
THEN give 100 XP
```

Rules should eventually be expressive enough to support:

- event conditions
- counters
- thresholds
- time windows
- consecutive activity
- user attributes
- accumulated state

But the first version should deliberately remain small.

### Reward

The result produced when a rule succeeds.

Examples:

- grant XP
- unlock badge
- unlock achievement
- increment progress
- change level
- issue a custom reward event

### User State

The gamification state that Oke Gaas maintains for a user.

Possible state:

```json
{
  "userId": "user_123",
  "xp": 1250,
  "level": 4,
  "badges": ["first-win"],
  "achievements": ["ten-lessons"],
  "streak": {
    "current": 7,
    "longest": 12
  }
}
```

---

## 4. Why These Primitives Matter

Features such as XP, achievements, badges, and streaks should not become isolated subsystems with unrelated logic.

Instead, they should preferably emerge from a common event/rule/reward model.

For example:

```text
lesson_completed
       |
       v
     Rule
       |
       +------> +100 XP
       |
       +------> increment lesson counter
       |
       +------> unlock badge after 10 lessons
```

This keeps the engine extensible.

A future product may need a gamification mechanic that was never anticipated by the original implementation. A generic rules/rewards foundation should make that possible without rewriting the architecture.

---

## 5. High-Level Architecture

```text
+--------------------------+
|     Client Application   |
| Web / Mobile / Backend   |
+------------+-------------+
             |
             | SDK / REST API
             v
+--------------------------+
|        Oke Gaas API      |
+------------+-------------+
             |
             v
+--------------------------+
|      Event Processor     |
+------------+-------------+
             |
             v
+--------------------------+
|       Rules Engine       |
+------------+-------------+
             |
      +------+------+
      |             |
      v             v
+-----------+  +-------------+
| Rewards   |  | User State  |
+-----------+  +-------------+
      |             |
      +------+------+
             |
             v
+--------------------------+
|        Database          |
+--------------------------+

Optional:
- Dashboard
- Analytics
- Webhooks
- Leaderboards
- Admin tooling
```

---

## 6. Developer Experience

The developer experience is a major part of the product.

A developer integrating Oke Gaas should ideally need only a few concepts.

Example client setup:

```ts
import { createGaas } from "@oke-gaas/sdk";

const gaas = createGaas({
  apiKey: process.env.OKE_GAAS_API_KEY,
});
```

Track an event:

```ts
await gaas.track("lesson_completed", {
  userId: "user_123",
  properties: {
    lessonId: "lesson_5",
  },
});
```

Read player state:

```ts
const player = await gaas.players.get("user_123");

console.log(player.xp);
console.log(player.level);
console.log(player.badges);
```

The SDK API shown here is conceptual and is **not yet a locked public API**.

---

## 7. Open Source and SaaS Boundary

A guiding principle:

> Open source should contain the real gamification engine. Cloud should make running it easier.

Possible split:

| Capability | Core / Self-hosted | Cloud |
|---|---:|---:|
| Event API | Yes | Yes |
| Rules engine | Yes | Yes |
| XP / levels | Yes | Yes |
| Achievements | Yes | Yes |
| Badges | Yes | Yes |
| Streaks | Yes | Yes |
| Leaderboards | Yes | Yes |
| REST API | Yes | Yes |
| SDKs | Yes | Yes |
| Self-hosting | Yes | N/A |
| Managed infrastructure | No | Yes |
| Managed backups | No | Yes |
| Hosted dashboard | Optional/basic | Yes |
| Team management | No / basic | Yes |
| Usage analytics | Basic | Yes |
| Scaling / operations | User-owned | Managed |

This boundary is provisional and may evolve.

---

## 8. Initial MVP

The MVP should prove one complete vertical slice rather than implementing every gamification feature.

### MVP Flow

```text
Application
    |
    | send event
    v
Oke Gaas
    |
    | evaluate rule
    v
Reward
    |
    | update
    v
User State
    |
    v
Application reads result
```

A valid first vertical slice could be:

1. Create a project.
2. Create/register a user.
3. Define one event.
4. Define one simple rule.
5. Send the event.
6. Match the rule.
7. Grant XP.
8. Persist XP.
9. Retrieve the user's XP.
10. Cover the flow with automated tests.

Example:

```text
lesson_completed -> +100 XP -> user total becomes 300 XP
```

If this works reliably through a public API, the core architecture has proven itself.

---

## 9. MVP Scope

### In scope

- projects
- API authentication
- users / players
- events with stable identity
- idempotent event ingestion
- simple rules
- XP reward
- reward grant / audit history
- persisted user state
- REST API
- initial JavaScript/TypeScript SDK
- automated tests
- local/self-hosted development setup
- documentation
- one example integration

### Prefer to defer

- advanced visual rule builder
- marketplace
- dozens of SDK languages
- complex quest graphs
- social feeds
- notifications platform
- AI-generated gamification
- elaborate billing
- complex organization permissions
- highly customizable white-label UI

The project should avoid becoming a giant engagement platform before the core engine is proven.

---

## 10. Possible Repository Structure

This is a working proposal, not a locked decision.

```text
oke-gaas/
├── apps/
│   ├── dashboard/
│   └── docs/
│
├── packages/
│   ├── core/
│   ├── sdk-js/
│   └── shared/
│
├── services/
│   └── api/
│
├── examples/
│   └── basic-web/
│
├── docs/
│   ├── architecture/
│   ├── decisions/
│   └── concepts/
│
├── tests/
├── README.md
└── PROJECT.md
```

The final structure should follow the actual implementation language and deployment model rather than forcing a monorepo prematurely.

---

## 11. Design Principles

### 11.1 Event-driven first

Client applications should describe **what happened**, not implement gamification calculations themselves.

Prefer:

```ts
gaas.track("purchase_completed", ...);
```

over:

```ts
gaas.addXp(500);
gaas.incrementPurchaseBadge();
gaas.updateLeaderboard();
```

The latter tightly couples the application to gamification internals.

### 11.2 Server-authoritative state

Important gamification state should not rely solely on browser storage.

The server/core should remain authoritative for XP, achievements, rewards, and leaderboard state.

### 11.3 Idempotency

Event APIs should support idempotency from the initial vertical slice so retries do not accidentally grant rewards twice.

Example concern:

```text
payment_completed sent twice
!=
reward granted twice
```

### 11.4 Auditable rewards

Ideally, rewards can be traced back to the event and rule that produced them.

This will help with:

- debugging
- disputes
- analytics
- rollback
- fraud prevention

### 11.5 Framework agnostic

The core should not depend on React, Next.js, Laravel, WordPress, or another application framework.

Framework adapters can exist separately.

### 11.6 Simple before clever

Do not start with a highly sophisticated DSL or distributed architecture.

Prove the primitive flow first.

---

## 12. Initial Architecture Decision

Oke Gaas should begin as an **event-driven modular monolith**, using **lightweight Domain-Driven Design (DDD)** to keep domain boundaries explicit.

The initial architecture is intentionally **not microservices**.

These terms describe different architectural dimensions and should not be treated as mutually exclusive choices:

- **Event-driven architecture** describes how important application behavior is initiated and propagated through events.
- **Modular monolith** describes the initial deployment architecture: one deployable application with clear internal module boundaries.
- **DDD** provides language and domain boundaries, not a requirement to use every enterprise pattern.
- **Distributed systems** become relevant when components run across independent processes, nodes, queues, or services.
- **Microservices** are a possible future deployment evolution when independently deployable boundaries are justified by real operational or scaling needs.

Initial direction:

```text
Oke Gaas
├── Architectural style
│   └── Event-driven
├── Deployment model
│   └── Modular monolith
├── Domain modeling
│   └── Lightweight DDD
├── Persistence
│   └── PostgreSQL preferred initially
├── Processing
│   └── Synchronous first
└── Evolution
    └── Async / distributed components only when justified
```

The guiding rule is:

> Design strong boundaries now; distribute them later only if there is a concrete reason.

### 12.1 Initial module boundaries

Likely domain modules include:

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

Not every module must exist in the MVP.

Dependencies should remain intentional. A conceptual direction is:

```text
External Event
      |
      v
    Event
      |
      v
     Rule
      |
      v
    Reward
      |
      v
 Player State

Domain Events
   |
   +--> Leaderboard
   +--> Webhook
   +--> Analytics
```

Modules should communicate through explicit interfaces or domain/application events rather than reaching arbitrarily into each other's persistence internals.

Avoid circular dependencies such as:

```text
Leaderboard -> Achievement -> Player -> Rule -> Leaderboard
```

### 12.2 External events vs domain events

Oke Gaas should distinguish events received from client applications from events produced internally by the gamification domain.

An **external/application event** describes something that happened in the integrating application:

```text
lesson_completed
purchase_completed
article_published
daily_login
```

A **domain event** describes something meaningful that happened inside Oke Gaas after processing:

```text
XpGranted
PlayerLeveledUp
AchievementUnlocked
StreakAdvanced
```

Conceptually:

```text
lesson_completed
       |
       v
   Rule Engine
       |
       v
      Reward
       |
       +--> XpGranted
       +--> PlayerLeveledUp
       +--> AchievementUnlocked
```

This separation allows future features such as webhooks, analytics, leaderboards, notifications, and asynchronous processing to react to domain outcomes without coupling themselves directly to rule evaluation.

### 12.3 Event identity and idempotency

Event identity is a first-class concern and should be included early rather than treated only as a later optimization.

A conceptual event envelope:

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

The system may also record server-generated metadata such as `received_at`.

A stable event identity supports:

- retry safety
- deduplication
- debugging
- auditability
- reward tracing
- abuse prevention

For externally retried requests, the API should support an idempotency mechanism so the same logical event cannot accidentally grant the same reward multiple times.

### 12.4 Reward ledger

Rewards should not exist only as mutations such as:

```text
player.xp += 100
```

Oke Gaas should preserve an auditable reward record.

Conceptually:

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

This creates a trace:

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

A reward ledger supports:

- debugging
- disputes
- analytics
- anti-cheat controls
- reward reversals
- reconstruction of why a player has a particular state

The current player state can remain a fast materialized representation while reward history preserves the audit trail.

### 12.5 Rule versioning

Rules should be version-aware.

For example:

```text
lesson_completed -> +100 XP   (version 1)
lesson_completed -> +50 XP    (version 2)
```

Historical rewards must remain traceable to the rule definition that produced them.

A reward grant should therefore be able to reference both:

```text
rule_id
rule_version
```

Changing a rule should not rewrite the historical meaning of previously processed events.

### 12.6 Transaction boundary

For the initial synchronous implementation, processing one event should have a clear transaction boundary.

Conceptually:

```text
receive event

BEGIN TRANSACTION

persist / deduplicate event
evaluate applicable rules
create reward grants
update player state

COMMIT
```

The exact implementation may evolve, but the invariant is important:

> An accepted event must not leave reward history and player state in an unintentionally inconsistent partial state.

Side effects that cannot safely participate in the same database transaction, such as external webhooks, should eventually be decoupled.

### 12.7 Event-driven does not require a message broker

The first implementation does **not** need Kafka, RabbitMQ, NATS, Redis Streams, or another broker merely to qualify as event-driven.

V1 may process events synchronously:

```text
POST /events
     |
     v
validate + persist event
     |
     v
evaluate rules
     |
     v
apply rewards
     |
     v
persist player state
```

A queue or worker should be introduced only when there is a concrete requirement such as:

- expensive processing
- burst handling
- independent retries
- webhook delivery
- analytics fan-out
- isolation of slow workloads
- throughput requirements that synchronous processing cannot meet

### 12.8 Event sourcing stance

Oke Gaas should **not begin with full Event Sourcing**.

However, it should preserve useful event-sourcing properties:

```text
append-oriented event history
+
reward ledger
+
current materialized player state
```

Conceptually:

```text
events
reward_grants
player_state
```

This provides strong auditability without requiring the entire application state to be reconstructed from an event stream in v1.

Full Event Sourcing should only be adopted later if concrete requirements justify its operational and modeling complexity.

### 12.9 Future asynchronous evolution and Outbox Pattern

If domain events later need to be delivered asynchronously, the system should consider a **Transactional Outbox** before splitting into distributed services.

Problem:

```text
database commit succeeds
message publish fails
```

Possible future flow:

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

This is intentionally a future concern, not an MVP requirement.

### 12.10 Microservices and distributed systems

Microservices are not an initial goal.

Possible future extraction may look like:

```text
Modular Monolith
      |
      +--> extract only where justified
                |
                +--> Leaderboard Service
                +--> Webhook Worker
                +--> Analytics Pipeline
```

Extraction should be driven by evidence such as:

- independent scaling needs
- different reliability characteristics
- clearly stable domain boundaries
- separate deployment cadence
- operational isolation
- materially different storage or compute needs

Do not split a module into a service merely because it has a name.

A distributed architecture introduces additional concerns such as:

- network failures
- retries and duplicate delivery
- event ordering
- schema compatibility
- eventual consistency
- distributed observability
- deployment coordination
- cross-service authorization
- failure recovery

Those costs should only be accepted when the benefits are concrete.

---

## 13. Example Use Cases

Oke Gaas could serve products such as:

### Education

- XP for completing lessons
- course streaks
- learning achievements
- leaderboard for cohorts

### E-commerce

- loyalty points
- purchase milestones
- referral rewards
- customer tiers

### Community

- reputation
- contribution badges
- posting streaks
- community levels

### Productivity

- completion XP
- streaks
- milestones
- weekly challenges

### SaaS products

- onboarding progress
- feature adoption achievements
- usage milestones
- referral mechanics

Oke Gaas should provide primitives for these patterns rather than hard-code one industry.

---

## 14. Possible API Model

Conceptual only:

```http
POST /v1/events
GET  /v1/players/:id
GET  /v1/players/:id/rewards

POST /v1/rules
GET  /v1/rules
PATCH /v1/rules/:id

GET /v1/leaderboards/:id
```

Event example:

```json
POST /v1/events

{
  "event": "lesson_completed",
  "user_id": "user_123",
  "idempotency_key": "lesson-5-user-123",
  "properties": {
    "lesson_id": "lesson_5"
  }
}
```

Possible response:

```json
{
  "accepted": true,
  "rewards": [
    {
      "type": "xp",
      "amount": 100
    }
  ]
}
```

Again, this API is exploratory and not a compatibility commitment.

---

## 15. Rules Engine Evolution

A phased approach is preferred.

### Phase 1 — Exact event rules

```text
WHEN lesson_completed
GIVE 100 XP
```

### Phase 2 — Property conditions

```text
WHEN purchase_completed
AND amount >= 100000
GIVE 50 XP
```

### Phase 3 — Aggregates

```text
WHEN lesson_completed COUNT >= 10
UNLOCK achievement
```

### Phase 4 — Time-aware rules

```text
WHEN daily_login occurs 7 consecutive days
UNLOCK streak reward
```

### Phase 5 — Advanced compositions

Possible later concepts:

- AND / OR conditions
- reusable predicates
- rolling windows
- scheduled evaluation
- rule versions
- rule priorities
- reward limits

Complexity should be introduced only when real use cases demand it.

---

## 16. Future Technical Concerns

These do not all belong in the first implementation, but should be kept visible.

- event idempotency
- concurrency
- duplicate delivery
- transaction boundaries
- rule versioning
- event ordering
- async processing
- retry policy
- webhook delivery
- rate limiting
- project isolation
- multi-tenancy
- caching
- leaderboard performance
- anti-cheat / abuse controls
- audit logs
- reward reversals
- privacy / deletion workflows
- observability

---

## 17. SaaS Concerns

Cloud-specific capabilities may eventually include:

### Projects and environments

```text
Project
├── development
├── staging
└── production
```

### API keys

Keys should be scoped appropriately.

Potential distinction:

- server secret key
- public/client identifier
- webhook secret

Sensitive operations should never depend on a secret embedded in frontend code.

### Usage

Possible metered units:

- events processed
- monthly active players
- API calls
- rule executions

Pricing is deliberately **not decided yet**.

Do not design the core around a pricing model that has not been validated.

---

## 18. Naming and Brand

Project:

# Oke Gaas

Expansion:

> **Gamification as a Service**

The name intentionally plays on the Indonesian expression **"Oke, gaas!"**, making it memorable while still mapping naturally to the acronym **GaaS**.

Possible professional tagline:

> **Gamification infrastructure for developers.**

Alternative:

> **Add gamification to your product without building the engine from scratch.**

The playful brand can coexist with a serious developer-focused product.

---

## 19. Project Rules

Until changed by an explicit project decision:

1. **The repository is the source of truth.**
2. Architectural decisions should be documented.
3. Prefer vertical slices over large disconnected foundations.
4. Keep the initial domain model small.
5. New abstractions must solve a concrete problem.
6. Core should remain useful without Oke Gaas Cloud.
7. Avoid unnecessary vendor lock-in.
8. Public APIs should not be considered stable until intentionally versioned.
9. Automated tests are part of a feature, not optional cleanup.
10. CI failures should be fixed rather than normalized.
11. Documentation must evolve together with architecture.
12. Significant changes to the vision should update this file.

---

## 20. Recommended Development Strategy

Build one end-to-end path first:

```text
SDK
 |
 v
POST /events
 |
 v
event validation
 |
 v
rule evaluation
 |
 v
XP reward
 |
 v
database transaction
 |
 v
GET /players/:id
 |
 v
updated XP returned
```

This vertical slice should include:

- domain model
- persistence
- API
- tests
- SDK call
- example
- documentation

Only after this path is clean should additional game mechanics be layered on top.

---

## 21. Initial Roadmap

### Phase 0 — Foundation

- define project vision
- document architecture
- select implementation stack
- decide license
- define contribution workflow
- establish CI

### Phase 1 — Core vertical slice

- project model
- player model
- event model and stable event identity
- idempotent event ingestion
- rule model with version-aware design
- XP reward
- reward grant / audit history
- persisted player state
- REST endpoint
- SDK
- tests

### Phase 2 — First real mechanics

- levels
- achievements
- badges
- counters
- basic streaks

### Phase 3 — Operational maturity

- asynchronous processing where justified
- transactional outbox if asynchronous publishing is introduced
- webhooks
- rate limiting
- observability
- better error handling

### Phase 4 — Self-hosted experience

- Docker setup
- configuration
- migration workflow
- deployment documentation
- backup/restore guidance

### Phase 5 — Cloud

- hosted projects
- authentication
- dashboard
- managed API keys
- analytics
- usage metering
- billing

This roadmap is directional, not a commitment to exact implementation order.

---

## 22. Open Questions

Important decisions still intentionally unresolved:

- What implementation language/runtime should power the modular monolith?
- PostgreSQL is the preferred initial persistence layer; are there implementation constraints that would invalidate it?
- Which workloads, if any, justify asynchronous processing after the synchronous v1 is proven?
- How should rules be represented internally?
- Should the rules API start as JSON configuration or code?
- Which open-source license should be used?
- How much UI belongs in the open-source project?
- What is the exact boundary between Core and Cloud?
- What is the first real application used to dogfood Oke Gaas?

These should be answered through small architectural decisions rather than assumptions buried in code.

---

## 23. Context for AI Agents and Future Chats

If an AI assistant is asked to work on this repository, treat this document as project context.

### Project identity

- Name: **Oke Gaas**
- Meaning: **Gamification as a Service**
- Repository: **putradwinandap/oke-gaas**
- Model: **open-source core + optional hosted SaaS**
- Primary audience: **developers adding gamification to Web2 applications**

### Core concept

```text
Event -> Rule -> Reward -> User State
```

### Primary goal

Make gamification infrastructure reusable so application developers do not need to repeatedly build systems such as XP, levels, achievements, badges, streaks, leaderboards, and challenges from scratch.

### Architecture preference

Start with a small vertical slice implemented as an **event-driven modular monolith** with **lightweight DDD**.

Use synchronous processing first. Keep module boundaries explicit so individual workloads can be extracted later if real scaling or operational requirements justify distribution.

Do **not** prematurely implement every mechanic, microservices architecture, message broker, full Event Sourcing, complex DSL, or large admin dashboard.

### Product boundary

The open-source core must remain useful independently.

The SaaS should primarily provide managed infrastructure, operations, analytics, collaboration, and convenience.

### Source-of-truth rule

When future discussion changes a major project decision, update repository documentation so later AI sessions can recover context from the repo rather than depending on chat memory.

---

## 24. Current State

At the time this document was created:

- repository has been initialized
- product concept exists
- no implementation language/runtime has been locked
- no public API has been locked
- PostgreSQL is the preferred initial persistence layer, subject to implementation validation
- initial deployment direction is an event-driven modular monolith
- initial processing direction is synchronous-first
- lightweight DDD will be used to keep domain boundaries explicit
- full Event Sourcing and microservices are intentionally deferred
- no SaaS pricing has been decided
- implementation details are still intentionally small and evolving

The next useful step is to turn the concept into the smallest testable technical architecture and vertical slice.

---

## 25. Short Version

If only a few lines of context can be loaded:

> **Oke Gaas** is an open-source Gamification as a Service engine plus an optional hosted SaaS. Developers send application events; Oke Gaas evaluates rules, grants auditable rewards, and maintains user gamification state. The foundational model is **Event -> Rule -> Reward -> User State**. Start as an **event-driven modular monolith** with **lightweight DDD**, **synchronous processing**, stable event identity/idempotency, version-aware rules, a reward ledger, and PostgreSQL as the preferred initial persistence layer. Keep append-oriented event history plus materialized player state; do not begin with microservices, a message broker, or full Event Sourcing. Extract asynchronous/distributed components only when concrete requirements justify them. The OSS core must remain genuinely usable independently; the SaaS primarily sells managed operations and convenience.

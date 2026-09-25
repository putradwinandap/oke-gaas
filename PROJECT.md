# Oke Gaas — Gamification as a Service

> **Status:** Concept / Product Direction  
> **Repository:** `putradwinandap/oke-gaas`  
> **Engineering source of truth:** [AGENTS.md](./AGENTS.md)

## 1. Overview

**Oke Gaas** is open-source gamification infrastructure with an optional hosted SaaS.

The goal is simple:

> Let developers add gamification to Web2 applications without rebuilding XP, levels, achievements, streaks, leaderboards, challenges, rewards, and event processing from scratch.

Oke Gaas should behave as infrastructure rather than a collection of UI gimmicks.

Applications describe what happened. Oke Gaas evaluates configured gamification rules, grants rewards, and maintains player gamification state.

Conceptually:

```text
Event -> Rule -> Reward -> Player State
```

The application owns its business domain. Oke Gaas owns the gamification logic.

For current architecture, engineering rules, terminology, testing requirements, and contribution workflow, see **AGENTS.md**.

---

## 2. Product Model

Oke Gaas is planned as two complementary products.

### Oke Gaas Open Source

The open-source product should contain the real gamification engine and remain genuinely useful when self-hosted.

Capabilities may include:

- event ingestion
- rules
- rewards
- player state
- XP
- levels
- achievements
- badges
- streaks
- leaderboards
- challenges / quests
- webhooks
- REST API
- SDKs
- self-hosting

### Oke Gaas Cloud

A managed SaaS built on the same product concepts.

Cloud value may include:

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
- usage metering
- billing

Guiding principle:

> Open source contains the real product. Cloud primarily sells convenience, operations, collaboration, and managed infrastructure.

---

## 3. Core Product Concepts

### Event

Something that happened in the integrating application.

Examples:

- `lesson_completed`
- `purchase_completed`
- `article_published`
- `daily_login`

### Rule

A condition that evaluates events and/or gamification state.

Example:

```text
WHEN lesson_completed
GIVE 100 XP
```

### Reward

An outcome produced by a matching rule.

Examples:

- grant XP
- unlock a badge
- unlock an achievement
- increment progress
- change a level
- emit a custom reward/domain event

### Player State

The current gamification state for a player.

Possible state includes:

- XP
- level
- badges
- achievements
- streaks
- progress

The canonical domain term is **Player**, not User, for the gamified end user.

Detailed semantics belong in `AGENTS.md` and eventually `docs/concepts/`.

---

## 4. Developer Experience

Developer experience is a primary product concern.

A developer should need only a small number of concepts to integrate Oke Gaas.

Conceptual SDK usage:

```ts
import { createGaas } from "@oke-gaas/sdk";

const gaas = createGaas({
  apiKey: process.env.OKE_GAAS_API_KEY,
});

await gaas.track("lesson_completed", {
  playerId: "player_123",
  properties: {
    lessonId: "lesson_5",
  },
});

const player = await gaas.players.get("player_123");
```

The public API is not yet a compatibility commitment.

The API should remain small, predictable, framework-agnostic, and easy to integrate.

---

## 5. Open Source and Cloud Boundary

| Capability | Open Source / Self-hosted | Cloud |
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

This boundary is provisional and can evolve as real usage is discovered.

---

## 6. Initial MVP

The MVP should prove one complete vertical slice instead of implementing every mechanic.

Target flow:

```text
Application
    |
    | event
    v
Oke Gaas
    |
    | evaluate rule
    v
Reward
    |
    | update
    v
Player State
    |
    v
Application reads result
```

A valid first vertical slice:

1. Create a project.
2. Create/register a player.
3. Define one event type/use case.
4. Define one simple rule.
5. Send an event with stable identity.
6. Process the event idempotently.
7. Match the rule.
8. Grant XP.
9. Persist reward history and player state.
10. Retrieve the player's XP.
11. Cover the flow with automated tests.

Example:

```text
lesson_completed -> +100 XP -> player total becomes 300 XP
```

If this works reliably through a public API, the first core product slice is proven.

---

## 7. MVP Scope

### In scope

- projects
- API authentication
- players
- events with stable identity
- idempotent event ingestion
- simple rules
- XP reward
- reward/audit history
- persisted player state
- REST API
- initial JavaScript/TypeScript SDK
- automated tests
- local/self-hosted development setup
- documentation
- one example integration

### Prefer to defer

- advanced visual rule builder
- marketplace
- many SDK languages
- complex quest graphs
- social feeds
- notifications platform
- AI-generated gamification
- elaborate billing
- complex organization permissions
- highly customizable white-label UI

Do not let Oke Gaas become a giant engagement platform before the engine is proven.

---

## 8. Example Use Cases

### Education

- XP for completing lessons
- course streaks
- learning achievements
- cohort leaderboards

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

Oke Gaas should provide reusable primitives for these patterns instead of hard-coding one industry.

---

## 9. Rules Engine Product Evolution

The rules engine should grow only when real use cases require additional expressiveness.

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
- rule priorities
- reward limits

Architecture and implementation details for rules belong in `AGENTS.md` and `docs/`.

---

## 10. SaaS Concerns

Cloud may eventually introduce project environments such as:

```text
Project
├── development
├── staging
└── production
```

Possible key types:

- server secret key
- public/client identifier
- webhook secret

Sensitive operations must not depend on secrets embedded in frontend code.

Possible metered units:

- events processed
- monthly active players
- API calls
- rule executions

Pricing is deliberately not decided yet.

Do not design the core around an unvalidated pricing model.

---

## 11. Naming and Brand

Project name:

# Oke Gaas

Expansion:

> **Gamification as a Service**

The name plays on the Indonesian expression **"Oke, gaas!"** while mapping naturally to **GaaS**.

Possible professional tagline:

> **Gamification infrastructure for developers.**

Alternative:

> **Add gamification to your product without building the engine from scratch.**

The playful brand can coexist with a serious developer-focused product.

---

## 12. Product Principles

1. The open-source product must be genuinely useful independently.
2. Prefer a complete vertical slice over broad disconnected foundations.
3. Keep the initial product model small.
4. Add complexity only when it solves a concrete problem.
5. Developer experience is part of the product.
6. Avoid unnecessary vendor lock-in.
7. Public APIs are not stable until intentionally versioned.
8. Do not optimize the product around an unvalidated SaaS pricing model.
9. Documentation must evolve with product decisions.
10. Engineering and architecture rules belong in `AGENTS.md`, not duplicated here.

---

## 13. Initial Roadmap

### Phase 0 — Foundation

- define product vision
- establish engineering rules
- select implementation stack
- decide license
- define contribution workflow
- establish CI
- establish initial `/docs` structure

### Phase 1 — Core vertical slice

- project model
- player model
- event ingestion
- simple rule
- XP reward
- reward history
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

- webhooks
- rate limiting
- observability
- stronger error handling
- asynchronous processing only where justified

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

The roadmap is directional, not a commitment to exact implementation order.

---

## 14. Open Product Questions

Still intentionally unresolved:

- Which implementation language/runtime should power the first release?
- What is the first real application used to dogfood Oke Gaas?
- How much UI belongs in the open-source distribution?
- What is the long-term boundary between open-source packaging and Cloud?
- Which open-source license should be used?
- Which initial gamification use case best validates the product?
- How should pricing eventually map to real customer value?

Engineering questions and architectural decisions should be recorded in `AGENTS.md` and/or `docs/decisions/`.

---

## 15. Documentation Evolution

`PROJECT.md` is intentionally not the engineering source of truth.

The intended repository documentation model is:

```text
PROJECT.md
    product vision, scope, use cases, roadmap

AGENTS.md
    engineering source of truth

docs/architecture/
    detailed architecture

docs/decisions/
    architectural decision records

docs/concepts/
    domain concept documentation

GitHub Issues
    small, actionable implementation work
```

As implementation starts, larger concepts currently summarized here may be decomposed into focused GitHub Issues and durable `/docs` pages.

Avoid duplicating the same authoritative rule in multiple documents.

---

## 16. Current State

At this stage:

- repository is initialized
- product direction is defined
- initial MVP is defined
- engineering architecture has a baseline in `AGENTS.md`
- implementation language/runtime is not yet locked
- public API is not yet locked
- SaaS pricing is not decided
- implementation has not yet been built in detail

The next useful step is to select the initial stack and turn the MVP into small implementation issues.

---

## 17. Short Version

> **Oke Gaas** is open-source Gamification as a Service infrastructure plus an optional hosted SaaS. Developers send application events; Oke Gaas evaluates rules, grants rewards, and maintains player gamification state. The foundational product model is **Event -> Rule -> Reward -> Player State**. Start with one complete vertical slice before adding advanced mechanics. The open-source product must remain genuinely useful independently, while Cloud primarily sells managed infrastructure and convenience. **PROJECT.md describes the product; AGENTS.md is the authoritative source for architecture, coding standards, testing, consistency, and engineering workflow.**

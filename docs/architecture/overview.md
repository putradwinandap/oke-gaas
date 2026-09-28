# Architecture Overview

## Status

Initial architecture baseline for Oke Gaas v1.

The authoritative engineering rules live in [AGENTS.md](../../AGENTS.md).

## Architectural Shape

Oke Gaas begins as an **event-driven modular monolith** with **lightweight DDD** and **synchronous processing first**.

```text
Client Application
       |
       | REST / SDK
       v
+--------------------------+
|      Fiber HTTP Layer    |
+------------+-------------+
             |
             v
+--------------------------+
|    Application Layer     |
| orchestration/transactions|
+------------+-------------+
             |
             v
+--------------------------+
|       Domain Modules     |
| Project / Player / Event |
| Rule / Reward / Counter / Badge |
+------------+-------------+
             |
             v
+--------------------------+
| Persistence Interfaces   |
+------------+-------------+
             |
             v
+--------------------------+
|    GORM + PostgreSQL     |
+--------------------------+
```

Fiber and GORM are adapters around the application/domain, not the domain itself.

## Core Processing Flow

```text
External Event
      |
      v
Validate tenant/player
      |
      v
Deduplicate event
      |
      v
Evaluate latest rule version
      |
      v
Match exact Event type + optional top-level property conditions
      |
      v
Create XP or Badge Reward Grant
      |
      v
Update matching Player Counters
      |
      v
Update Player State
      |
      v
Commit
```

The initial transaction should keep accepted Event processing, reward history, unique Badge ownership, Player Counter progress, Achievement unlocks, and Player State consistent.

## Domain Events

Internal outcomes may emit domain events such as:

```text
XpGranted
PlayerLeveledUp
AchievementUnlocked
StreakAdvanced
```

These are distinct from external application events.

Synchronous in-process handling is acceptable initially. A message broker is not required.

## Persistence

PostgreSQL is the initial database.

GORM is the default ORM, but raw SQL remains valid when it is clearer or better suited to a query.

Important persistence properties:

- project isolation
- stable event identity
- idempotent ingestion
- version-aware rules with optional exact top-level Event property conditions
- auditable reward grants
- immutable Project Badges with unique Player ownership and Rule-version audit
- materialized player state
- Project-defined Player Counters, updated atomically with accepted Event processing

## Evolution

Do not start with microservices or full Event Sourcing.

Possible later evolution:

```text
Modular Monolith
      |
      +--> async webhook worker
      +--> analytics pipeline
      +--> leaderboard workload
```

When asynchronous reliable publication becomes necessary, consider a Transactional Outbox before introducing broader distributed complexity.

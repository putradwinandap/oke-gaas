# Events

## External Events

An external event describes something that happened in an integrating application.

Examples:

```text
lesson_completed
purchase_completed
daily_login
comment_created
```

Conceptual envelope:

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

The server may add metadata such as `received_at`.

## Identity and Idempotency

Event identity is first-class.

Event identity is scoped to a Project.

A retry using the same Project + Event ID and the same logical payload is treated as an idempotent duplicate and resolves to the originally persisted Event. Reusing the same identity with different Player, type, occurrence time, or properties is rejected as an identity conflict. Properties are normalized through JSON before identity comparison, and timestamps are normalized to PostgreSQL microsecond precision, so a database round-trip cannot turn an otherwise identical retry into a false conflict.

Player ownership is validated within the Event's Project before persistence, and the database enforces the same Project/Player relationship.

Retries must not grant the same logical reward twice.

Idempotency is part of the initial vertical slice, not an optional later optimization.

## Domain Events

Domain events represent meaningful internal outcomes.

Examples:

```text
XpGranted
PlayerLeveledUp
AchievementUnlocked
StreakAdvanced
```

External events and domain events must remain conceptually distinct.

## Ordering

Do not assume global event ordering unless a concrete requirement establishes it.

Ordering requirements should be defined per use case or aggregate when necessary.

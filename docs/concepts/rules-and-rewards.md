# Rules and Rewards

## Rule

A Rule evaluates an event and/or gamification state and produces zero or more rewards.

Initial example:

```text
WHEN lesson_completed
GIVE 100 XP
```

Start with the smallest representation needed for the MVP.

Do not introduce a complex DSL prematurely.

## Rule Versioning

Rules are version-aware.

Changing a rule must not change the historical interpretation of rewards already granted.

Example:

```text
version 1: lesson_completed -> +100 XP
version 2: lesson_completed -> +50 XP
```

Reward history should identify the rule and rule version responsible for the grant.

## Reward

A Reward is the outcome of successful rule evaluation.

Examples:

- XP
- badge unlock
- achievement unlock
- progress increment
- level change
- custom domain event

## Reward Grant

A Reward Grant is the auditable historical record that a reward was applied.

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

Do not represent reward history only as mutation of current player state.

## Player State

Player State is the current materialized result of processed rewards.

The desired relationship is:

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

This preserves both efficient reads and historical auditability.

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

For the initial model, the highest persisted version of a given `rule_id` is the active version. Older versions remain immutable historical definitions and must not be re-evaluated for new events.

## Count-based aggregate rules

The first aggregate capability remains deliberately small. A Rule may set `match_every = N` to award its XP on every Nth matching Event for each Player.

For example:

```text
WHEN lesson_completed
AND course_id = course_7
EVERY 5 MATCHES PER PLAYER
GIVE 250 XP
```

The Event must first match the Rule's exact event type and all configured top-level property conditions. Only then is the Project + Player + Rule identity + Rule version counter incremented. `match_every = 1` is the existing immediate-reward behavior.

Aggregate progress is materialized in `rule_match_counts` and updated in the same transaction as event processing, Reward Grants, and Player State. This keeps duplicate retries from advancing the counter twice and lets concurrent matching Events cross each threshold exactly once. Rule versions have independent counters so historical Rule meaning remains stable.

This is not a general aggregate DSL. Arbitrary expressions, rolling windows, time-aware rules, nested property paths, and advanced compositions remain outside this slice.

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

Player State is the current materialized result of processed rewards. The initial implementation stores Project-scoped XP in `player_states`, updates it in the same transaction as Reward Grants, and uses a transaction-scoped `event_processing` claim to distinguish Event identity from completed processing. A committed duplicate Event retry reads the already-persisted outcome rather than applying XP again; a failed transaction rolls the claim back so a retry can process safely.

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

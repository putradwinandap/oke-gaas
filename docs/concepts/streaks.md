# Daily Streaks

A Project Streak qualifies a Player on a UTC calendar day when an accepted Event matches its exact `event_type` and optional exact top-level property conditions. The UTC day is derived from normalized Event `occurred_at`, not receipt time.

Definitions are immutable and names are unique within a Project. Every qualified day is persisted with its first qualifying Event identity and processing timestamp. Database uniqueness allows one claim per Project + Player + Streak + UTC day and prevents a logical Event from contributing twice to the same Streak. Claims commit or roll back with Event processing.

Current consecutive length is derived from the persisted set of qualified days. A missing day breaks the sequence. A late Event can fill a historical gap and repair the derived length deterministically. The active run is zero when the latest qualified day is older than yesterday UTC; today and yesterday remain active. No custom timezone or grace period applies.

The REST API creates and lists definitions at `/v1/projects/{projectId}/streaks` and returns definitions with each Player's active count and latest qualified date at `/v1/projects/{projectId}/players/{playerId}/streaks`. This state is separate from the `once_per_utc_day` XP rule gate.

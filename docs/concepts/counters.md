# Player-Visible Counters

Counters expose simple, current progress for exact Event patterns such as completed lessons or submitted forms. A Counter is configured by a Project and has a name, an exact `event_type`, and optional exact top-level Event property conditions. Every configured condition must match.

Counter definitions are immutable. To count a different Event pattern, create another Counter. Names are unique within a Project.

For each accepted Event, Oke Gaas increments every matching Counter for that Event's Player. A Player's value is scoped to the Project and Counter. Events that do not match leave the value unchanged. A duplicate Event retry does not increment it again.

Counter updates share the Event processing transaction with event completion, reward grants, and Player State. If any part fails, the Counter update rolls back with the Event. Concurrent matching Events use an atomic database increment so updates are not lost.

The API creates and lists Project Counter definitions at `/v1/projects/{projectId}/counters`, and returns all Counter values (including zero values) at `/v1/projects/{projectId}/players/{playerId}/counters`.

These Counters are explicit Player progress indicators, not analytics aggregates or arbitrary gauges. Rule count thresholds remain separate: `rule_match_counts` track evaluation progress for a particular Rule version and do not serve as public Counter state.

Daily consecutive Streaks are a separate mechanic. They derive active runs from auditable qualified UTC days rather than using increment-only Counter values. See [Daily Streaks](streaks.md).

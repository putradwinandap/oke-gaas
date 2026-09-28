# Achievements

An Achievement is a durable milestone linked to one Project-owned Counter and a positive target. When an accepted Event advances that Counter to or beyond the target, the Player unlocks the Achievement once.

Definitions are immutable. To change a target or the referenced Counter, create another Achievement. Unlocks remain queryable and record the triggering Event, observed Counter value, and unlock time.

Counter increments and unlock records are persisted in the same Event-processing transaction. Duplicate Event retries do not reevaluate progress, and a Project + Player + Achievement uniqueness constraint prevents concurrent Events from creating duplicate unlocks.

Project APIs create and list definitions at `/v1/projects/{projectId}/achievements`. Player state is returned at `/v1/projects/{projectId}/players/{playerId}/achievements`, including each definition and whether it has been unlocked. A cross-Project Counter reference is rejected.

The first slice supports exactly one Counter threshold per Achievement. It does not include condition composition, mutable targets, revocation, repeatable or time-limited milestones, or badge visuals. A Badge is a collectible Reward; an Achievement is a milestone.

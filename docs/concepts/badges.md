# Badges

A Badge is a Project-scoped collectible Reward. An Achievement describes a milestone in Player progress; a Badge is granted by a matching, versioned Rule.

## Definitions

Badge definitions have an opaque `badge_` identifier, a Project-unique display name, optional description, and creation time. Definitions are immutable in this first slice.

## Grants and ownership

A Rule version grants either XP or one Badge. Badge Rules reuse the existing exact Event type and optional exact top-level property conditions. Aggregate thresholds and once-per-UTC-day gates are not supported for Badge Rules in this slice.

The first successful grant creates a Badge Reward Grant in `reward_grants`, linked to its Project, Player, Event, Rule identity/version, and Badge. The database enforces one grant per Project + Player + Badge, so duplicate retries and concurrent qualifying Events cannot duplicate ownership. Later qualifying Events leave the existing first-grant audit record intact.

Badge definition access and Player collections are always Project-scoped. Collection responses include the Badge display metadata and the Event/Rule version that first granted it. Badge trading, expiration, revocation, rarity, and multiple rewards per Rule remain out of scope.

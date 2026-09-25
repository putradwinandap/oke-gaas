# Projects and Players

## Project

A Project is the primary tenant and integration boundary.

Data belonging to one Project must not be readable or mutable by another Project.

Project-scoped entities should carry or derive an unambiguous `project_id`.

Examples include:

- players
- events
- rules
- reward grants
- leaderboards
- achievements
- webhooks

Tenant isolation must be enforced by implementation and covered by automated tests.

## Player

A Player is the gamified end user.

Use **Player** consistently for the person/entity receiving:

- XP
- levels
- achievements
- badges
- streaks
- challenge progress
- other gamification state

Do not use `User` as a synonym for Player.

Reserve `User` for future Oke Gaas account users such as administrators, operators, or dashboard members.

## Player State

Player State is the materialized current gamification state used for efficient reads.

Historical explanation should come from event and reward history rather than relying only on the current snapshot.

## Future Environments

A Project may later contain environments such as development, staging, and production.

The exact environment model is not yet locked and should be introduced through an explicit decision when needed.

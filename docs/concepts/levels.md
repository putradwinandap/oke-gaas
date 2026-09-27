# XP Levels

Levels are a Project-scoped progression view derived from Player XP.

## Initial model

Level 1 is implicit at 0 XP and is not persisted.

Projects may append immutable thresholds for later Levels:

\`\`\`text
Level 1 -> 0 XP
Level 2 -> 100 XP
Level 3 -> 250 XP
\`\`\`

Configured thresholds must be contiguous by Level number and strictly increasing by XP. The first persisted threshold is therefore Level 2.

## Source of truth

Levels do not replace or duplicate XP state.

\`\`\`text
Reward Grants -> Player XP State -> Derived Level
\`\`\`

Reward Grants remain the auditable explanation for XP changes. Player State remains the materialized XP value. Current Level is resolved from the Project's threshold ladder when state is returned.

## Project isolation

Thresholds are scoped by \`project_id\`. A Project's ladder must never affect another Project's Players.

## First-slice constraints

The initial implementation intentionally excludes:

- mutable or reordered thresholds
- named tiers
- per-Level rewards
- level-up or level-down domain events
- prestige/reset mechanics
- formula-based Levels
- a generic progression DSL

These capabilities require concrete use cases before introduction.

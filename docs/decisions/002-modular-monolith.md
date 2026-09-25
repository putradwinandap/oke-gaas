# ADR-002: Event-Driven Modular Monolith

- **Status:** Accepted
- **Date:** 2026-09-25

## Context

Oke Gaas is event-oriented by nature, but an early microservices architecture would add deployment, networking, retry, ordering, observability, and consistency complexity before real scaling requirements exist.

## Decision

Start with an **event-driven modular monolith** using **lightweight Domain-Driven Design**.

Initial processing is synchronous.

Do not require a message broker.

Do not use full Event Sourcing initially.

## Module Direction

Likely modules include:

```text
Project
Player
Event
Rule
Reward
Progression
Achievement
Leaderboard
Webhook
```

Not every module must exist in the MVP.

Boundaries should be explicit enough that a module can be extracted later if evidence justifies it.

## Consequences

### Positive

- simple deployment and local development
- easier transactions
- easier debugging
- lower operational cost
- preserves architectural boundaries without premature distribution

### Trade-offs

- module discipline must be enforced inside one process
- future extraction may require adapter work
- in-process coupling must be reviewed carefully

## Evolution Criteria

Consider extracting a workload only for concrete reasons such as:

- independent scaling
- materially different reliability needs
- independent deployment cadence
- workload isolation
- different storage/compute characteristics

A module name alone is not justification for a service.

# @oke-gaas/sdk

Initial JavaScript/TypeScript integration SDK for Oke Gaas.

> **Compatibility notice:** this is an MVP API. Public SDK and REST compatibility are not yet guaranteed between versions.

The SDK is intentionally thin: it authenticates, calls the Oke Gaas REST API, maps transport fields into JavaScript-friendly names, and normalizes transport/API errors. Gamification rules, reward calculation, tenant authorization, and idempotency semantics remain server responsibilities.

## Security

A Project API key is a secret. Use this SDK from a trusted server-side application or another controlled runtime. Do **not** embed the key in a public browser bundle.

## Runtime

The current SDK requires Node.js 22 or newer. This matches the runtime exercised by repository CI.

Requests use a 15-second client timeout by default. Override it with `timeoutMs` when creating the client:

```ts
const gaas = createGaas({
  projectId: process.env.OKE_GAAS_PROJECT_ID!,
  apiKey: process.env.OKE_GAAS_API_KEY!,
  timeoutMs: 20_000,
});
```

## Install from this repository

```bash
npm install ./sdk/typescript
```

## Usage

```ts
import { createGaas } from "@oke-gaas/sdk";

const gaas = createGaas({
  baseUrl: process.env.OKE_GAAS_BASE_URL,
  projectId: process.env.OKE_GAAS_PROJECT_ID!,
  apiKey: process.env.OKE_GAAS_API_KEY!,
});

const result = await gaas.track("lesson_completed", {
  playerId: "player_123",
  properties: {
    lessonId: "lesson_5",
  },
});

const state = await gaas.players.get("player_123");
console.log(result, state);
```

`track()` generates an event ID for the call when one is omitted. If your application needs idempotency to survive a new process or a caller-managed retry, provide a stable `eventId` explicitly:

```ts
await gaas.track("purchase_completed", {
  playerId: "player_123",
  eventId: "checkout-order-42",
  occurredAt: new Date("2026-09-26T12:34:56Z"),
});
```

The current SDK also exposes the thin MVP setup wrappers `players.create()`, `rules.create()`, and `counters.create()` so an integration can exercise the complete Project-scoped vertical slice. Project provisioning remains an operator/admin API concern and is intentionally not exposed through this Project-key client.

### Player-visible counters

Create an immutable Counter for an exact Event type, optionally narrowed by exact top-level Event properties, then read its current value for a Player:

```ts
await gaas.counters.create({
  name: "completed lessons",
  eventType: "lesson_completed",
  conditions: { course_id: "course_7" },
});

const counters = await gaas.players.getCounters("player_123");
```

The response includes every Counter in the Project, with zero for Counters the Player has not matched. Accepted matching Events increment the value once; duplicate Event retries do not increment it again.

Every SDK operation accepts an optional request-options argument with an `AbortSignal`:

```ts
const controller = new AbortController();

const statePromise = gaas.players.get("player_123", {
  signal: controller.signal,
});

controller.abort();
await statePromise;
```

## Errors

HTTP API errors are exposed as `GaasError` with `code`, optional `status`, and the server-safe message.

Transport-related SDK codes include:

- `network_error` — connection or response-body transport failure
- `timeout_error` — the configured client deadline expired
- `request_aborted` — the caller's `AbortSignal` cancelled the request
- `invalid_response` — a successful HTTP response did not match the expected API shape

Successful API responses are runtime-validated before mapping. Integer XP/reward values must fit JavaScript's safe-integer range; the SDK rejects values that would otherwise lose precision.

## Development

```bash
npm install
npm run check
```


### Conditional XP rules

Rules may optionally require exact matches on top-level event properties:

```ts
await gaas.rules.create({
  eventType: "lesson_completed",
  xp: 100,
  conditions: {
    course_id: "course_7",
    difficulty: "hard",
  },
});
```

Every configured condition must match. Nested property paths and comparison operators are intentionally not supported yet.

### Count-based aggregate XP rules

Use `matchEvery` when a reward should fire on every Nth matching Event for each Player:

```ts
await gaas.rules.create({
  eventType: "lesson_completed",
  xp: 250,
  conditions: {
    course_id: "course_7",
  },
  matchEvery: 5,
});
```

Omitting `matchEvery` defaults to `1`, preserving immediate rewards. Counting and threshold evaluation remain server responsibilities; the SDK only sends the configuration. Arbitrary aggregate expressions and a general rule DSL are intentionally not supported.

### Once-per-UTC-day XP rules

Use `oncePerUtcDay` for a narrow daily-login style rule that may grant at most once per UTC calendar day:

```ts
await gaas.rules.create({
  eventType: "daily_login",
  xp: 25,
  oncePerUtcDay: true,
});
```

The day is derived from the Event's `occurredAt` in UTC. This first time-aware slice requires `matchEvery: 1`; custom time zones, rolling windows, streak state, and general scheduling expressions are intentionally out of scope.

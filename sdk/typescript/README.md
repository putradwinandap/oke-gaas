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

The current SDK also exposes the thin MVP setup wrappers `players.create()` and `rules.create()` so an integration can exercise the complete Project-scoped vertical slice. Project provisioning remains an operator/admin API concern and is intentionally not exposed through this Project-key client.

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

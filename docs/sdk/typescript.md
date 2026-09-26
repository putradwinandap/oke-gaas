# TypeScript SDK

The initial TypeScript SDK lives under `sdk/typescript/` and is a thin Project-scoped integration client for the REST API.

## Boundary

The SDK may:

- attach the Project bearer key
- construct REST requests
- generate a per-call external Event ID when the caller does not supply one
- map REST transport field names into JavaScript-friendly result objects
- normalize HTTP/API/transport failures into `GaasError`
- validate successful response payloads before exposing typed SDK results
- enforce a finite client request deadline and propagate caller cancellation

The SDK must not:

- evaluate gamification Rules
- calculate Rewards or XP
- reproduce tenant authorization rules
- infer Player State locally
- replace the server's idempotency or transaction semantics

A caller that needs idempotency across process restarts or caller-managed retries must supply its own stable `eventId`. An SDK-generated ID is stable only for the lifetime of that `track()` call.

## Credential model

`createGaas()` accepts a single `projectId` and Project API key. The client always sends the key as a bearer credential to that Project's endpoints.

Project API keys are secrets. The initial SDK is intended for trusted server-side or controlled runtimes; keys must not be embedded in public browser bundles.

The SDK deliberately does not accept the operator/admin key. Project provisioning remains separate from normal application integration.

## Runtime and request lifecycle

The initial SDK requires Node.js 22 or newer, matching the runtime used by repository CI.

Each request has a 15-second client timeout by default. `GaasConfig.timeoutMs` may override the client-wide deadline, and each SDK operation accepts optional `RequestOptions` with an `AbortSignal` for caller cancellation.

Fetch failures and failures while reading the response body are normalized as `GaasError`. Timeout and caller cancellation use distinct `timeout_error` and `request_aborted` codes.

Successful JSON responses are validated at runtime instead of being trusted through TypeScript assertions alone. Response identifiers must remain bound to the requested Project, Player, Event, or Rule inputs where applicable. Numeric XP, rule versions, and reward amounts must be JavaScript safe integers; responses outside that range are rejected as `invalid_response` rather than silently losing precision.

Base URLs must use HTTP(S) and must not contain embedded credentials, query strings, or fragments. A path prefix remains allowed for reverse-proxy deployments.

## Public API stability

The REST API and SDK are still in MVP validation. Compatibility is not yet guaranteed between versions. Breaking changes must be documented while this unstable status remains.

## Quality checks

From `sdk/typescript/`:

```bash
npm install
npm run check
```

`npm run check` performs strict TypeScript type-checking, builds declarations/JavaScript, and runs Node's test runner against the compiled SDK.

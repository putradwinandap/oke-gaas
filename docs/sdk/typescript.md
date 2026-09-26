# TypeScript SDK

The initial TypeScript SDK lives under `sdk/typescript/` and is a thin Project-scoped integration client for the REST API.

## Boundary

The SDK may:

- attach the Project bearer key
- construct REST requests
- generate a per-call external Event ID when the caller does not supply one
- map REST transport field names into JavaScript-friendly result objects
- normalize HTTP/API/transport failures into `GaasError`

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

## Public API stability

The REST API and SDK are still in MVP validation. Compatibility is not yet guaranteed between versions. Breaking changes must be documented while this unstable status remains.

## Quality checks

From `sdk/typescript/`:

```bash
npm install
npm run check
```

`npm run check` performs strict TypeScript type-checking, builds declarations/JavaScript, and runs Node's test runner against the compiled SDK.

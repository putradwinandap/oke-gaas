# REST API conventions

The first public API slice is intentionally marked unstable while the MVP is validated.

## Authentication

- Project creation is an operator action authenticated with `OKE_GAAS_ADMIN_API_KEY`.
- Creating a Project returns one Project API key once.
- Only the SHA-256 digest of the Project API key is persisted.
- Project-scoped endpoints require that Project's bearer key.
- A key issued for one Project must never authorize access to another Project.

## Public identifiers

Oke Gaas-generated identifiers use a short semantic prefix plus a random opaque suffix:

- `proj_...` for Projects
- `player_...` for Players
- `rule_...` for Rules
- `grant_...` for Reward Grants

External Event identity is different: `event_id` is supplied by the integrating application, must be stable across retries, and is unique only within one Project.

Clients must treat all identifiers as opaque strings and must not infer ordering or database structure from them.

## Errors

API failures use one JSON envelope:

```json
{
  "error": {
    "code": "machine_readable_code",
    "message": "human-readable message"
  }
}
```

See `openapi.yaml` for the currently implemented endpoints.

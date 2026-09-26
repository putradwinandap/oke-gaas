# Oke Gaas

Oke Gaas is open-source gamification infrastructure with an optional managed SaaS layer.

The canonical flow is:

```text
Event -> Rule -> Reward -> Player State
```

Engineering rules and architectural constraints live in [AGENTS.md](./AGENTS.md). Durable technical documentation lives under [docs/](./docs/).

## Development

### Requirements

- Go 1.26+
- PostgreSQL for persistence-backed features

### Run the API

```bash
go run ./cmd/server
```

The server listens on `:8080` by default.

Override it with:

```bash
OKE_GAAS_HTTP_ADDR=:9090 go run ./cmd/server
```

Health check:

```text
GET /health
```

### Configuration

Copy `.env.example` as a reference for supported environment variables.

Current baseline:

- `OKE_GAAS_HTTP_ADDR` — HTTP listen address.
- `DATABASE_URL` — PostgreSQL connection string. Persistence is not connected at process startup until a feature requires it.

### Quality checks

```bash
gofmt -w .
go vet ./...
go test ./...
```

CI runs formatting, vet, and tests on pull requests and pushes to `main`.

## Initial repository structure

```text
cmd/server/                    application entry point
internal/platform/config/     process configuration
internal/platform/http/       Fiber delivery adapter
internal/platform/database/   GORM/PostgreSQL adapter
internal/platform/validation/ validator adapter
docs/                         durable technical documentation
```

Domain modules will be introduced only as concrete vertical slices require them. Fiber and GORM must not leak into domain code.

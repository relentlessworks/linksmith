# LinkSmith

> Agentic-first link shortener. The agent IS the interface.

LinkSmith is a link shortener designed for AI agents to drive over plain HTTP. No UI, no SDK. The API is the product.

## Quick Start

```bash
make build
./linksmith
```

The server starts on `:8080` with zero config. Data is persisted to a JSON file automatically.

## How It Works

1. **Get a token**: `POST /auth/request` → `POST /auth/verify` → bearer token
2. **Shorten a link**: `POST /api/links` with `url=https://example.com`
3. **Redirect**: `GET /l/link_a1b2c` → 301 to original URL
4. **Manage**: `GET /api/links`, `GET /api/links/<handle>`, `DELETE /api/links/<handle>`

## Principles

- **Plain text by default** — one labeled, grepable line per record. JSON on demand via `Accept: application/json` or `?format=json`.
- **Instructive errors** — every 4xx includes a hint telling the agent what to do next.
- **Self-documenting** — `GET /help` returns a one-page operating manual.
- **Simple auth** — OTP via email → long-lived bearer token.
- **Single static binary** — Go, zero external dependencies, deploys as one file.
- **Zero config defaults** — runs out of the box. Config: defaults < env < flags.
- **Multi-tenant** — workspaces isolate links per tenant.
- **Short stable handles** — every link gets a workspace-scoped handle like `link_k7m2q`.

## Configuration

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `-addr` | `LINKSMITH_ADDR` | `:8080` | Listen address |
| `-db` | `LINKSMITH_DB` | `linksmith.json` | Data file path |
| `-secret` | `LINKSMITH_SECRET` | random | Token signing secret |

## Build

```bash
make build    # CGO_ENABLED=0, single static binary
make test     # go test ./...
make vet      # go vet ./...
```

## API Reference

### Authentication

```
POST /auth/request   email=<email>&workspace=<handle>  → OTP code
POST /auth/verify     email=<email>&code=<code>          → Bearer token
```

### Links (requires Bearer token)

```
POST   /api/links          url=<https://example.com>     → handle=link_xxx url=...
GET    /api/links                                        → list all links
GET    /api/links/<handle>                                → link details
DELETE /api/links/<handle>                                → delete link
GET    /api/workspace                                     → workspace info
```

### Redirect (public)

```
GET /l/<handle>  → 301 redirect to original URL
```

### Formats

- **Plain text** (default): `handle=link_a1b2c url=https://example.com clicks=0`
- **JSON**: add `Accept: application/json` or `?format=json`

## License

MIT

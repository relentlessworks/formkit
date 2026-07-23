# FormKit

Agentic-first form builder and submission collector. Plain text API, agent-driven, single Go binary with JSON file storage.

The agent IS the interface. No UI, no SDK. The API is the product.

## Quick Start

```bash
# Build
make build

# Run (defaults to :7705, data in ./formkit-data)
./formkit

# Or with custom config
./formkit -addr :8080 -data /var/lib/formkit
```

## Auth Flow

```bash
# 1. Request OTP
curl -X POST http://localhost:7705/auth/request -d 'email=agent@example.com'
# → ok: OTP sent to agent@example.com (check stderr if no SMTP configured)

# 2. Verify OTP → get bearer token
curl -X POST http://localhost:7705/auth/verify -d 'email=agent@example.com&code=123456'
# → token=abc123... workspace=ws_x1y2z email=agent@example.com

# 3. Use token for all subsequent requests
curl -H "Authorization: Bearer abc123..." http://localhost:7705/forms
```

## API Reference

### Forms (auth required)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/forms` | Create a form |
| GET | `/forms` | List all forms |
| GET | `/forms/{handle}` | Get form details |
| PATCH | `/forms/{handle}` | Update form |
| DELETE | `/forms/{handle}` | Delete form + submissions |

### Submissions (auth required for viewing)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/submissions?form={handle}` | List submissions for a form |
| GET | `/submissions/{handle}` | Get submission data |
| DELETE | `/submissions/{handle}` | Delete a submission |

### Public Submission (no auth)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/s/{form_handle}` | Submit data to a form |

### Other

| Method | Path | Description |
|--------|------|-------------|
| GET | `/help` | Operating manual for agents |
| GET | `/.well-known/agent.md` | Same as /help |
| POST | `/auth/request` | Request OTP |
| POST | `/auth/verify` | Verify OTP, get token |

## Creating a Form

```bash
curl -X POST http://localhost:7705/forms \
  -H "Authorization: Bearer TOKEN" \
  -d 'title=Customer Feedback&fields=name:Name:text:true,email:Email:email:true,rating:Rating:number:false,feedback:Feedback:textarea:false'
# → handle=form_a1b2c title=Customer Feedback fields=4 active=true
```

### Field Format

`name:label:type:required[:options]`

- **name**: Machine name (no spaces)
- **label**: Human-readable label
- **type**: `text`, `email`, `number`, `textarea`, `select`, `checkbox`
- **required**: `true` or `false`
- **options**: Comma-separated values (only for `select` type)

Example: `color:Favorite Color:select:true:red,green,blue`

## Collecting Submissions

```bash
# Public endpoint — no auth needed
curl -X POST http://localhost:7705/s/form_a1b2c \
  -d 'name=Alice&email=alice@example.com&rating=5&feedback=Great service!'
# → handle=sub_x1y2z form=form_a1b2c ok=submission accepted
```

## Retrieving Submissions

```bash
# List submissions for a form
curl -H "Authorization: Bearer TOKEN" \
  'http://localhost:7705/submissions?form=form_a1b2c'
# → handle=sub_x1y2z form=form_a1b2c created=2026-07-23T00:00:00Z

# Get submission details
curl -H "Authorization: Bearer TOKEN" \
  http://localhost:7705/submissions/sub_x1y2z
# → handle=sub_x1y2z form=form_a1b2c created=2026-07-23T00:00:00Z
#   data:
#   name=Alice
#   email=alice@example.com
#   rating=5
#   feedback=Great service!
```

## Response Format

- **Plain text** by default (one record per line, `key=value` pairs)
- **JSON** via `Accept: application/json` header or `?format=json` query param
- **Errors**: `error: message | hint: what to do next`

## Configuration

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `-addr` | `FORMKIT_ADDR` | `:7705` | Listen address |
| `-data` | `FORMKIT_DATA` | `./formkit-data` | Data directory |
| `-smtp` | `FORMKIT_SMTP` | (empty) | SMTP host:port for OTP email |
| `-secret` | `FORMKIT_SECRET` | (auto) | Token signing secret |

Config priority: defaults < flags < env vars

## Build

```bash
make build    # CGO_ENABLED=0, single static binary
make test     # go test -race
make vet      # go vet
```

## License

MIT

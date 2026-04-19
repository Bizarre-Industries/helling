# Pagination Contract

Normative pagination rules for list endpoints under `/api/v1/*`.

## Policy

- Cursor pagination only.
- Offset pagination (`offset`, `page`) is not supported.
- Cursors are opaque to clients.

## Request Parameters

| Param  | Type      | Default          | Max | Notes                               |
| ------ | --------- | ---------------- | --- | ----------------------------------- |
| limit  | int       | 50               | 500 | Values above max are clamped to 500 |
| cursor | string    | none             | n/a | Opaque token from previous response |
| sort   | string    | endpoint-defined | n/a | Must be from endpoint allowlist     |
| order  | asc\|desc | desc             | n/a | Direction for selected sort key     |

Example request:

`GET /api/v1/users?limit=50&cursor=eyJ0cyI6...&order=desc`

## Response Meta

List responses return pagination state in `meta.page`.

```json
{
  "data": [],
  "meta": {
    "request_id": "req_01JZ...",
    "page": {
      "has_next": true,
      "next_cursor": "eyJ0cyI6...",
      "limit": 50
    }
  }
}
```

`next_cursor` is null or absent when no further page exists.

## Cursor Format (Internal)

Internal shape may be base64url-encoded JSON such as `{ "ts": ..., "id": ... }`, but clients must treat cursor as opaque and stable only for short-lived pagination flows.

## Endpoint Notes

Endpoints may define narrower defaults or hard caps but must remain compatible with this top-level contract.

Current list endpoints:

- `/api/v1/users`
- `/api/v1/schedules`
- `/api/v1/audit`
- `/api/v1/auth/tokens` (limit/cursor optional for future growth)
- `/api/v1/webhooks` (limit/cursor optional for future growth)

# API error responses

Every non-2xx response from `core-service` uses the same envelope:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "request validation failed",
    "fields": {
      "whatsapp": "must be a valid Indonesian whatsapp number",
      "password": "must be between 8 and 72 characters"
    },
    "field_codes": {
      "whatsapp": "invalid_format",
      "password": "invalid_length"
    }
  },
  "request_id": "6f740008-3387-47ac-a7f1-480cefbee14a"
}
```

| Key | Always present | Meaning |
| --- | --- | --- |
| `error.code` | yes | Stable error code (see below). Branch on this. |
| `error.message` | yes | English, developer-facing. **Do not show it to end users** — it may change. |
| `error.fields` | 400 only | Per-field English messages, keyed by JSON field name. Kept for backward compatibility. |
| `error.field_codes` | 400 only | Same keys as `fields`, with a stable machine-readable code. Clients map these to their own user-facing text. |
| `request_id` | yes | Correlates with server logs; useful in bug reports. |

## Field codes (`error.field_codes`)

| Code | When |
| --- | --- |
| `required` | Field is missing or empty. |
| `invalid_format` | Wrong format: email, WhatsApp number, UUID, date. |
| `invalid_length` | Text is too short or too long (e.g. names 2–120 chars, password 8–72 bytes). |
| `out_of_range` | Number outside the allowed range (coordinates, quantities, limits). |
| `invalid_choice` | Value is not one of the allowed options (`account_type`, payment method, …). |
| `invalid_type` | JSON value has the wrong type (e.g. string instead of number). |
| `invalid_json` | Request body is not a single valid JSON object. Reported under `body`. |
| `unknown_field` | The endpoint does not accept this JSON property. Requests are decoded strictly. |
| `duplicate` | Value repeats within the request (e.g. the same product twice). |
| `invalid` | Any other validation failure. |

Codes are derived centrally in `internal/platform/apperror/fieldcode.go` from the validators' messages, so validation rules themselves are unchanged. When adding a validator, reuse one of the existing phrasings (`is required`, `must be between X and Y characters`, `must be a valid …`, `must be one of …`) or extend `FieldCode` and its test.

## Error codes (`error.code`) and HTTP status

| Status | Codes | Typical cause |
| --- | --- | --- |
| 400 | `VALIDATION_ERROR` | One or more fields invalid — see `fields` / `field_codes`. |
| 401 | `UNAUTHORIZED`, `INVALID_CREDENTIALS`, `BUSINESS_SCOPE_REQUIRED` | Missing/expired token, or wrong email/password on login. |
| 403 | `FORBIDDEN` | Authenticated but not allowed. |
| 404 | `NOT_FOUND` | Resource does not exist or is outside the caller's business. |
| 409 | `EMAIL_TAKEN`, `WHATSAPP_TAKEN`, `CONFLICT`, `IDEMPOTENCY_MISMATCH`, `STOCK_MISMATCH`, … | Request conflicts with existing data. |
| 429 | `RATE_LIMITED` | Too many requests (public invoice endpoints). |
| 5xx | `INTERNAL_ERROR`, `UNAVAILABLE`, … | Server-side failure; safe to retry later. |

## Register (`POST /v1/auth/register`) validation rules

| Field | Rule | Codes |
| --- | --- | --- |
| `owner_name` | 2–120 characters after trimming | `invalid_length` |
| `business_name` | 2–120 characters after trimming (penitip) | `invalid_length` |
| `email` | trimmed + lower-cased; `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`, max 254 | `invalid_format` |
| `whatsapp` | optional; non-digits removed, leading `0`/`8` becomes `62`, then `^62[0-9]{8,15}$` (stored as `62…`) | `invalid_format` |
| `password` | 8–72 **bytes** (bcrypt limit) | `invalid_length` |

Conflicts return `409 EMAIL_TAKEN` or `409 WHATSAPP_TAKEN`. Unknown properties (for example `account_type` against a build without account types) return `400` with `field_codes.<name> = "unknown_field"`.

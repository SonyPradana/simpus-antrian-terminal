# API contract — `app serve`

This application only **writes the flag `printer = 'medium'`** to `finger_unit`.
It mimics the **Cetak Antrian** button. The legacy print server still:

- fetches rows with `printer = 'medium'`,
- prints them via ESC/POS,
- marks them as `printer = 'Sudah'`.

## General information

- Base URL: `http://<host>:<port>` (`http://localhost:8080` by default)
- Format: `application/json; charset=utf-8`
- Authentication: `X-API-Key: <API_TOKEN>` for every endpoint except `/healthz`
- Server filters are always: `UnitID = 'Pendaftaran'` and `DATE(TanggalFingger) = CURDATE()`

## Endpoints

| Method | Path | Auth | Purpose |
|---|---|---|---|
| `POST` | `/api/print` | Yes | Write `printer='medium'` for today's queue code |
| `GET` | `/api/print/{kode}` | Yes | Check the current flag for a queue code |
| `GET` | `/healthz` | No | Check the process and database connection |

### `POST /api/print`

Request:

```json
{
  "kode": "B001",
  "force": false
}
```

`force` is optional and defaults to `false`. Any other field is rejected with `400 INVALID_REQUEST`.

Response `200 OK`:

```json
{
  "kode": "B001",
  "unit_id": "Pendaftaran",
  "tanggal": "2026-11-27",
  "printer": "medium",
  "rows_terubah": 1
}
```

Idempotent: retrying with the same code cannot cause a duplicate print. The second
request returns `409 ALREADY_REQUESTED` while the flag is still `medium`, or
`409 ALREADY_PRINTED` once the print server has marked it `Sudah`.

For a controlled reprint (for example during testing), send `"force": true`:

```json
{
  "kode": "C021",
  "force": true
}
```

Response `200 OK` then also contains `"force": true`. `force` skips only the
`ALREADY_REQUESTED` and `ALREADY_PRINTED` guards. Row existence, the `Pendaftaran`
unit filter, today's date, and birth-date validity are still enforced. Use it
carefully: a reprint intentionally produces a duplicate printout.

With `app serve --dry-run`, the endpoint only validates and does not run `UPDATE`:

```json
{
  "kode": "B001",
  "unit_id": "Pendaftaran",
  "tanggal": "2026-11-27",
  "printer": "menunggu",
  "rows_terubah": 0,
  "dry_run": true
}
```

### `GET /api/print/{kode}`

Response `200 OK`:

```json
{
  "kode": "B001",
  "unit_id": "Pendaftaran",
  "tanggal": "2026-11-27",
  "printer": "medium",
  "tgl_lahir_valid": true
}
```

`printer` can be `medium`, `Sudah`, or another value. `tgl_lahir_valid = false`
means the print server will skip this row (`0000-00-00` or NULL).

### `GET /healthz`

`200 OK`:

```json
{
  "status": "ok",
  "db": "ok",
  "time": "2026-11-27T10:00:00+07:00"
}
```

`503 Service Unavailable` when the database ping fails:

```json
{
  "status": "degraded",
  "db": "error",
  "time": "2026-11-27T10:00:00+07:00"
}
```

## Errors

All errors use this shape:

```json
{
  "error": {
    "code": "QUEUE_NOT_FOUND",
    "message": "queue B001 for Pendaftaran today was not found"
  }
}
```

| HTTP | `error.code` | Cause |
|---|---|---|
| `400` | `INVALID_REQUEST` | Invalid JSON, unknown field, or empty `kode` |
| `401` | `UNAUTHORIZED` | `X-API-Key` is missing or invalid |
| `404` | `QUEUE_NOT_FOUND` | No row for the code, unit, and today's date |
| `409` | `NOT_ELIGIBLE` | `tgl_lahir` is `0000-00-00` / NULL |
| `409` | `ALREADY_REQUESTED` | `printer` is already `medium` |
| `409` | `ALREADY_PRINTED` | `printer` is already `Sudah` |
| `500` | `DATABASE_ERROR` | Transaction or query failed |

With `force: true`, `ALREADY_REQUESTED` and `ALREADY_PRINTED` are skipped.
`NOT_ELIGIBLE` still applies because it is a data-quality guard, not a duplicate guard.

Database error details are never sent to the client; they are only written to the server log.

## Examples

```powershell
$headers = @{
  'X-API-Key' = '<api-token>'
  'Content-Type' = 'application/json'
}

Invoke-RestMethod -Method Post `
  -Uri 'http://localhost:8080/api/print' `
  -Headers $headers `
  -Body '{"kode":"B001"}'

Invoke-RestMethod -Uri 'http://localhost:8080/api/print/B001' -Headers $headers
Invoke-RestMethod -Uri 'http://localhost:8080/healthz'
```

```bash
curl -X POST http://localhost:8080/api/print \
  -H 'X-API-Key: <api-token>' \
  -H 'Content-Type: application/json' \
  -d '{"kode":"B001"}'

# Controlled reprint
curl -X POST http://localhost:8080/api/print \
  -H 'X-API-Key: <api-token>' \
  -H 'Content-Type: application/json' \
  -d '{"kode":"C021","force":true}'
```

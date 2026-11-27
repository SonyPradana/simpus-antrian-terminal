# simpus-antrian-terminal

[![CI](https://github.com/SonyPradana/simpus-antrian-terminal/actions/workflows/ci.yml/badge.svg)](https://github.com/SonyPradana/simpus-antrian-terminal/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/SonyPradana/simpus-antrian-terminal)](https://github.com/SonyPradana/simpus-antrian-terminal/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/SonyPradana/simpus-antrian-terminal)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A Go CLI that exposes an HTTP API to **write the flag `printer = 'medium'`** to `finger_unit`.
It mimics the **Cetak Antrian** button. The legacy print server keeps responsibility for:

- fetching rows with `printer = 'medium'`,
- printing via ESC/POS,
- marking rows as `printer = 'Sudah'`.

This application has **no** worker, ESC/POS code, receipt template, or reset endpoint.

> ⚠️Production: never run `DELETE`, `TRUNCATE`, or mass `UPDATE` statements on the database.
> The only write performed by this application is `UPDATE finger_unit SET printer='medium'`
> with `WHERE Kode=? AND UnitID='Pendaftaran' AND DATE(TanggalFingger)=CURDATE() LIMIT 1`,
> inside a transaction that first runs `SELECT ... FOR UPDATE`.

## Install

```bash
go install github.com/SonyPradana/simpus-antrian-terminal@latest
```

Or build from source:

```powershell
go build -o app.exe .
```

## Configuration

Copy `config.json.example` to `config.json` and replace every placeholder.
Alternatively, use `.env.example` as a template and export those variables into your
shell — the application reads process environment variables and does not auto-load `.env`.
Environment variables override values from the file:

| Env | Default | Description |
|---|---|---|
| `DB_HOST` | required | Print server MySQL host |
| `DB_PORT` | `3306` | MySQL port |
| `DB_USER` | required | MySQL user |
| `DB_PASSWORD` | required | MySQL password |
| `DB_NAME` | required | Database name |
| `HTTP_ADDR` | `:8080` | HTTP listen address |
| `API_TOKEN` | required | Value for the `X-API-Key` header |

No database credentials or internal hostnames are stored in source. `config.json`
must not be committed (it is already in `.gitignore`).

## Run

```powershell
$env:DB_HOST = '<db-host>'
$env:DB_USER = '<db-user>'
$env:DB_PASSWORD = '<db-password>'
$env:DB_NAME = '<db-name>'
$env:API_TOKEN = '<api-token>'

.\app.exe serve --config config.json

# Dry-run: validate only, without writing to the database
.\app.exe serve --config config.json --dry-run

# Override the listen address
.\app.exe serve --config config.json --addr :9090
```

`serve` pings the database at startup and fails when it is unreachable.
Stop it with `Ctrl+C` for a graceful shutdown. Check the build with `app version`.

## API

The full contract is in [`docs/api.md`](docs/api.md). In short:

```powershell
$headers = @{ 'X-API-Key' = '<api-token>'; 'Content-Type' = 'application/json' }

# Register a print request
Invoke-RestMethod -Method Post `
  -Uri 'http://localhost:8080/api/print' `
  -Headers $headers `
  -Body '{"kode":"B001"}'

# Controlled reprint: skips the already-requested/already-printed guards
Invoke-RestMethod -Method Post `
  -Uri 'http://localhost:8080/api/print' `
  -Headers $headers `
  -Body '{"kode":"C021","force":true}'

# Check the current flag
Invoke-RestMethod -Uri 'http://localhost:8080/api/print/B001' -Headers $headers

# Healthcheck
Invoke-RestMethod -Uri 'http://localhost:8080/healthz'
```

Every successful write is logged with the queue code, the `force` flag, and the
client address. Database error details stay in the server log.

## Tests

Automated tests only use mocks and **never touch the production database**:

```powershell
go test -race ./...
go vet ./...
gofmt -l .
```

There is no integration test that runs automatically. Database verification requires
explicit permission first, for example with this read-only query:

```sql
SELECT fu.Kode, fu.UnitID, fu.TanggalFingger, fu.printer, p.tgl_lahir
FROM finger_unit fu
LEFT JOIN patients p ON p.id = fu.NoPasien
WHERE fu.Kode = 'B001'
  AND fu.UnitID = 'Pendaftaran'
  AND DATE(fu.TanggalFingger) = CURDATE();
```

Once the flag is `medium`, the query used by the legacy print server should return
that code. Do not run `UPDATE ... 'Sudah'` yourself — the print server does it.

## Security

See [`SECURITY.md`](SECURITY.md) for vulnerability reporting. Never put real
credentials or API tokens in issues, pull requests, or commits.

## License

[MIT](LICENSE) © 2026 Sony Pradana

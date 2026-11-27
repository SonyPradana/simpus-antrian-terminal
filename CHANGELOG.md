# Changelog

All notable changes to this project are documented here.

## v0.1.1 - 2026-11-27

### Changed

- Release builds now target Windows/amd64 only.
- Release archives include `.env.example`, `config.json.example`, and `docs/api.md`.

### Added

- `.env.example` template for environment-based configuration.

## v0.1.0 - 2026-11-27

### Added

- `app serve` HTTP API for registering print requests.
- `POST /api/print`, `GET /api/print/{kode}`, and `GET /healthz`.
- Strict validation: `Pendaftaran` unit, today's date, and valid `tgl_lahir`.
- Transactional `UPDATE ... SET printer='medium'` with `SELECT ... FOR UPDATE`.
- `force: true` for controlled reprints.
- `--dry-run` mode for validation without database writes.
- `X-API-Key` authentication, configuration via JSON/env, and audit logging.
- CI, GoReleaser release workflow, MIT license, and security policy.

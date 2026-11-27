# Contributing

## Local checks

Run these before opening a pull request:

```powershell
gofmt -l .
go vet ./...
go test -race ./...
go build -o app.exe .
```

Tests use mocks only. Do not add tests that connect to a real database.

## Conventions

- Keep code comments and documentation in English.
- Use conventional commit messages, for example `fix: reject zero birth date`.
- Do not commit `config.json`, `.env` files, or real credentials.
- Update `docs/api.md` whenever an endpoint or payload changes.

## Releases

Releases are created automatically by GitHub Actions when a `v*` tag is pushed.
Use semantic versioning, for example `v0.1.0`.

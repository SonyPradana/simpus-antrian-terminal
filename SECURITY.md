# Security Policy

## Supported versions

Only the latest released version receives security fixes.

| Version | Supported |
|---|---|
| `v0.1.x` | ✅ |
| `< 0.1` | ❌ |

## Reporting a vulnerability

Do **not** open a public issue for security problems. Report them privately through
GitHub Security Advisories: **Report a vulnerability** on this repository.

Include:

- affected version or commit,
- reproduction steps,
- impact,
- suggested fix, if any.

## Handling credentials

This tool talks to a production MySQL database and accepts a static `API_TOKEN`.
Never include real hostnames, usernames, passwords, tokens, or database dumps in
issues, pull requests, logs, or commits. `config.json` is gitignored; keep it that way.

Rotate `API_TOKEN` immediately if it is ever exposed.

## Production safety

The application only writes `printer = 'medium'` with a strict `WHERE` clause and
`LIMIT 1`. The `force: true` option intentionally allows controlled reprints and can
produce duplicate printouts. Use it only when a duplicate is acceptable.

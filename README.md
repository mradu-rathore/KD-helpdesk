# KD Helpdesk

Internal support ticketing prototype for KD Electronics, built on
[Libredesk](https://github.com/abhinavxd/libredesk) (Go + Vue, Postgres, Redis).
It bridges the gap between cancelling Zendesk and moving support into HubSpot.

## Repo layout

```
libredesk/              Libredesk source (upstream v2.9.2): edit freely
  cmd/, internal/       Go backend (API handlers, features, SQL in queries.sql)
  internal/migrations/  database migrations
  frontend/             Vue 3 frontend
dev/                    scripts to run everything locally
local/                  git-ignored runtime: Postgres data, Redis, uploads, secrets, logs
```

## Run locally (macOS)

Needs Go (version in `libredesk/go.mod`), Node + pnpm, Redis and Postgres 16:

```bash
brew install postgresql@16 redis
```

One-time setup (starts Postgres/Redis, builds, installs the database):

```bash
dev/setup.sh
```

Start the app:

```bash
dev/start.sh
```

Open http://localhost:9000 and log in with email `System`. The password is
`SYSTEM_USER_PASSWORD` in `local/secrets.env`.

- After changing code: run `dev/build.sh`, then restart `dev/start.sh`.
- `Ctrl+C` stops the app. `dev/services.sh stop` stops Postgres and Redis.
- Postgres (`127.0.0.1:5434`) and Redis (`127.0.0.1:6380`) run only for this project, with
  their data in `local/`. They are not system services.

## Updating Libredesk from upstream

```bash
git fetch libredesk-upstream tag vX.Y.Z --no-tags
git subtree pull --prefix=libredesk libredesk-upstream vX.Y.Z --squash
```

(`libredesk-upstream` = https://github.com/abhinavxd/libredesk, fetch-only.)

## Status

🚧 Prototype under active development.

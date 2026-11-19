# KD Helpdesk

Internal support ticketing prototype for KD Electronics, built on
[Frappe Helpdesk](https://github.com/frappe/helpdesk). It bridges the gap between
cancelling Zendesk and moving support into HubSpot.

## Repo layout

```
apps/helpdesk/   Frappe Helpdesk source (from upstream `main`): edit freely
  desk/src/      the agent UI (Vue 3 + frappe-ui)
  helpdesk/      backend (Python doctypes, APIs)
dev/             scripts to run everything locally
local/           git-ignored runtime: database data, Frappe bench, logs, passwords
```

Frappe Framework itself is not committed. `dev/setup.sh` downloads it into `local/`.

## Run locally (macOS, no Docker)

One-time setup (about 10 minutes):

```bash
brew install mariadb@10.11 redis uv
dev/setup.sh
```

Start the server:

```bash
dev/start.sh
```

Open http://helpdesk.localhost:8000/helpdesk and log in as `Administrator`. The password is in
`local/site.env`.

- `Ctrl+C` stops Frappe. `dev/db.sh stop` stops the database.
- MariaDB runs only for this project, on `127.0.0.1:3307`, with its data in `local/mariadb`.
  It is not a system service.

## Updating Helpdesk from upstream

```bash
git fetch helpdesk-upstream main
git subtree pull --prefix=apps/helpdesk helpdesk-upstream main --squash
```

(`helpdesk-upstream` = https://github.com/frappe/helpdesk, fetch-only.)

## Status

🚧 Prototype under active development.

# kanban233

A lightweight Kanban web server for intranet teams. Built with Go 1.26, SQLite (Postgres optional), and a static HTML/JS frontend.

## Features

- **Project groups** — public/private, free join or apply-to-join
- **Kanban boards** — columns, drag-and-drop cards, mark columns as done
- **Research overview** — cross-project in-progress tasks in one view
- **Completed task history** — finished cards leave the board but stay traceable
- **Audit logs** — all mutating actions recorded for the whole team
- **Import / export** — export all projects or single boards; import single boards with version conflict rules
- **Agent API** — daily collaboration reports in Markdown for external agents and automation

## Quick Start

### Requirements

- Go 1.26+
- Windows: use `build.cmd`, `run-dev.cmd`, `test.cmd`
- Docker (optional)

### Local development

```cmd
run-dev.cmd
```

Open http://localhost:61333

Default account: **root** / **root** (seeded on first start).

### Build

```cmd
build.cmd
```

Output: `bin\kanban.exe`

### Tests

```cmd
test.cmd
```

## Configuration

Config file: `server.yaml` (override with `KANBAN_CONFIG` env var).

```yaml
server:
  addr: ":61333"
  static_dir: "./web"

auth:
  registration_open: true
  jwt_secret: change-me-in-production
  token_ttl_hours: 168
  default_user:
    username: root
    password: root

database:
  driver: sqlite          # sqlite | postgres
  dsn: "./data/kanban.db"

project:
  default_join_mode: free # free | apply

agent:
  enabled: true
  api_token: kanban-agent-intranet

locale:
  week_start: monday      # monday | sunday — first day of the week for reports
```

## Docker

### docker compose

```bash
docker compose up --build
```

Service listens on port **61333**. Data is stored in the `kanban-data` volume.

### Publish image (PowerShell)

```powershell
# Build only
.\docker-deploy-image.ps1

# Build and push to a registry
.\docker-deploy-image.ps1 -Registry registry.example.com/team -Tag 1.0.0 -Push

# Also tag and push :latest
.\docker-deploy-image.ps1 -Registry registry.example.com/team -Tag 1.0.0 -Push -PushLatest
```

Parameters:

| Parameter | Default | Description |
|-----------|---------|-------------|
| `-Registry` | _(empty)_ | Registry prefix, e.g. `docker.io/myorg` |
| `-ImageName` | `kanban233` | Image name |
| `-Tag` | `latest` | Image tag |
| `-Push` | — | Push image after build |
| `-PushLatest` | — | Also push `:latest` when `-Tag` is not `latest` |

## Agent API

When `agent.enabled: true`, external tools can fetch collaboration data and daily reports.

**Authentication** (any one):

- Header `X-Agent-Token: <api_token>`
- JWT from normal login
- HTTP Basic `root:root`

**Endpoints**

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/agent/collaboration?date=YYYY-MM-DD&user=` | JSON collaboration data |
| GET | `/api/agent/daily-report.md?date=&user=` | Markdown daily report |
| GET | `/api/agent/daily-report?format=markdown` | JSON with `markdown` field |

Card title format (recommended): `【Backend】100% summary text` — description is appended as detail.

Weekly range in reports uses `locale.week_start` (default **Monday**).

Example Markdown header:

```markdown
# 2026-05-26 周二
> 本周（周一起）：2026-05-26（周一）~ 2026-06-01（周日）
```

## UI

| Page | URL |
|------|-----|
| Login | `/login.html` |
| Workspace | `/board.html` — research overview (default) + project boards |

## CI/CD

- **GitHub Actions** — `go test`, build, Docker on push
- **Jenkins** — test → build → Docker publish on `main`/`master` (see `Jenkinsfile`)

## Project layout

```
cmd/kanban/          Entry point
internal/            config, db, api, auth, agent, locale, server
web/                 Static frontend
server.yaml          Default config
Dockerfile
docker-compose.yml
docker-deploy-image.ps1
```

## License

See repository for license details.

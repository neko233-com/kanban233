---
name: kanban233
description: >-
  Develop, build, test, and deploy the kanban233 Go 1.26 Kanban web server.
  Use when working on kanban233 features, server.yaml config, SQLite/Postgres
  persistence, project groups, audit logs, import/export, static web UI
  (login.html + board.html), Docker/Jenkins/GitHub Actions CI, or Windows
  build.cmd / run-dev.cmd / test.cmd scripts.
---

# kanban233 Project Skill

## Stack

- Go 1.26, stdlib `net/http`
- Static frontend: `web/login.html` (auth), `web/board.html` (workspace)
- Config: `server.yaml` (`KANBAN_CONFIG` env override)
- Default DB: SQLite; optional Postgres via pgx

## Layout

```
cmd/kanban/main.go
internal/config/ db/ auth/ api/ server/
web/login.html  web/board.html  web/common.js  web/auth.js  web/board.js
server.yaml
build.cmd  run-dev.cmd  test.cmd
Dockerfile  docker-compose.yml  Jenkinsfile  .github/workflows/ci.yml
```

## Domain Model

- **ProjectGroup**: `is_public`, `join_mode` (`free` 默认 | `apply`)
- **Members**: `project_group_members` — 自由加入或申请审批
- **Card**: `status` active/completed; 完结后不在看板显示，可查历史
- **Column**: `is_done` — 「完成」列触发归档

## Config (`server.yaml`)

```yaml
server:
  addr: ":61333"
project:
  default_join_mode: free
```

## Pages

| URL | Purpose |
|-----|---------|
| `/board.html` | 研发全视图（默认）+ 项目组看板 + 加入项目 + 完结历史 |

## API Highlights

**Research:** `GET /api/research/overview` — 研发全视图（跨项目进行中任务）

**Join:** `GET /api/groups/explore`, `POST /api/groups/{id}/join`
- `join_mode=free`: 立即成为成员
- `join_mode=apply`: 待 owner `POST .../applications/{uid}/approve|reject`

**History:** `GET /api/boards/{id}/history`, `GET /api/groups/{id}/history`

**Complete:** `POST /api/cards/{id}/complete` — 归档任务（移入 is_done 列）

**Groups:** `GET/POST /api/groups`, ...

**Boards:** `GET /api/boards`, `GET/PUT/DELETE /api/boards/{id}`, columns/cards CRUD

**Export (no bulk import):**
- `GET /api/export/all` — 当前账号全部项目 JSON（仅导出）
- `GET /api/boards/{id}/export` — 单个看板 JSON

**Import (single board only):**
- `POST /api/boards/import` — body: `BoardExportPayload`
- Match by `board.export_key`
- If exists: overwrite only when `exported_at` **newer** than local `last_export_at`
- If not exists: create group (by name) + new board

Export JSON fields: `export_version`, `export_type`, `exported_at`, `exported_by`

**Audit:** `GET /api/audit-logs?limit=100&offset=0` — 内网全员可查

**Agent（外部 Agent / 日报）** — `agent.enabled: true`
- 认证：`X-Agent-Token` / JWT / Basic `root:root`
- `GET /api/agent/collaboration?date=YYYY-MM-DD&user=`
- `GET /api/agent/daily-report.md?date=&user=` — Markdown 日报
- `GET /api/agent/daily-report?format=markdown` — JSON 含 markdown 字段
- 卡片标题推荐：`【后端】100% 任务摘要`（description 作 `- 细节`）
- 今日完结 → 正文列表；待办列 → `明天：` 段

## Default Account

- 用户名/密码：`root` / `root`（`auth.default_user`，首次启动 seed）

## Local Commands

```cmd
run-dev.cmd    # http://localhost:61333
test.cmd
build.cmd
```

## CI/CD

- GitHub Actions: test + build; Docker on push
- Jenkins: test → build → docker push (main/master)
- `docker compose up --build`

## Dev Notes

- Register auto-creates default group「我的项目」
- New boards: 待办 / 进行中 / 完成(is_done，看板不显示该列)
- 完结任务：`status=completed`，看板仅显示 active 卡片
- Audit all mutating API actions in handlers via `h.audit(...)`
- Keep JSON snake_case; minimal diffs

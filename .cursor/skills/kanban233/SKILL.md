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
build.cmd  run-dev.cmd  test.cmd  deploy-to-github.cmd  git-auto-up.cmd
Dockerfile  docker-compose.yml  docker-deploy-image.ps1  Jenkinsfile  .github/workflows/ci.yml
README.md  README-CN.md
```

## Domain Model

- **ProjectGroup**: `is_public`, `join_mode` (`free` 默认 | `apply`)
- **Members**: `project_group_members` — 自由加入或申请审批
- **Card**: `status` active / archived / completed；归档后不在看板列显示，计入统计与历史
- **Card 字段**: `workers`（工作人员，与账号无关）、`start_date`、`end_date`
- **Column**: `is_done` — 「完成」列触发归档

## Config (`server.yaml`)

```yaml
server:
  addr: ":61333"
project:
  default_join_mode: free
locale:
  week_start: monday      # monday | sunday
```

## Pages

| URL | Purpose |
|-----|---------|
| `/board.html` | 研发全视图（默认）+ 项目组看板 + 加入项目 + 完结历史 |

## API Highlights

**Research:** `GET /api/research/overview` — 研发全视图（跨项目进行中任务）
**Stats:** `GET /api/research/stats`、`GET /api/groups/{id}/stats` — ECharts 宏观数据（按工作人员/列/状态）

**Join:** `GET /api/groups/explore`, `POST /api/groups/{id}/join`
- `join_mode=free`: 立即成为成员
- `join_mode=apply`: 待 owner `POST .../applications/{uid}/approve|reject`

**History:** `GET /api/boards/{id}/history`, `GET /api/groups/{id}/history`

**Complete:** `POST /api/cards/{id}/complete` — 拖入完成列（status=completed）
**Archive:** `POST /api/cards/{id}/archive` — 归档（status=archived，不占看板列，计入统计）

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

**Audit:** `GET /api/audit-logs?limit=100&offset=0` — 内网全员可查；`GET /api/audit-logs/export?format=json|csv` — 导出

**Agent（外部 Agent / 日报 / AI 协作）** — `agent.enabled: true`
- 认证：`X-Agent-Token` / JWT / Basic `root:root`
- `GET /api/agent/capabilities` — 能力发现（含 task_categories）
- `GET /api/agent/collaboration?date=YYYY-MM-DD&user=`
- `GET /api/agent/daily-report.md?date=&user=` — Markdown 日报
- `GET /api/agent/research/overview?as=root` — 研发全视图
- `GET /api/agent/tasks?category=AI&status=active&q=` — 按分类/关键词搜任务
- `POST /api/agent/cards?as=root` — 创建任务（支持 category 自动 `【AI】` 标题）
- `PUT /api/agent/cards/{id}` / `POST .../move` / `POST .../complete`
- 卡片标题推荐：`【后端】100% 任务摘要`（description 作 `- 细节`）
- 列名识别含 AI/智能/研发 等；`locale.week_start` 默认 `monday`

## Default Account

- 用户名/密码：`root` / `root`（`auth.default_user`，首次启动 seed）
- 内置 demo：`入门教学` + `AI 协作示例` 公开项目组（首次启动自动创建，可教学）

## Local Commands

```cmd
run-dev.cmd    # http://localhost:61333，web 热重载（KANBAN_DEV=1）
test.cmd
build.cmd
deploy-to-github.cmd              # 测试通过后 commit + push 到 origin
deploy-to-github.cmd 修复说明     # 自定义 commit message
```

## CI/CD

- GitHub Actions: test + build; Docker on push
- Jenkins: test → build binary（无 Docker）；产物 `kanban`
- `docker compose up --build`
- `docker-deploy-image.ps1 -Registry <host> -Tag <ver> -Push [-PushLatest]`

## Dev Notes

- UI 审美见 `.cursor/skills/kanban233-ui/SKILL.md`（Apple HIG 深色风格）
- Register auto-creates default group「我的项目」
- **一项目一看板**：创建项目组时自动创建同名看板；`GET /api/groups/{id}/kanban` 直接进入看板详情
- New boards: 待办 / 进行中 / 完成(is_done，看板不显示该列)
- 完结任务：`status=completed`，看板仅显示 active 卡片
- Audit all mutating API actions in handlers via `h.audit(...)`
- Keep JSON snake_case; minimal diffs

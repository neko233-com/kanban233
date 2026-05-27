# kanban233

面向内网团队的轻量看板 Web 服务。Go 1.26 + SQLite（可选 Postgres）+ 静态 HTML/JS 前端。

## 功能

- **项目组** — 公开/私有，自由加入或申请审批
- **看板** — 列与卡片，支持拖拽，「完成」列归档任务
- **研发全视图** — 跨项目汇总进行中的任务
- **完结历史** — 已完成任务不在看板显示，但可查询历史
- **审计日志** — 内网全员可查看变更记录
- **导入 / 导出** — 全量导出；单看板导入，按 `export_key` 与时间戳冲突规则合并
- **Agent API** — 供外部 Agent / 脚本拉取协作数据与 Markdown 日报

## 快速开始

### 环境要求

- Go 1.26+
- Windows：使用 `build.cmd`、`run-dev.cmd`、`test.cmd`
- Docker（可选）

### 本地开发

```cmd
run-dev.cmd
```

浏览器访问 http://localhost:61333

默认账号：**root** / **root**（首次启动自动创建）。

### 编译

```cmd
build.cmd
```

产物：`bin\kanban.exe`

### 测试

```cmd
test.cmd
```

## 配置

配置文件：`server.yaml`（可用环境变量 `KANBAN_CONFIG` 指定路径）。

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
  week_start: monday      # monday | sunday — 周报/日报中「本周」的起始日，默认周一
```

## Docker

### docker compose

```bash
docker compose up --build
```

服务监听 **61333** 端口，数据保存在 `kanban-data` 卷中。

### 发布镜像（PowerShell）

```powershell
# 仅构建
.\docker-deploy-image.ps1

# 构建并推送到镜像仓库
.\docker-deploy-image.ps1 -Registry registry.example.com/team -Tag 1.0.0 -Push

# 同时推送 :latest 标签
.\docker-deploy-image.ps1 -Registry registry.example.com/team -Tag 1.0.0 -Push -PushLatest
```

参数说明：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-Registry` | 空 | 仓库前缀，如 `docker.io/myorg` |
| `-ImageName` | `kanban233` | 镜像名 |
| `-Tag` | `latest` | 镜像标签 |
| `-Push` | — | 构建后推送 |
| `-PushLatest` | — | 当 `-Tag` 不是 `latest` 时，额外推送 `:latest` |

## Agent API

`agent.enabled: true` 时，外部工具可获取协作数据与日报告。

**认证方式**（任选其一）：

- 请求头 `X-Agent-Token: <api_token>`
- 正常登录获得的 JWT
- HTTP Basic `root:root`

**接口**

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/agent/collaboration?date=YYYY-MM-DD&user=` | JSON 协作数据 |
| GET | `/api/agent/daily-report.md?date=&user=` | Markdown 日报 |
| GET | `/api/agent/daily-report?format=markdown` | JSON，含 `markdown` 字段 |

卡片标题推荐格式：`【后端】100% 任务摘要` — 描述会作为 `- 细节` 追加。

周报范围由 `locale.week_start` 控制，**默认周一为每周第一天**。

Markdown 示例：

```markdown
# 2026-05-26 周二
> 本周（周一起）：2026-05-26（周一）~ 2026-06-01（周日）
```

## 页面

| 页面 | 地址 |
|------|------|
| 登录 | `/login.html` |
| 工作台 | `/board.html` — 研发全视图（默认）+ 项目组看板 |

## CI/CD

- **GitHub Actions** — 测试、构建、推送 Docker 镜像
- **Jenkins** — 测试 → 构建 → 在 `main`/`master` 分支发布镜像（见 `Jenkinsfile`）

## 目录结构

```
cmd/kanban/          入口
internal/            config, db, api, auth, agent, locale, server
web/                 静态前端
server.yaml          默认配置
Dockerfile
docker-compose.yml
docker-deploy-image.ps1
```

## 许可证

见仓库说明。

# kanban233 — Agent 开发规范

面向在本仓库中协作的 AI Agent / 自动化工具。请与 `.cursor/skills/kanban233/SKILL.md` 一并阅读。

## 项目概览

- Go 1.26 Kanban Web 服务，静态前端在 `web/`
- 默认账号 `root` / `root`，端口 `:61333`
- 配置：`server.yaml`（`KANBAN_CONFIG` 可覆盖）

## 前端 UI 规范（强制）

详见 `.cursor/skills/kanban233-ui/SKILL.md`（Apple 深色审美、设计 token）。

### 禁止使用浏览器原生对话框

**不得**在 `web/` 中使用以下 API：

- `window.alert()`
- `window.confirm()`
- `window.prompt()`

原因：样式不可控、阻塞主线程、与深色主题不一致、移动端体验差。

### 应使用的替代方案

| 场景 | 做法 |
|------|------|
| 单行/多行输入 | 页面内 `<dialog>` 表单，或 `common.js` 的 `uiPrompt()` |
| 二次确认（删除等） | `<dialog>` + 确认/取消按钮，或 `uiConfirm()` |
| 操作结果提示 | 页面内 `#auth-error` 等区域，或 `uiToast()` |
| 复杂编辑 | 专用 `<dialog>`（参考 `board.html` 的 `group-dialog`、`card-dialog`） |

### 现有 dialog 约定

- 使用 HTML `<dialog>` + `showModal()` / `close()`
- 样式类：`.dialog-actions`、`.wide-dialog`（宽面板）
- 关闭按钮用 `type="button"`，提交用 `type="submit"` 或显式 `preventDefault`
- 文案默认中文，与页面语言一致

### 代码审查检查项

提交前在 `web/` 搜索，**不得出现**：

```text
alert(
confirm(
prompt(
```

## 后端规范

- JSON 字段 snake_case
- 变更写审计日志（`h.audit(...)`）
- 数据库迁移：先建表 → `migrateLegacy` 补列 → 再建索引
- Agent API 认证：`X-Agent-Token` / JWT / Basic `root:root`

## 测试与启动

```cmd
test.cmd
run-dev.cmd
```

新增功能需保证 `go test ./...` 通过；前端改动需手动验证登录与看板交互。

## 禁止事项

- 不要使用原生 `alert` / `confirm` / `prompt`
- 不要跳过数据库 legacy 迁移顺序
- 不要提交 `.env`、密钥或本地 `data/kanban.db`
- 不要擅自 force push `main` / `master`

---
name: kanban233-ui
description: >-
  Apple Human Interface-inspired UI for kanban233 web frontend. Use when
  editing web/styles.css, login.html, board.html, or any visual/interaction
  changes. Ensures premium dark-mode aesthetics, glass materials, segmented
  controls, and no native alert/confirm/prompt.
---

# kanban233 UI — Apple 审美规范

## 设计原则

1. **清晰** — 层级靠字号/字重/颜色，不靠粗边框
2. **克制** — 单一强调色（系统蓝 `#0A84FF`），状态色仅用于 badge/危险操作
3. **深度** — 背景分层 + 轻微阴影 + 毛玻璃顶栏，不用霓虹渐变
4. **动效** — 150–250ms ease，hover/active 微反馈

## 字体

```css
font-family: -apple-system, BlinkMacSystemFont, "SF Pro Display", "SF Pro Text",
  "Helvetica Neue", "PingFang SC", "Microsoft YaHei", sans-serif;
-webkit-font-smoothing: antialiased;
```

| 用途 | 大小 | 字重 |
|------|------|------|
| 大标题 | 28px | 700 |
| 区块标题 | 17px | 600 |
| 正文/按钮 | 15px | 400–500 |
| 说明/caption | 13px | 400，secondary 色 |

## 色板（Dark）

| Token | 值 | 用途 |
|-------|-----|------|
| `--bg-base` | `#000000` | 页面底 |
| `--bg-elevated` | `#1c1c1e` | 卡片/侧栏 |
| `--bg-secondary` | `#2c2c2e` | 列/输入框 |
| `--bg-tertiary` | `#3a3a3c` | 分段控件底 |
| `--label-primary` | `#ffffff` | 主文字 |
| `--label-secondary` | `rgba(235,235,245,0.6)` | 次要文字 |
| `--tint` | `#0a84ff` | 主操作 |
| `--separator` | `rgba(84,84,88,0.65)` | 分隔线 |

## 组件约定

- **顶栏**：`backdrop-filter: saturate(180%) blur(20px)` + 半透明底
- **主导航**：`.segmented-control` 分段控件，不用独立描边 pill
- **按钮**：`.btn.primary` 填充蓝；`.btn.ghost` 纯文字；`.btn` 灰色填充
- **卡片/列**：无边框或极淡 separator，圆角 12–16px
- **对话框**：`<dialog>` + `::backdrop` 模糊；禁止 `alert/confirm/prompt`
- **Toast**：右上角，毛玻璃小条

## 禁止

- 高饱和霓虹渐变、多重阴影堆叠
- 原生 `alert` / `confirm` / `prompt`
- 纯 `#4f8cff` 等非 Apple 系统色作为主色
- Segoe UI 作为首选字体

## 文件

```
web/styles.css    设计 token + 组件
web/login.html    认证页
web/board.html    工作台
web/common.js     uiPrompt / uiConfirm / uiToast
```

修改 UI 时只动 `web/`，保持 class 名稳定以免破坏 `board.js` 选择器。

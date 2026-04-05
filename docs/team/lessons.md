# Lessons Learned

P0 阶段踩坑复盘，供后续 session 和新成员参考。

## 1. Migration Embed FS 同步问题

**现象**: 迁移文件只存在于 `server/migrations/`，Go 二进制通过 `//go:embed` 从 `server/pkg/migrations/migrations/` 加载，导致 E2E 运行时表不存在。

**根因**: 两个目录必须保持同步。`server/migrations/` 是源码，`server/pkg/migrations/migrations/` 是 embed FS 源。修改迁移文件时必须同时更新两个位置。

**踩坑记录**:
- `039_outbox` / `038_team` 版本号冲突 — 远程分支和 main 使用了相同的版本号但不同的内容，导致 embed FS 中出现重复
- 修复方式：将 outbox 迁移重新编号为 `043`，同时添加到两个目录

**规则**: 修改任何 `*.sql` 迁移文件时，必须同时更新 `server/migrations/` 和 `server/pkg/migrations/migrations/`。

## 2. SameSite=Lax Cookie 问题

**现象**: E2E 测试 41/42 个失败，auth fixture 登录后 cookie 未设置，所有测试 redirect 到登录页。

**根因**: `loginAsDefault()` 从 Node.js 后端发起跨域 POST 到 `http://localhost:8080/auth/verify-code`。浏览器设置 `SameSite=Lax` 的 cookie 时，跨域 POST 请求不会携带 cookie，导致认证失败。

**修复**: 改用相对路径 `/auth/verify-code`，通过 Next.js dev proxy 同源请求，浏览器正确设置 cookie。

**规则**: E2E auth fixture 中的 API 调用必须走同源（相对路径），不能直接调用后端地址。

## 3. Migration FK 引用表名错误

**现象**: 迁移 `042_mcp_servers` 中 `REFERENCES workspaces(id)` (复数) 导致 FK 创建失败。

**根因**: 主表名是 `workspace` (单数)，但 FK 引用了 `workspaces` (复数)。

**规则**: 编写 FK 时，必须确认被引用表的确切名称（单复数）。

## 4. Go 代码引用表名与迁移表名不匹配

**现象**: `server/internal/events/outbox.go` 查询 `outbox` 表，但迁移 `039_run_orchestrator` 创建的是 `outbox_messages` 表。

**根因**: 两个独立的 outbox 实现使用了不同的表名，迁移移除了原始 `outbox` 表创建。

**规则**: Go SQL 查询中的表名必须与迁移文件中的 `CREATE TABLE` 名称一致。添加新表时，先 grep 确认没有命名冲突。

## 5. E2E 选择器过时

**现象**: 测试期望 `text=All Issues`，但 UI 重构后页面只有 "Issues" + "All" 两个独立文字。

**修复**: `text=All Issues` → `text=Issues`，5 处替换。

**规则**: UI 文案变更时，同步检查 E2E 选择器。优先使用语义化选择器（`data-testid`、`role`）而非文本匹配。

## 6. 跨文件函数签名变更级联失败

**现象**: `createTestApi()` 被改为 `createTestApi(token, workspaceId)`，但 `loginAsDefault()` 不返回值，导致 5 个 spec 文件运行时 TypeError。

**根因**: 一个 agent 修改了共享函数签名，但没有 grep 检查所有调用方。`e2e/comments.spec.ts`、`e2e/task-report.spec.ts`、`e2e/prompt-preview.spec.ts`、`e2e/runtime-policy.spec.ts`、`e2e/issues.spec.ts` 全部传入了未定义的参数。

**修复**: 恢复 `createTestApi()` 无参签名，所有 spec 文件回退到 `await createTestApi()`。

**规则**: 修改共享函数签名前，必须 grep 所有调用方并在同一 commit 中原子更新。签名变更和调用方更新不可拆分。

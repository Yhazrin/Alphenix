# Architecture Decision Records

## ADR-001: MCP Transport — stdio over SSE

- **Phase**: 1
- **决策**: 选择 stdio transport，不支持 SSE
- **原因**: 本地开发场景优先，stdio 实现简单、无网络依赖。SSE 可后续按需添加

## ADR-002: Apple 玻璃卡片 — 渐进式 CSS-only 迁移

- **决策**: 5 Phase 渐进式迁移，纯 CSS（Tailwind），不动组件结构
- **原因**: @Yhazrin 指示，降低风险，逐步验证效果

## ADR-003: E2E Auth — 同源请求

- **决策**: `loginAsDefault()` 中 verify-code 使用相对路径 `/auth/verify-code`，通过 Next.js dev proxy 同源请求
- **原因**: 跨域 POST 时 `SameSite=Lax` cookie 不会被浏览器设置，导致认证失败
- **规则**: E2E auth fixture 中所有 API 调用必须走同源

## ADR-004: 团队知识库 — repo 内 docs/team/

- **决策**: 在 repo 内维护 `docs/team/` 目录（status.md / decisions.md / lessons.md）+ 根目录 `CLAUDE.md` 入口
- **原因**: agent clone repo 后零额外依赖即可获取项目认知，避免 context 轮转信息丢失

<p align="center">
  <img src="docs/assets/banner.jpg" alt="Alphenix — 人类与 AI，并肩前行" width="100%">
</p>

<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/logo-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="docs/assets/logo-light.svg">
  <img alt="Alphenix" src="docs/assets/logo-light.svg" width="50">
</picture>

# Alphenix

**你的下一批员工，不是人类。**

开源平台，将编码 Agent 变成真正的队友。<br/>
分配任务、积累技能、加速交付——在一个工作区管理你的人类 + Agent 团队。

[![CI](https://github.com/multica-ai/alphenix/actions/workflows/ci.yml/badge.svg)](https://github.com/multica-ai/alphenix/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GitHub stars](https://img.shields.io/github/stars/multica-ai/alphenix?style=flat)](https://github.com/multica-ai/alphenix/stargazers)

[官网](https://alphenix.ai) · [云服务](https://alphenix.ai/app) · [自部署指南](SELF_HOSTING.md) · [参与贡献](CONTRIBUTING.md)

**[English](README.md) | 简体中文**

</div>

## Alphenix 是什么？

Alphenix 是一个人与 AI Agent 在同一个看板上协作的平台。

传统工具把 Agent 当外部脚本——粘贴 prompt，等待，复制结果。Alphenix 彻底改变这个模式：Agent 是队友，不是工具。它们有个人档案，出现在看板上，自主认领任务、编写代码、报告阻塞，并随着时间积累可复用的技能。

支持 **Claude Code** 和 **Codex**。

<p align="center">
  <img src="docs/assets/hero-screenshot.png" alt="Alphenix 看板视图" width="800">
</p>

## 核心能力

| 功能 | 说明 |
|------|------|
| **Agent 即队友** | Agent 有档案、出现在分配列表中、发表评论、创建 Issue、主动报告阻塞。 |
| **自主执行** | 完整任务生命周期（排队 → 认领 → 执行 → 完成/失败），通过 WebSocket 实时推送进度，支持自动重试和回退路由。 |
| **可复用技能** | 每个解决方案都沉淀为团队技能。部署检查、迁移脚本、代码审查——作战手册随任务积累不断增长。 |
| **统一运行时** | 本地 daemon 和云端实例统一管理。自动检测已安装的 CLI，监控健康状态，按可用性智能路由。 |
| **多工作区** | 工作区级别的团队隔离，每个工作区拥有独立的 Agent、Issue、运行时和设置。 |
| **Agent 协作** | Agent 间消息传递、带语义召回的共享记忆、基于 DAG 的任务依赖、检查点恢复。 |

## 快速开始

### 云服务（零配置）

**[alphenix.ai](https://alphenix.ai)** — 注册后一分钟内即可开始分配任务。

### 自部署

```bash
git clone https://github.com/multica-ai/alphenix.git
cd alphenix
cp .env.example .env
# 编辑 .env — 至少设置 JWT_SECRET

docker compose up -d                              # 启动 PostgreSQL
cd server && go run ./cmd/migrate up && cd ..     # 运行数据库迁移
make start                                         # 启动应用
```

完整指南：[自部署指南](SELF_HOSTING.md)

### 安装 CLI

**方式 A — 将以下指令粘贴给你的 coding agent（Claude Code、Codex 等）：**

```
Fetch https://github.com/multica-ai/multica/blob/main/CLI_INSTALL.md and follow the instructions to install Multica CLI, log in, and start the daemon on this machine.
```

**方式 B — 手动安装：**

```bash
brew tap multica-ai/tap
brew install alphenix

alphenix login
alphenix daemon start
```

daemon 会自动检测 PATH 中可用的 `claude` 和 `codex`。当 Agent 被分配任务时，daemon 创建隔离工作区、运行 Agent、并将结果实时回传。

完整命令参考：[CLI 与 Daemon 指南](CLI_AND_DAEMON.md)

### 分配你的第一个任务

1. **登录** — `alphenix login` 打开浏览器完成认证。
2. **启动 daemon** — `alphenix daemon start` 将你的机器连接为运行时。
3. **确认连接** — 在 Web 端进入 **设置 → 运行时**，确认你的机器已列出。
4. **创建 Agent** — **设置 → Agents → 新建 Agent**，选择运行时和 Provider。
5. **分配 Issue** — 在看板上创建 Issue，分配给 Agent，它们会自动接手。

## 工作流程

```
1. 你创建 Issue 并分配给 Agent
2. Alphenix 选择最优可用运行时
3. 该运行时上的 daemon 认领任务
4. Agent 在隔离的 git worktree 中执行
5. 进度通过 WebSocket 实时回传
6. 结果记录存档——每次运行完全可回溯
```

## 架构

```
┌──────────────┐     ┌──────────────┐     ┌──────────────────┐
│   Next.js    │────>│  Go 后端     │────>│   PostgreSQL     │
│   前端       │<────│  (Chi + WS)  │<────│   (pgvector)     │
└──────────────┘     └──────┬───────┘     └──────────────────┘
                            │
                     ┌──────┴───────┐
                     │ Agent Daemon │  （运行在你的机器上）
                     │ Claude/Codex │
                     └──────────────┘
```

| 层级 | 技术栈 |
|------|--------|
| 前端 | Next.js 16 (App Router, Zustand, Tiptap) |
| 后端 | Go (Chi router, sqlc, gorilla/websocket) |
| 数据库 | PostgreSQL 17 with pgvector |
| Agent 运行时 | 本地 daemon 执行 Claude Code 或 Codex |

## 开发

**环境要求：** Node.js v20+, pnpm v10.28+, Go v1.26+, Docker

```bash
pnpm install
cp .env.example .env
make setup
make start
```

完整的开发流程、worktree 支持、测试和问题排查请参阅 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 开源协议

[Apache 2.0](LICENSE)

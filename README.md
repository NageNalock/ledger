# Ledger

[中文](./README.md) | [English](./README.en.md)

一个带有预算、项目、报表与可选 AI 消费洞察的记账系统。

- 前端：Vue 3 + Vite + TypeScript + TailwindCSS + Element Plus
- 后端：Go + Gin + GORM（SQLite）

## 功能概览（按页面）

- **主页**：数据总览（本月/近 30 天口径切换）、快捷记一笔、常用模块入口
- **流水**：列表、新增/编辑/删除、筛选（日期、分类、来源、记账人、项目）、分页
- **报表**：趋势图（日/月）、分类饼图、商户排行
- **预算**：当前活跃预算、分类拆分、使用率跟踪、历史预算
- **项目**：项目列表、项目详情（按项目汇总流水）
- **导入账单**：微信/支付宝 CSV/Excel 导入到草稿箱，支持单条/批量 AI 辅助分类，再确认入账
- **AI 消费顾问**：基于指定范围流水生成消费洞察、标签、可优化建议
- **设置**：个人设置、系统设置（管理员：AI 提供方配置，支持检测连通性、获取模型列表）

## 快速开始（开发环境）

### 依赖

- Node.js 18+
- Go 1.25+

### 1）配置环境变量

从 `.env.example` 复制为 `.env`，并设置 `JWT_SECRET`（至少 32 位）。

```bash
cp .env.example .env
```

### 2）启动后端

```bash
set -a
source .env
set +a

go run ./api
```

默认后端地址：`http://localhost:8080`。

### 3）启动前端

```bash
npm install
npm run dev
```

开发环境下 `/api` 会代理到 `http://localhost:8080`。

## 用户初始化方式

本项目使用 JWT 登录。常见初始化方式：

- 开启注册（仅建议用于开发或可信部署）：设置 `ENABLE_SIGNUP=1`。
- 种子管理员（可控）：设置 `SEED_ADMIN=1` 并提供 `ADMIN_PASSWORD`（可选 `ADMIN_USERNAME`）。

## 配置项

常用环境变量：

- `JWT_SECRET`（必填）：JWT 签名密钥，长度 ≥ 32
- `PORT`（默认 `8080`）
- `DB_PATH`（默认 `ledger.db`）
- `CORS_ALLOW_ORIGINS`（默认 `http://localhost:5173`，逗号分隔；仅可信环境可使用 `*`）
- `ENABLE_SIGNUP`（`1` 时开启 `/api/auth/register`）
- `SEED_ADMIN`（`1` 时初始化管理员）
- `ADMIN_USERNAME`（默认 `admin`）
- `ADMIN_PASSWORD`（当 `SEED_ADMIN=1` 时必填）
- `SERVE_UPLOADS`（`1` 时静态暴露 `/uploads`，默认关闭；不建议生产开启）
- `STORE_IMPORT_RAW`（`1` 时在草稿里存导入原始行，默认关闭）

AI 配置：

- 在页面 `设置` → `系统设置`（仅管理员）里配置 AI 提供方（Base URL / API Key / 模型）。
- 后端从数据库读取 AI 配置，不再依赖环境变量。

## 安全与隐私说明

- 请勿提交 `.env`、数据库文件（`*.db`）、日志、上传文件。
- `JWT_SECRET` 未配置时后端会直接拒绝启动。
- 导入账单的草稿可能包含敏感字段；默认不存 `raw_data` 原始行。
- AI 功能可能会将部分交易信息发送到第三方模型服务。合规/隐私敏感场景建议关闭 AI 或避免使用。

## 常用脚本

- `npm run lint`
- `npm run build`
- `npm run seed:tx`（需要 `API_TOKEN` 或 `API_USER`/`API_PASS`）

## Shell 脚本（推荐）

仓库根目录下提供了若干脚本，适合用于本地开发/自托管部署的快捷操作：

- `./build.sh`：构建前端（`dist/`）与后端二进制（`ledger-server`）
- `./server_control.sh start|stop|restart|status`：用 PID 文件管理后端进程
- `./sync_code.sh`：拉取最新代码并重启服务（仅建议维护者使用，运行前请 review 变更）

示例：

```bash
chmod +x build.sh server_control.sh
./build.sh
./server_control.sh start
./server_control.sh status
```

## 数据迁移与 MCP

管理员可在设置中导出完整迁移备份；`/mcp` 支持携带记账账号密码调用流水增删改查工具。恢复步骤和 Codex 接入配置见 [MIGRATION_MCP.md](./MIGRATION_MCP.md)。

## License

AGPL-3.0-or-later，详见 [LICENSE](./LICENSE)。

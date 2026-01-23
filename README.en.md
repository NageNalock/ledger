# Ledger

[English](./README.en.md) | [中文](./README.md)

Personal finance ledger with budgets, projects, reports, and optional AI spending insights.

- Frontend: Vue 3 + Vite + TypeScript + TailwindCSS + Element Plus
- Backend: Go + Gin + GORM (SQLite)

## Features (high level)

- Home: overview (month / last 30 days), quick add transaction
- Transactions: list, add/edit/delete, filters, pagination
- Reports: trend (day/month), category pie, merchant ranking
- Budgets: active monthly budget, category splits, usage tracking
- Projects: project list, project detail (transaction aggregation)
- Import: WeChat/Alipay bill import into a draft inbox, with optional AI-assisted categorization before confirming
- AI Advisor: spending insights, labels, and optimization suggestions based on selected transactions
- Settings: personal settings, system settings (admin only: AI provider configuration with connectivity test and model list fetching)

## Quick Start (Development)

### Prerequisites

- Node.js 18+
- Go 1.24+

### 1) Configure environment

Create `.env` from `.env.example` and set `JWT_SECRET` (must be at least 32 characters).

```bash
cp .env.example .env
```

### 2) Start backend

```bash
set -a
source .env
set +a

go run ./api
```

Default backend address: `http://localhost:8080`.

### 3) Start frontend

```bash
npm install
npm run dev
```

The dev server proxies `/api` to `http://localhost:8080`.

## Bootstrapping Users

This project uses JWT auth. Common options:

- Enable signup (development / trusted deployments): set `ENABLE_SIGNUP=1`.
- Seed an admin account (controlled): set `SEED_ADMIN=1` and provide `ADMIN_PASSWORD` (and optionally `ADMIN_USERNAME`).

## AI Configuration

Configure AI provider (Base URL / API Key / Model) in the web UI: `Settings` → `System Settings` (admin only).
The backend reads AI settings from the database, not from environment variables.

## License

AGPL-3.0-or-later. See [LICENSE](./LICENSE).

## More details (Chinese)

See the main [README.md](./README.md) (in Chinese) for full configuration, security notes, and scripts.

#!/bin/bash
set -euo pipefail

echo "开始编译..."

if [ -f package.json ]; then
  echo "构建前端..."
  if [ -f package-lock.json ]; then
    npm ci
  else
    npm install
  fi
  VITE_BUILD_SSR=false \
  NODE_OPTIONS="--max-old-space-size=1024" \
  npm run build
fi

echo "构建后端..."
required_go_minor=24
go_version="$(go env GOVERSION 2>/dev/null || true)"
if [[ ! "$go_version" =~ ^go1\.([0-9]+)\..*$ ]]; then
  echo "无法检测 Go 版本，请确保已安装 Go 1.${required_go_minor}+" >&2
  exit 1
fi
go_minor="${BASH_REMATCH[1]}"
if [ "$go_minor" -lt "$required_go_minor" ]; then
  echo "Go 版本过低：$go_version（需要 Go 1.${required_go_minor}+）" >&2
  exit 1
fi

go mod download
go build -o ledger-server api/main.go

echo "编译完成。"

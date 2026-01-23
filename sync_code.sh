#!/bin/bash
set -euo pipefail

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "Working tree is dirty. Commit/stash changes before syncing." >&2
  exit 1
fi

git pull --rebase --autostash

./build.sh

./server_control.sh restart

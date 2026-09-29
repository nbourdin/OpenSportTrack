#!/bin/sh
set -eu

repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
snapshot=${1:-$repo}

if ! command -v gofmt >/dev/null 2>&1; then
  echo 'gofmt not found: install Go to check formatting.' >&2
  exit 1
fi

oxfmt="$repo/apps/web/node_modules/.bin/oxfmt"
if [ ! -x "$oxfmt" ]; then
  echo 'Oxfmt not found: run npm ci in apps/web.' >&2
  exit 1
fi

go_files=$(cd "$snapshot" && gofmt -l apps/api apps/simulator internal)
if [ -n "$go_files" ]; then
  printf 'Go files needing formatting:\n%s\n' "$go_files" >&2
  echo 'Run ./scripts/format.sh and commit the changes.' >&2
  exit 1
fi

(cd "$snapshot/apps/web" && "$oxfmt" --check .)

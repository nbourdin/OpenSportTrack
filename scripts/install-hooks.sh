#!/bin/sh
set -eu

repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
current=$(git -C "$repo" config --local --get core.hooksPath || true)
if [ -n "$current" ] && [ "$current" != '.githooks' ]; then
  printf 'Git hooks are already configured in %s; configuration unchanged.\n' "$current" >&2
  exit 1
fi
git -C "$repo" config --local core.hooksPath .githooks
echo 'Pre-push hook enabled for this repository.'

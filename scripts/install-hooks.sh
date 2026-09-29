#!/bin/sh
set -eu

repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
current=$(git -C "$repo" config --local --get core.hooksPath || true)
if [ -n "$current" ] && [ "$current" != '.githooks' ]; then
  printf 'Hooks Git déjà configurés dans %s ; configuration inchangée.\n' "$current" >&2
  exit 1
fi
git -C "$repo" config --local core.hooksPath .githooks
echo 'Hook pre-push activé pour ce dépôt.'

#!/bin/sh
set -eu

repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
snapshot=${1:-$repo}

if ! command -v gofmt >/dev/null 2>&1; then
  echo 'gofmt est introuvable : installez Go pour vérifier le format.' >&2
  exit 1
fi

oxfmt="$repo/apps/web/node_modules/.bin/oxfmt"
if [ ! -x "$oxfmt" ]; then
  echo 'Oxfmt est introuvable : lancez npm ci dans apps/web.' >&2
  exit 1
fi

go_files=$(cd "$snapshot" && gofmt -l apps/api apps/simulator internal)
if [ -n "$go_files" ]; then
  printf 'Fichiers Go à formater :\n%s\n' "$go_files" >&2
  echo 'Lancez ./scripts/format.sh puis intégrez les changements dans un commit.' >&2
  exit 1
fi

(cd "$snapshot/apps/web" && "$oxfmt" --check .)

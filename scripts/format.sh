#!/bin/sh
set -eu

repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo"

gofmt -w apps/api apps/simulator internal
npm --prefix apps/web run format

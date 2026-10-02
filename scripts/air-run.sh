#!/usr/bin/env bash

set -u

if [[ -f "$(dirname -- "${BASH_SOURCE[0]}")/../.env" ]]; then
	set -a
	# shellcheck disable=SC1091
	source "$(dirname -- "${BASH_SOURCE[0]}")/../.env"
	set +a
fi

root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

bins=(
	"$root/tmp/usuarios-service"
	"$root/tmp/ferias-service"
	"$root/tmp/servidor-service"
	"$root/tmp/contracheque-service"
)

pids=()
for bin in "${bins[@]}"; do
	if [[ ! -x "$bin" ]]; then
		printf 'binário ausente: %s\n' "$bin" >&2
		exit 1
	fi
	"$bin" &
	pids+=("$!")
done

status=0
if ! wait -n; then
	status=$?
fi

for pid in "${pids[@]}"; do
	kill -TERM "$pid" 2>/dev/null || true
done
for pid in "${pids[@]}"; do
	wait "$pid" 2>/dev/null || true
done

exit "$status"

#!/usr/bin/env bash

set -Eeuo pipefail

PROJECT_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
RUNTIME_DIR="$PROJECT_ROOT/.runtime"
BIN_DIR="$RUNTIME_DIR/bin"
PID_DIR="$RUNTIME_DIR/pids"
LOG_DIR="$RUNTIME_DIR/logs"
SERVICES=("usuarios-service" "ferias-service" "servidor-service" "contracheque-service" "chefe-service")

mkdir -p "$BIN_DIR" "$PID_DIR" "$LOG_DIR"

# Variáveis de ambiente locais (MSSQL_DSN, MYSQL_DSN, JWT_SIGNING_KEY...). Nunca versionar o .env.
if [[ -f "$PROJECT_ROOT/.env" ]]; then
	set -a
	# shellcheck disable=SC1091
	source "$PROJECT_ROOT/.env"
	set +a
fi

show_usage() {
	printf '%s\n' \
		"Uso: ./servicos.sh <comando> [serviço]" \
		"" \
		"Comandos:" \
		"  iniciar                  Compila e inicia todos os serviços" \
		"  iniciar <serviço>        Compila e inicia somente um serviço" \
		"  parar <serviço>          Para somente um serviço" \
		"  parar tudo               Para todos os serviços" \
		"  reiniciar <serviço>      Reinicia somente um serviço" \
		"  status                   Mostra PID, porta e estado" \
		"  logs <serviço>           Acompanha o log de um serviço" \
		"  compilar                 Compila todos os binários" \
		"  testar                   Executa todos os testes" \
		"  benchmark                Executa benchmarks Go internos" \
		"  ajuda                    Exibe esta ajuda" \
		"" \
		"Serviços: usuarios | ferias | servidor | contracheque | chefe"
}

require_command() {
	local command_name="$1"
	if ! command -v "$command_name" >/dev/null 2>&1; then
		printf 'Erro: o comando %s não está instalado ou não está no PATH.\n' "$command_name" >&2
		exit 1
	fi
}

normalize_service() {
	case "${1:-}" in
		usuarios|usuarios-service) printf '%s\n' "usuarios-service" ;;
		ferias|ferias-service) printf '%s\n' "ferias-service" ;;
		servidor|servidor-service) printf '%s\n' "servidor-service" ;;
		contracheque|contracheque-service) printf '%s\n' "contracheque-service" ;;
		chefe|chefe-service) printf '%s\n' "chefe-service" ;;
		*)
			printf 'Erro: serviço inválido: %s\n' "${1:-não informado}" >&2
			return 1
			;;
	esac
}

service_port() {
	case "$1" in
		usuarios-service) printf '%s\n' "8081" ;;
		ferias-service) printf '%s\n' "8082" ;;
		servidor-service) printf '%s\n' "8083" ;;
		contracheque-service) printf '%s\n' "8084" ;;
		chefe-service) printf '%s\n' "8085" ;;
	esac
}

binary_path() {
	printf '%s/%s\n' "$BIN_DIR" "$1"
}

pid_path() {
	printf '%s/%s.pid\n' "$PID_DIR" "$1"
}

log_path() {
	printf '%s/%s.log\n' "$LOG_DIR" "$1"
}

read_service_pid() {
	local service="$1"
	local file
	local pid
	file="$(pid_path "$service")"

	[[ -f "$file" ]] || return 1
	read -r pid < "$file"
	[[ "$pid" =~ ^[0-9]+$ ]] || return 1
	printf '%s\n' "$pid"
}

service_running() {
	local service="$1"
	local pid
	local command_line
	local binary

	pid="$(read_service_pid "$service")" || return 1
	kill -0 "$pid" 2>/dev/null || return 1

	binary="$(binary_path "$service")"
	command_line="$(ps -p "$pid" -o args= 2>/dev/null || true)"
	[[ "$command_line" == *"$binary"* ]]
}

health_ready() {
	local service="$1"
	local port
	port="$(service_port "$service")"

	if command -v curl >/dev/null 2>&1; then
		curl --silent --show-error --fail --output /dev/null --max-time 1 \
			"http://127.0.0.1:${port}/healthz" 2>/dev/null
	else
		service_running "$service"
	fi
}

build_service() {
	local service="$1"
	local output
	output="$(binary_path "$service")"

	printf 'Compilando %s...\n' "$service"
	(
		cd "$PROJECT_ROOT/$service"
		CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$output" ./cmd/api
	)
}

build_all() {
	local service
	for service in "${SERVICES[@]}"; do
		build_service "$service"
	done
}

start_without_build() {
	local service="$1"
	local binary
	local pid_file
	local output_log
	local port
	local pid
	local attempt

	if service_running "$service"; then
		printf '%s já está ativo.\n' "$service"
		return 0
	fi

	binary="$(binary_path "$service")"
	pid_file="$(pid_path "$service")"
	output_log="$(log_path "$service")"
	port="$(service_port "$service")"

	if [[ ! -x "$binary" ]]; then
		printf 'Erro: binário não encontrado: %s\n' "$binary" >&2
		return 1
	fi

	printf 'Iniciando %s na porta %s...\n' "$service" "$port"
	PORT="$port" nohup "$binary" >> "$output_log" 2>&1 &
	pid=$!
	printf '%s\n' "$pid" > "$pid_file"

	for ((attempt = 1; attempt <= 40; attempt++)); do
		if health_ready "$service"; then
			printf '%s iniciado com PID %s.\n' "$service" "$pid"
			return 0
		fi
		if ! service_running "$service"; then
			printf 'Erro: %s encerrou durante a inicialização. Consulte %s.\n' \
				"$service" "$output_log" >&2
			return 1
		fi
		sleep 0.5
	done

	printf 'Erro: %s não ficou saudável dentro do tempo esperado.\n' "$service" >&2
	return 1
}

start_service() {
	local service="$1"
	if service_running "$service"; then
		printf '%s já está ativo.\n' "$service"
		return 0
	fi
	build_service "$service"
	start_without_build "$service"
}

start_all() {
	local service
	build_all
	for service in "${SERVICES[@]}"; do
		start_without_build "$service"
	done
}

stop_service() {
	local service="$1"
	local pid
	local pid_file
	local attempt

	pid_file="$(pid_path "$service")"
	if ! service_running "$service"; then
		printf '%s já está parado.\n' "$service"
		rm -f "$pid_file"
		return 0
	fi

	pid="$(read_service_pid "$service")"
	printf 'Parando %s, PID %s...\n' "$service" "$pid"
	kill -TERM "$pid"

	for ((attempt = 1; attempt <= 40; attempt++)); do
		if ! kill -0 "$pid" 2>/dev/null; then
			rm -f "$pid_file"
			printf '%s foi parado.\n' "$service"
			return 0
		fi
		sleep 0.25
	done

	printf '%s não encerrou no prazo; finalizando o PID validado.\n' "$service" >&2
	kill -KILL "$pid"
	rm -f "$pid_file"
}

stop_all() {
	local service
	local result=0
	for service in "${SERVICES[@]}"; do
		if ! stop_service "$service"; then
			result=1
		fi
	done
	return "$result"
}

show_status() {
	local service
	local pid
	for service in "${SERVICES[@]}"; do
		if service_running "$service"; then
			pid="$(read_service_pid "$service")"
			printf '%-20s ATIVO   PID %-8s porta %s\n' \
				"$service" "$pid" "$(service_port "$service")"
		else
			printf '%-20s PARADO  porta %s\n' "$service" "$(service_port "$service")"
		fi
	done
}

run_tests() {
	local module
	local modules=("shared-common" "usuarios-service" "ferias-service" "servidor-service" "contracheque-service" "chefe-service")
	for module in "${modules[@]}"; do
		printf 'Testando %s...\n' "$module"
		(cd "$PROJECT_ROOT/$module" && go test ./...)
	done
}

run_benchmarks() {
	local service
	for service in "${SERVICES[@]}"; do
		printf 'Benchmark interno de %s...\n' "$service"
		(cd "$PROJECT_ROOT/$service" && go test -run '^$' -bench . -benchmem ./internal/api)
	done
}

main() {
	local command_name="${1:-ajuda}"
	local target="${2:-}"
	local service

	case "$command_name" in
		iniciar|start)
			require_command go
			require_command ps
			if [[ -z "$target" || "$target" == "tudo" || "$target" == "all" ]]; then
				start_all
			else
				service="$(normalize_service "$target")"
				start_service "$service"
			fi
			;;
		parar|stop)
			require_command ps
			if [[ "$target" == "tudo" || "$target" == "all" ]]; then
				stop_all
			elif [[ -n "$target" ]]; then
				service="$(normalize_service "$target")"
				stop_service "$service"
			else
				printf '%s\n' "Erro: informe um serviço ou use 'parar tudo'." >&2
				exit 1
			fi
			;;
		reiniciar|restart)
			require_command go
			require_command ps
			service="$(normalize_service "$target")"
			stop_service "$service"
			start_service "$service"
			;;
		status)
			require_command ps
			show_status
			;;
		logs|log)
			require_command tail
			service="$(normalize_service "$target")"
			touch "$(log_path "$service")"
			tail -f "$(log_path "$service")"
			;;
		compilar|build)
			require_command go
			build_all
			;;
		testar|test)
			require_command go
			run_tests
			;;
		benchmark|bench)
			require_command go
			run_benchmarks
			;;
		ajuda|help|-h|--help)
			show_usage
			;;
		*)
			printf 'Erro: comando inválido: %s\n' "$command_name" >&2
			show_usage >&2
			exit 1
			;;
	esac
}

main "$@"

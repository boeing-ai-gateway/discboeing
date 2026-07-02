#!/bin/sh
set -eu

AGENT_ENV_FILE="${DISCBOEING_AGENT_ENV_FILE:-/run/discboeing/agent-env}"
WORKSPACE_ENV_FILE="${DISCBOEING_WORKSPACE_ENV_FILE:-/workspace/.discboeing/env}"
CR=$(printf '\r')

warn_invalid_env_line() {
	env_file=$1
	line_number=$2
	echo "discboeing-session-env: warning: ignoring invalid env line ${line_number} in ${env_file}" >&2
}

warn_unreadable_env_file() {
	env_file=$1
	echo "discboeing-session-env: warning: cannot read env file ${env_file}" >&2
}

trim_leading_whitespace() {
	value=$1
	while :; do
		case "$value" in
			' '*)
				value=${value# }
				;;
			'	'*)
				value=${value#	}
				;;
			*)
				break
				;;
		esac
	done
	printf '%s' "$value"
}

trim_trailing_whitespace() {
	value=$1
	while :; do
		case "$value" in
			*' ')
				value=${value% }
				;;
			*'	')
				value=${value%	}
				;;
			*)
				break
				;;
		esac
	done
	printf '%s' "$value"
}

load_env_file() {
	env_file=$1
	if [ ! -f "$env_file" ]; then
		return 0
	fi
	if [ ! -r "$env_file" ]; then
		warn_unreadable_env_file "$env_file"
		return 0
	fi

	line_number=0
	# shellcheck disable=SC2094
	while IFS= read -r line || [ -n "$line" ]; do
		line_number=$((line_number + 1))

		case "$line" in
			*"$CR")
				line=${line%"$CR"}
				;;
		esac

		trimmed=$(trim_leading_whitespace "$line")
		case "$trimmed" in
			""|\#*)
				continue
				;;
		esac

		case "$trimmed" in
			export\ *)
				trimmed=${trimmed#export }
				trimmed=$(trim_leading_whitespace "$trimmed")
				;;
		esac

		case "$trimmed" in
			*=*)
				;;
			*)
				warn_invalid_env_line "$env_file" "$line_number"
				continue
				;;
		esac

		key=${trimmed%%=*}
		value=${trimmed#*=}
		key=$(trim_trailing_whitespace "$key")
		value=$(trim_leading_whitespace "$value")

		case "$key" in
			""|[0-9]*|*[!ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_]*)
				warn_invalid_env_line "$env_file" "$line_number"
				continue
				;;
		esac

		case "$value" in
			\"*\")
				value=${value#\"}
				value=${value%\"}
				;;
			\"*)
				warn_invalid_env_line "$env_file" "$line_number"
				continue
				;;
			\'*\')
				value=${value#\'}
				value=${value%\'}
				;;
			\'*)
				warn_invalid_env_line "$env_file" "$line_number"
				continue
				;;
		esac

		export "$key=$value"
	done < "$env_file"
}

preserve_keys=""
if [ "${1:-}" = "--preserve" ]; then
	preserve_keys="${2:-}"
	shift 2
fi

if [ "${1:-}" = "--" ]; then
	shift
fi

if [ "$#" -eq 0 ]; then
	echo "discboeing-session-env: command is required" >&2
	exit 2
fi

restore_file=""
cleanup() {
	if [ -n "$restore_file" ] && [ -f "$restore_file" ]; then
		rm -f "$restore_file"
	fi
}
trap cleanup EXIT INT TERM HUP

if [ -n "$preserve_keys" ]; then
	restore_file=$(mktemp)
	old_ifs=$IFS
	IFS=,
	for key in $preserve_keys; do
		if [ -z "$key" ]; then
			continue
		fi
		if env | grep -q "^${key}="; then
			value=$(printenv "$key")
			printf '%s\t%s\n' "$key" "$value" >> "$restore_file"
		fi
	done
	IFS=$old_ifs
fi

for env_file in "$AGENT_ENV_FILE" "$WORKSPACE_ENV_FILE"; do
	load_env_file "$env_file"
done

if [ -n "$restore_file" ] && [ -f "$restore_file" ]; then
	while IFS='	' read -r key value; do
		export "$key=$value"
	done < "$restore_file"
fi

exec "$@"

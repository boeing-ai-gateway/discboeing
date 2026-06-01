#!/usr/bin/env bash
set -Eeuo pipefail

DEFAULT_USER=discobot
PROXY_BINARY=/opt/discobot/bin/proxy
PROXY_PORT=17080
DATA_DIR=/.data
BASE_HOME_DIR=/.data/discobot
WORKSPACE_DIR=/.data/discobot/workspace
STAGING_DIR=/.data/discobot/workspace.staging
OVERLAYFS_DIR=/.data/.overlayfs
MOUNT_HOME=/home/discobot

log() {
	printf 'discobot-sandbox-init: %s\n' "$*"
}

warn() {
	log "warning: $*"
}

user_field() {
	local user=$1 field=$2
	getent passwd "$user" | cut -d: -f"$field"
}

require_user() {
	local user=$1
	if ! getent passwd "$user" >/dev/null; then
		printf 'failed to lookup user %s\n' "$user" >&2
		return 1
	fi
}

user_uid() { user_field "$1" 3; }
user_gid() { user_field "$1" 4; }
user_home() { user_field "$1" 6; }

fix_localhost_resolution() {
	python3 - <<'PY'
from pathlib import Path

path = Path('/etc/hosts')
lines = path.read_text().splitlines()
out = []
modified = False
has_ipv4 = False
for line in lines:
    stripped = line.strip()
    if not stripped or stripped.startswith('#'):
        out.append(line)
        continue
    fields = stripped.split()
    if len(fields) < 2 or 'localhost' not in fields[1:]:
        out.append(line)
        continue
    ip, names = fields[0], fields[1:]
    if ip == '127.0.0.1':
        has_ipv4 = True
        out.append(line)
    elif ip == '::1':
        remaining = [name for name in names if name != 'localhost']
        if remaining:
            out.append(ip + '\t' + ' '.join(remaining))
        modified = True
        print("discobot-sandbox-init: removed 'localhost' from ::1 line in /etc/hosts")
    else:
        out.append(line)
if not has_ipv4:
    out.insert(0, '127.0.0.1\tlocalhost')
    modified = True
    print("discobot-sandbox-init: added '127.0.0.1 localhost' to /etc/hosts")
if modified:
    path.write_text('\n'.join(out) + '\n')
    print('discobot-sandbox-init: /etc/hosts updated to ensure localhost resolves to 127.0.0.1')
else:
    print('discobot-sandbox-init: /etc/hosts already configured correctly for localhost')
PY
}

fix_mtu_for_nested_docker() {
	sysctl -w net.ipv4.ip_no_pmtu_disc=1 >/dev/null
	sysctl -w net.ipv4.tcp_mtu_probing=1 >/dev/null
	log 'configured TCP MTU probing for nested Docker (PMTUD disabled, TCP probing enabled)'
}

setup_git_safe_directories() {
	local dirs=(
		/.workspace
		/.workspace/.git
		"$WORKSPACE_DIR"
		"$STAGING_DIR"
		"$MOUNT_HOME/workspace"
	)
	log 'configuring git safe.directory for workspace paths'
	local dir
	for dir in "${dirs[@]}"; do
		git config --system --add safe.directory "$dir" || warn "git config safe.directory $dir failed"
	done
}

setup_base_home() {
	local user=$1 uid gid
	uid=$(user_uid "$user")
	gid=$(user_gid "$user")
	if [[ -d $BASE_HOME_DIR ]]; then
		log "base home already exists at $BASE_HOME_DIR, syncing new files"
		mkdir -p "$BASE_HOME_DIR"
		cp -a -n "$MOUNT_HOME/." "$BASE_HOME_DIR/"
	else
		log "copying $MOUNT_HOME to $BASE_HOME_DIR"
		mkdir -p "$(dirname "$BASE_HOME_DIR")"
		cp -a "$MOUNT_HOME" "$BASE_HOME_DIR"
		log 'base home created successfully'
	fi
	chown -R "$uid:$gid" "$BASE_HOME_DIR"
}

remove_obsolete_bundled_home_config() {
	local home=$1 rel
	for rel in \
		.discobot/scripts/discobot-commit \
		.discobot/scripts/discobot-commit-remote \
		.discobot/scripts/discobot-rebase \
		.discobot/commands/discobot-commit.md \
		.discobot/commands/discobot-commit-remote.md \
		.discobot/commands/discobot-rebase.md \
		.discobot/skills/browser-harness/SKILL.md
	do
		rm -rf "${home:?}/$rel"
	done
	rm -rf "${home:?}/.claude/commands"
}

setup_overlayfs() {
	local session_id=$1 user=$2 uid gid session_dir upper_dir work_dir
	uid=$(user_uid "$user")
	gid=$(user_gid "$user")
	session_dir="$OVERLAYFS_DIR/$session_id"
	upper_dir="$session_dir/upper"
	work_dir="$session_dir/work"
	log "setting up overlayfs directories at $session_dir"
	mkdir -p "$upper_dir" "$work_dir"
	chown "$uid:$gid" "$session_dir" "$upper_dir" "$work_dir"
	log 'overlayfs directories created successfully'
}

mount_overlayfs() {
	local session_id=$1 session_dir upper_dir work_dir opts
	if findmnt -n "$MOUNT_HOME" >/dev/null; then
		log "overlayfs already mounted at $MOUNT_HOME"
		return 0
	fi
	session_dir="$OVERLAYFS_DIR/$session_id"
	upper_dir="$session_dir/upper"
	work_dir="$session_dir/work"
	opts="lowerdir=$BASE_HOME_DIR,upperdir=$upper_dir,workdir=$work_dir"
	log "mounting overlayfs at $MOUNT_HOME"
	log "overlayfs options: $opts"
	mount -t overlay overlay -o "$opts" "$MOUNT_HOME"
	log 'overlayfs mounted successfully'
}

ensure_workspace_directory() {
	local user=$1 uid gid dir
	uid=$(user_uid "$user")
	gid=$(user_gid "$user")
	dir="$MOUNT_HOME/workspace"
	mkdir -p "$dir"
	chown "$uid:$gid" "$dir"
}

proxy_env_lines() {
	local proxy_url="http://localhost:$PROXY_PORT"
	local ca_cert_path="$DATA_DIR/proxy/certs/ca.crt"
	cat <<EOF
HTTP_PROXY=$proxy_url
HTTPS_PROXY=$proxy_url
http_proxy=$proxy_url
https_proxy=$proxy_url
ALL_PROXY=$proxy_url
all_proxy=$proxy_url
NO_PROXY=localhost,127.0.0.1,::1
no_proxy=localhost,127.0.0.1,::1
NODE_EXTRA_CA_CERTS=$ca_cert_path
UV_SYSTEM_CERTS=1
EOF
}

write_default_proxy_config() {
	cat <<'EOF'
# Default Discobot Proxy Configuration
# Written by discobot-sandbox-init during sandbox startup.

proxy:
  port: 17080
  api_port: 17081

tls:
  cert_dir: /.data/proxy/certs

cache:
  enabled: true
  dir: /.data/cache/proxy
  max_size: 21474836480
  content_aware: true
  patterns:
    - "^/registry-v2/docker/registry/v2/blobs/sha256/"

allowlist:
  enabled: false

headers: {}

logging:
  level: info
  format: text

recording:
  enabled: true
  dir: /.data/proxy/recordings
  max_body_size: 10485760
EOF
}

setup_proxy_config() {
	local proxy_data_dir="$DATA_DIR/proxy"
	mkdir -p "$proxy_data_dir"
	log 'using default proxy config with Docker caching enabled'
	write_default_proxy_config > "$proxy_data_dir/config.yaml"
	chmod 0644 "$proxy_data_dir/config.yaml"
}

setup_proxy_certificate() {
	local user=$1
	"$PROXY_BINARY" init-certs -config "$DATA_DIR/proxy/config.yaml" -user "$user"
}

set_proxy_in_profile() {
	local profile_path=/etc/profile.d/discobot-proxy.sh
	mkdir -p /etc/profile.d
	{
		printf '%s\n' '# Discobot Proxy Configuration'
		printf '%s\n' '# Automatically generated by discobot-sandbox-init'
		proxy_env_lines | sed 's/^/export /'
	} > "$profile_path"
	chmod 0644 "$profile_path"
	log "proxy settings written to $profile_path"
}

write_proxy_environment_file() {
	if [[ ! -x $PROXY_BINARY ]]; then
		printf 'proxy binary not found: %s\n' "$PROXY_BINARY" >&2
		return 1
	fi
	mkdir -p /run/discobot
	proxy_env_lines > /run/discobot/proxy-env
	chmod 0644 /run/discobot/proxy-env
	log 'proxy environment written to /run/discobot/proxy-env'
	set_proxy_in_profile || warn 'failed to set proxy in /etc/profile.d'
}

write_agent_environment_file() {
	local user=$1 home
	home=$(user_home "$user")
	mkdir -p /run/discobot
	{
		env | grep -Ev '^(HOME|USER|LOGNAME|SESSION_ID)=' || true
		printf 'HOME=%s\n' "$home"
		printf 'USER=%s\n' "$user"
		printf 'LOGNAME=%s\n' "$user"
		printf 'DISCOBOT_HOOKS_ENABLED=true\n'
		proxy_env_lines
	} > /run/discobot/agent-env
	chmod 0644 /run/discobot/agent-env
	log 'agent environment written to /run/discobot/agent-env'
}

write_docker_daemon_config() {
	command -v dockerd >/dev/null
	local current_mtu docker_mtu
	current_mtu=$(< /sys/class/net/eth0/mtu)
	docker_mtu=$((current_mtu - 100))
	if ((docker_mtu < 1200)); then
		docker_mtu=1200
	fi
	mkdir -p /etc/docker "$DATA_DIR/docker"
	cat > /etc/docker/daemon.json <<EOF
{
  "features": {
    "containerd-snapshotter": true
  },
  "mtu": $docker_mtu
}
EOF
	log "configured Docker daemon with MTU=$docker_mtu (interface MTU: $current_mtu, overhead: 100)"
}

cache_paths() {
	cat <<'EOF'
/home/discobot/.cache
/home/discobot/.npm
/home/discobot/.pnpm-store
/home/discobot/.yarn
/home/discobot/.local/share/uv
/home/discobot/go/pkg/mod
/home/discobot/.cargo/registry
/home/discobot/.cargo/git
/home/discobot/.rustup
/home/discobot/.bundle
/home/discobot/.gem
/home/discobot/.m2/repository
/home/discobot/.gradle/caches
/home/discobot/.gradle/wrapper
/home/discobot/.nuget/packages
/home/discobot/.composer/cache
/nix
/var/cache/apt
/home/discobot/.ccache
/home/discobot/.config/JetBrains
/home/discobot/.local/share/JetBrains/Toolbox/apps
/home/discobot/.local/share/JetBrains/Daemon/bundles
/home/discobot/.local/share/discobot-code-server/Machine
/home/discobot/.local/share/discobot-code-server/extensions
/home/discobot/.vscode-server
/home/discobot/.cursor-server
/home/discobot/.zed_server
EOF
	local cfg="$MOUNT_HOME/workspace/.discobot/cache.json"
	if [[ -f $cfg ]]; then
		jq -r '.additionalPaths[]? // empty' "$cfg" 2>/dev/null | while IFS= read -r path; do
			case "$(realpath -m "$path")" in
				/home/discobot/*) printf '%s\n' "$(realpath -m "$path")" ;;
				*) warn "ignoring invalid cache path from config: $path" ;;
			esac
		done
	fi
}

chmod_path_to_root() {
	local path=$1 root=$2 mode=$3 current
	current=$(realpath -m "$path")
	root=$(realpath -m "$root")
	while [[ $current != "$root" && $current != / && $current != . ]]; do
		chmod "$mode" "$current" 2>/dev/null || break
		current=$(dirname "$current")
	done
}

mount_cache_directories() {
	if [[ ${CACHE_ENABLED:-} == false ]]; then
		log 'cache volumes disabled via CACHE_ENABLED=false'
		return 0
	fi
	local cache_volume_base="$DATA_DIR/cache"
	if [[ ! -d $cache_volume_base ]]; then
		log "cache volume not found at $cache_volume_base, skipping cache mounts"
		return 0
	fi
	local mounted=0 cache_path sub_dir source target_root
	while IFS= read -r cache_path; do
		[[ -n $cache_path ]] || continue
		sub_dir=${cache_path#/}
		source="$cache_volume_base/$sub_dir"
		mkdir -p "$source" "$cache_path" || {
			warn "failed to create cache mount directories for $cache_path"
			continue
		}
		chmod_path_to_root "$source" "$cache_volume_base" 0777
		target_root=/home/discobot
		if [[ $cache_path != /home/discobot/* ]]; then
			target_root=$(dirname "$cache_path")
		fi
		chmod_path_to_root "$cache_path" "$target_root" 0777
		if findmnt -n "$cache_path" >/dev/null; then
			((mounted += 1))
			continue
		fi
		if mount --bind "$source" "$cache_path"; then
			((mounted += 1))
		else
			warn "failed to bind mount $source to $cache_path"
		fi
	done < <(cache_paths)
	if ((mounted > 0)); then
		log "mounted $mounted cache directories"
	fi
}

notify_ready() {
	if command -v systemd-notify >/dev/null && [[ -n ${NOTIFY_SOCKET:-} ]]; then
		systemd-notify --ready || warn 'sd_notify failed'
	fi
}

run_setup() {
	local startup_start run_as_user session_id step_start step_end elapsed
	startup_start=$(date +%s%3N)
	log "setup beginning at $(date --iso-8601=seconds)"
	cd /

	fix_localhost_resolution || warn 'failed to fix localhost resolution'
	fix_mtu_for_nested_docker || warn 'failed to fix MTU for nested Docker'

	run_as_user=${AGENT_USER:-$DEFAULT_USER}
	session_id=${DISCOBOT_SESSION_ID:-}
	if [[ -z $session_id ]]; then
		printf 'DISCOBOT_SESSION_ID environment variable is required\n' >&2
		return 1
	fi
	require_user "$run_as_user"

	step_start=$(date +%s%3N); setup_git_safe_directories; step_end=$(date +%s%3N); elapsed=$((step_end - step_start)); printf 'discobot-sandbox-init: [%d.%03ds] git safe.directory setup completed\n' "$((elapsed / 1000))" "$((elapsed % 1000))"
	step_start=$(date +%s%3N); setup_base_home "$run_as_user"; remove_obsolete_bundled_home_config "$BASE_HOME_DIR"; step_end=$(date +%s%3N); elapsed=$((step_end - step_start)); printf 'discobot-sandbox-init: [%d.%03ds] base home setup completed\n' "$((elapsed / 1000))" "$((elapsed % 1000))"

	step_start=$(date +%s%3N)
	log 'using OverlayFS'
	setup_overlayfs "$session_id" "$run_as_user"
	mount_overlayfs "$session_id"
	ensure_workspace_directory "$run_as_user"
	step_end=$(date +%s%3N); elapsed=$((step_end - step_start)); printf 'discobot-sandbox-init: [%d.%03ds] filesystem setup completed (overlayfs)\n' "$((elapsed / 1000))" "$((elapsed % 1000))"

	step_start=$(date +%s%3N); mount_cache_directories || log 'Cache mount failed'; step_end=$(date +%s%3N); elapsed=$((step_end - step_start)); printf 'discobot-sandbox-init: [%d.%03ds] cache directories mounted\n' "$((elapsed / 1000))" "$((elapsed % 1000))"
	step_start=$(date +%s%3N); setup_proxy_config || log 'Proxy config setup failed'; step_end=$(date +%s%3N); elapsed=$((step_end - step_start)); printf 'discobot-sandbox-init: [%d.%03ds] proxy config setup completed\n' "$((elapsed / 1000))" "$((elapsed % 1000))"
	step_start=$(date +%s%3N); setup_proxy_certificate "$run_as_user" || log 'Proxy certificate setup failed'; step_end=$(date +%s%3N); elapsed=$((step_end - step_start)); printf 'discobot-sandbox-init: [%d.%03ds] proxy certificate setup completed\n' "$((elapsed / 1000))" "$((elapsed % 1000))"
	step_start=$(date +%s%3N); write_docker_daemon_config || log 'Docker daemon config failed'; step_end=$(date +%s%3N); elapsed=$((step_end - step_start)); printf 'discobot-sandbox-init: [%d.%03ds] Docker daemon config written\n' "$((elapsed / 1000))" "$((elapsed % 1000))"

	rm -f "$(user_home "$run_as_user")/.docker/buildx/current" "$(user_home "$run_as_user")/.docker/buildx/instances/discobot-shared"

	step_start=$(date +%s%3N)
	write_proxy_environment_file || warn 'failed to write proxy env file'
	write_agent_environment_file "$run_as_user"
	step_end=$(date +%s%3N); elapsed=$((step_end - step_start)); printf 'discobot-sandbox-init: [%d.%03ds] environment files written\n' "$((elapsed / 1000))" "$((elapsed % 1000))"

	step_end=$(date +%s%3N); elapsed=$((step_end - startup_start)); printf 'discobot-sandbox-init: [%d.%03ds] setup completed successfully\n' "$((elapsed / 1000))" "$((elapsed % 1000))"
	notify_ready
}

main() {
	if [[ $# -lt 1 || $1 != setup ]]; then
		printf 'usage: discobot-sandbox-init <setup>\n' >&2
		return 1
	fi
	run_setup
}

main "$@"

#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/temproot.sh [--keep] [--tmpdir DIR] [--no-install] -- COMMAND [ARG...]
       scripts/temproot.sh [--keep] [--tmpdir DIR] [--no-install] COMMAND [ARG...]

Run a command in a temporary git worktree that includes the current working
tree's tracked modifications plus non-ignored untracked files.

If the repo has a pnpm-lock.yaml file, dependencies are installed in the
temporary worktree before the command runs.

The command runs from the same relative directory inside the temporary copy.
The temporary worktree is removed when the command exits unless --keep is used.

Examples:
  scripts/temproot.sh pnpm ci
  scripts/temproot.sh -- pnpm check
  scripts/temproot.sh --no-install go test ./...
  scripts/temproot.sh --keep --tmpdir /var/tmp pnpm test:unit
EOF
}

keep=false
install=true
tmp_parent="${TMPDIR:-/tmp}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h | --help)
      usage
      exit 0
      ;;
    --keep)
      keep=true
      shift
      ;;
    --no-install)
      install=false
      shift
      ;;
    --tmpdir)
      if [[ $# -lt 2 ]]; then
        echo "error: --tmpdir requires a directory" >&2
        exit 1
      fi
      tmp_parent="$2"
      shift 2
      ;;
    --)
      shift
      break
      ;;
    -*)
      echo "error: unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
    *)
      break
      ;;
  esac
done

if [[ $# -eq 0 ]]; then
  echo "error: command is required" >&2
  usage >&2
  exit 1
fi

if ! command -v git >/dev/null 2>&1; then
  echo "error: git is required" >&2
  exit 1
fi

repo_root="$(git rev-parse --show-toplevel)"
repo_name="$(basename "$repo_root")"
current_dir="$(pwd -P)"
relative_dir="${current_dir#"$repo_root"}"
relative_dir="${relative_dir#/}"

if [[ "$current_dir" != "$repo_root" && "$current_dir" != "$repo_root"/* ]]; then
  echo "error: current directory is not inside $repo_root" >&2
  exit 1
fi

mkdir -p "$tmp_parent"
tmp_root="$(mktemp -d "$tmp_parent/$repo_name.temproot.XXXXXXXX")"
worktree="$tmp_root/$repo_name"

cleanup() {
  if [[ "$keep" == true ]]; then
    echo "temproot kept at: $worktree" >&2
    return
  fi

  git -C "$repo_root" worktree remove --force "$worktree" >/dev/null 2>&1 || true
  rm -rf "$tmp_root"
}
trap cleanup EXIT

git -C "$repo_root" worktree add --detach --quiet "$worktree" HEAD

if ! git -C "$repo_root" diff --quiet HEAD -- .; then
  if ! git -C "$repo_root" diff --binary HEAD -- . | git -C "$worktree" apply --index --allow-binary-replacement --whitespace=nowarn -; then
    echo "error: failed to apply tracked changes to temporary worktree" >&2
    exit 1
  fi
fi

while IFS= read -r -d '' path; do
  mkdir -p "$worktree/$(dirname "$path")"
  cp -Pp "$repo_root/$path" "$worktree/$path"
done < <(git -C "$repo_root" ls-files --others --exclude-standard -z)

echo "temproot: $worktree" >&2

if [[ "$install" == true && -f "$worktree/pnpm-lock.yaml" ]]; then
  if ! command -v pnpm >/dev/null 2>&1; then
    echo "error: pnpm is required to install dependencies" >&2
    exit 1
  fi

  echo "temproot: installing dependencies" >&2
  pnpm --dir "$worktree" install --frozen-lockfile
fi

if [[ -n "$relative_dir" ]]; then
  cd "$worktree/$relative_dir"
else
  cd "$worktree"
fi

"$@"

#!/usr/bin/env python3
"""Print the current public organization bootstrap token for local development.

Meta stores only a SHA-256 hash of bootstrap tokens. The raw token is printed to
logs when the service starts, so this script extracts logged tokens and, when the
local SQLite database is available, validates them against the active public org
bootstrap token hash.
"""

from __future__ import annotations

import argparse
import hashlib
import os
import re
import sqlite3
import sys
from dataclasses import dataclass
from pathlib import Path

TOKEN_RE = re.compile(r"mboot_[A-Za-z0-9]{52}")
PUBLIC_ORG_DOMAIN = "public"


@dataclass
class TokenMatch:
    token: str
    path: Path
    mtime: float
    order: int


def main() -> int:
    parser = argparse.ArgumentParser(description="Print the current public org bootstrap token")
    parser.add_argument("logs", nargs="*", help="log files to scan; defaults to ~/.discobot/services/meta/output.log")
    parser.add_argument("--database", "-d", default=default_database_path(), help="SQLite database path or sqlite:// DSN")
    parser.add_argument("--no-validate", action="store_true", help="print the newest logged token without checking the database")
    args = parser.parse_args()

    matches = find_tokens(candidate_logs(args.logs))
    if not matches:
        print(
            "error: no bootstrap token found in logs; restart the Meta service and try again",
            file=sys.stderr,
        )
        return 1

    newest = sorted(matches, key=lambda match: (match.mtime, match.order))[-1]
    db_path = clean_sqlite_path(args.database)
    if args.no_validate:
        print(newest.token)
        return 0

    if db_path and db_path.exists():
        try:
            token = active_token_from_matches(db_path, matches)
        except sqlite3.Error as err:
            print(f"warning: failed to validate token against {db_path}: {err}", file=sys.stderr)
        else:
            if token:
                print(token)
                return 0
            print(
                f"error: found logged tokens, but none match the active public org token in {db_path}",
                file=sys.stderr,
            )
            return 1

    print(
        "warning: database not found; printing newest logged token without validation",
        file=sys.stderr,
    )
    print(newest.token)
    return 0


def candidate_logs(args: list[str]) -> list[Path]:
    if args:
        return [Path(arg).expanduser() for arg in args]

    workspace = Path(__file__).resolve().parents[1]
    logs: list[Path] = []
    env_log = os.environ.get("META_BOOTSTRAP_LOG")
    if env_log:
        logs.extend(Path(part).expanduser() for part in env_log.split(os.pathsep) if part)
    logs.extend(
        [
            Path.home() / ".discobot" / "services" / "meta" / "output.log",
            workspace / ".discobot" / "logs" / "meta.log",
            Path.home() / ".local" / "state" / "discobot" / "logs" / "server.log",
        ]
    )
    return dedupe(logs)


def find_tokens(paths: list[Path]) -> list[TokenMatch]:
    matches: list[TokenMatch] = []
    order = 0
    for path in paths:
        if not path.exists() or not path.is_file():
            continue
        try:
            text = path.read_text(errors="ignore")
        except OSError as err:
            print(f"warning: cannot read {path}: {err}", file=sys.stderr)
            continue
        try:
            mtime = path.stat().st_mtime
        except OSError:
            mtime = 0
        for match in TOKEN_RE.finditer(text):
            order += 1
            matches.append(TokenMatch(token=match.group(0), path=path, mtime=mtime, order=order))
    return matches


def active_token_from_matches(db_path: Path, matches: list[TokenMatch]) -> str | None:
    hashes = active_public_bootstrap_hashes(db_path)
    if not hashes:
        return None
    for match in sorted(matches, key=lambda item: (item.mtime, item.order), reverse=True):
        token_hash = hashlib.sha256(match.token.encode()).hexdigest()
        if token_hash in hashes:
            return match.token
    return None


def active_public_bootstrap_hashes(db_path: Path) -> set[str]:
    query = """
        SELECT obt.token_hash
        FROM organization_bootstrap_tokens obt
        JOIN organizations o ON o.id = obt.organization_id
        WHERE o.domain = ?
          AND o.deleted_at IS NULL
          AND obt.deleted_at IS NULL
          AND obt.status = 'active'
          AND (obt.expires_at IS NULL OR obt.expires_at > CURRENT_TIMESTAMP)
    """
    with sqlite3.connect(str(db_path)) as db:
        return {row[0] for row in db.execute(query, (PUBLIC_ORG_DOMAIN,))}


def default_database_path() -> str:
    return os.environ.get(
        "META_DATABASE_DSN",
        os.environ.get("DATABASE_DSN", str(Path.home() / ".local" / "share" / "discobot" / "meta.db")),
    )


def clean_sqlite_path(value: str) -> Path | None:
    if value.startswith("sqlite3://"):
        return Path(value.removeprefix("sqlite3://")).expanduser()
    if value.startswith("sqlite://"):
        return Path(value.removeprefix("sqlite://")).expanduser()
    if value.startswith("postgres://") or value.startswith("postgresql://"):
        return None
    return Path(value).expanduser()


def dedupe(paths: list[Path]) -> list[Path]:
    seen: set[str] = set()
    result: list[Path] = []
    for path in paths:
        key = str(path)
        if key in seen:
            continue
        seen.add(key)
        result.append(path)
    return result


if __name__ == "__main__":
    raise SystemExit(main())

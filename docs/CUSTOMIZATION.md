# Workspace Customization

Discboeing supports per-workspace customization through the `.discboeing/` directory at the root of your workspace. Four workspace-level customizations are available:

- **Environment file** (`.discboeing/env`) — Environment variables loaded into tools, console sessions, hooks, and services
- **Scripts** (`.discboeing/scripts/`) — User-facing executable slash commands exposed to the agent through the Skill tool
- **Hooks** (`.discboeing/hooks/`) — Automation scripts or AI review prompts that run at specific lifecycle points
- **Services** (`.discboeing/services/`) — Background processes and HTTP endpoints

Scripts, script hooks, and executable services use the same file format: executable scripts with YAML front matter. AI hooks and passive services use front matter with a non-executable body or declaration.

```
.discboeing/
├── env
├── scripts/
│   ├── summarize-diff.sh
│   └── deploy-preview
├── hooks/
│   ├── install-deps.sh
│   ├── lint.sh
│   └── typecheck.sh
└── services/
    ├── ui.sh
    ├── api-server.sh
    └── database
```

## Front Matter Format

All `.discboeing/` files use YAML front matter to declare configuration. Three delimiter styles are supported:

**Hash-prefixed (shell scripts):**

```bash
#!/bin/bash
#---
# name: My Script
# type: session
#---
echo "hello"
```

**Slash-prefixed (JavaScript/TypeScript):**

```javascript
#!/usr/bin/env node
//---
// name: Node Script
// http: 3000
//---
require("./server").start();
```

**Plain delimiters:**

```yaml
---
name: My Config
http: 8080
---
```

For comment-prefixed styles (`#---`, `//---`), all whitespace after the prefix is trimmed, so `#   name: Foo` and `# name: Foo` are equivalent.

---

## Environment File

If `.discboeing/env` exists at the workspace root, Discboeing loads it into the session environment used by tools, console sessions, hooks, and services.
Changes are picked up for new tool executions, hook runs, service launches, and console sessions without restarting the session.

Each non-empty line must be a `KEY=VALUE` assignment. Leading whitespace is ignored, lines starting with `#` are treated as comments, and `export KEY=VALUE` is also supported. Quoted values are allowed and are treated literally.

Invalid lines are ignored with a warning that includes the file path and line number, but Discboeing does not echo the rejected line contents.

---

## Scripts

Scripts are executable files directly in `.discboeing/scripts/` that behave like
user-facing slash commands. A file at `.discboeing/scripts/summarize-diff.sh`
becomes `/summarize-diff`. Discboeing does not recurse into subdirectories.
Scripts are discovered from the workspace and from supported user-level script
directories, but when Discboeing tells the LLM about them, they are presented
through the same `Skill` tool as markdown skills.

Discboeing resolves executable slash commands from these locations, in order:

1. workspace `.discboeing/scripts/`
2. user `~/.discboeing/scripts/`
3. user `~/.agents/scripts/`
4. system `/opt/discboeing/scripts/`
5. system `/usr/local/share/discboeing/scripts/`
6. system `/usr/share/discboeing/scripts/`

That means:

- users can invoke them with `/name ...args`
- the agent can invoke them through the `Skill` tool
- hidden scripts are omitted from the agent-visible reminder text
- the LLM does not need to know whether an entry came from markdown or an
  executable script

### Common Fields

| Field           | Type    | Required | Description                                                |
| --------------- | ------- | -------- | ---------------------------------------------------------- |
| `name`          | string  | No       | Slash-command name (defaults to the normalized filename)   |
| `description`   | string  | No       | Human-readable description shown in reminders and UI        |
| `visible`       | boolean | No       | Whether the script is exposed to the agent (default: true) |
| `argument-hint` | string  | No       | Optional guidance for expected arguments                   |

`argumentHint` is also accepted as an alternative key spelling.

### File Requirements

Script files must:

- be directly in `.discboeing/scripts/`
- be executable (`chmod +x`)
- have a shebang line (`#!/bin/bash`, `#!/usr/bin/env python`, etc.)
- include valid front matter

On Windows, Discboeing uses the script's shebang to choose an interpreter, so
Bash-based scripts require `bash` to be available on `PATH`.

### Execution Semantics

When a script is run:

- Discboeing executes the file directly and passes everything after `/name` as one
  raw string in `$1`
- the working directory is `/home/discboeing/workspace`
- the session environment, including `.discboeing/env`, is available
- on success, only trimmed `stdout` is forwarded to the LLM
- on failure, Discboeing forwards formatted execution metadata plus trimmed
  `stdout` and `stderr`
- if successful output is empty after trimming, Discboeing records the execution
  in message metadata and UI, but avoids starting a no-op model response

Hidden scripts (`visible: false`) can still exist in the repository for
internal workflows, but they are not surfaced in reminders and are not
available to the LLM through the `Skill` tool.

### Example — Visible script

Path: `.discboeing/scripts/summarize-diff.sh`

```bash
#!/bin/bash
#---
# name: summarize-diff
# description: Summarize the current git diff for the user
# visible: true
# argument-hint: Optional pathspec
#---
git diff --stat -- "$1"
```

### Example — Hidden script

Path: `.discboeing/scripts/internal-refresh`

```bash
#!/bin/bash
#---
# name: internal-refresh
# description: Refresh generated assets without advertising the command
# visible: false
#---
pnpm run generate
```

---

## Hooks

Hooks are files in `.discboeing/hooks/` that run at specific points during a session. Script hooks execute commands, while AI hooks ask the session's AI agent to review the relevant changes. They enable automated setup, validation, and enforcement of code quality standards.

Discboeing supports two hook engines:

| Engine            | How it runs                                                                 | File requirements                         |
| ----------------- | --------------------------------------------------------------------------- | ----------------------------------------- |
| `script` default  | Executes the hook file as a subprocess                                      | Executable bit, shebang, front matter     |
| `ai`              | Sends the hook body and change context to the session's AI agent            | Front matter only; no shebang or `chmod`  |

Script hooks are shell, Python, Node, or other executable files. AI hooks are
prompt files: the text after front matter is used as the hook's instructions.

### Hook Types

There are three hook types, set via the `type` field in front matter:

| Type         | When it runs                                     | On failure                          |
| ------------ | ------------------------------------------------ | ----------------------------------- |
| `session`    | Once at container startup                        | Logged, does not block startup      |
| `file`       | After each LLM turn, when matching files changed | LLM is re-prompted to fix the issue |
| `pre-commit` | On `git commit` (installed as a git hook)        | Commit is blocked                   |

### Common Fields

| Field         | Type   | Required | Description                                  |
| ------------- | ------ | -------- | -------------------------------------------- |
| `name`        | string | No       | Display name (defaults to filename)          |
| `type`        | string | **Yes**  | `session`, `file`, or `pre-commit`           |
| `engine`      | string | No       | `script` default, or `ai` for AI hooks       |
| `description` | string | No       | Human-readable description                   |
| `phase`       | string | No       | Gate file-hook execution to `review` phase; only `review` is currently valid |
| `subagent`    | string | No       | Sub-agent name for `engine: ai` hooks        |

### File Requirements

Hook files must:

- Be in the `.discboeing/hooks/` directory
- Have front matter with a valid `type` field
- For `engine: script` hooks, be executable (`chmod +x`) and have a shebang line (`#!/bin/bash`, `#!/usr/bin/env python`, etc.)
- For `engine: ai` hooks, have `engine: ai`; no executable bit or shebang is required

On Windows, Discboeing uses a script hook's shebang to choose an interpreter. Bash-based hooks therefore require `bash` to be available on `PATH`.

Files are sorted alphabetically by filename, so prefix with numbers to control execution order (e.g., `01-install.sh`, `02-lint.sh`).

### Session Hooks

Session hooks run once when the container starts, before the AI agent begins. They're ideal for installing dependencies, setting up the environment, or configuring tools.

**Additional fields:**

| Field    | Type             | Default | Description                             |
| -------- | ---------------- | ------- | --------------------------------------- |
| `run_as` | `root` or `user` | `user`  | Execute as root or as the discboeing user |

**Behavior:**

- Run sequentially in alphabetical order
- 5-minute timeout per hook
- Working directory is `/home/discboeing/workspace`
- Failures are logged but do not block the session from starting

**Environment variables:**

| Variable              | Description                                 |
| --------------------- | ------------------------------------------- |
| `DISCBOEING_SESSION_ID` | Current session ID                          |
| `DISCBOEING_WORKSPACE`  | Workspace path (`/home/discboeing/workspace`) |
| `DISCBOEING_HOOK_TYPE`  | `session`                                   |

Workspace credentials are scoped per runtime context. A credential must be
marked visible to **hooks** before Discboeing injects it into hook processes.

**Example — Install system packages (as root):**

```bash
#!/bin/bash
#---
# name: Install system deps
# type: session
# run_as: root
#---
apt-get update && apt-get install -y postgresql-client redis-tools
```

**Example — Install project dependencies (as user):**

```bash
#!/bin/bash
#---
# name: Install dependencies
# type: session
#---
pnpm install --frozen-lockfile 2>&1 || pnpm install 2>&1
```

### File Hooks

File hooks run after each LLM turn completes, checking whether files matching a glob pattern have changed. If a file hook fails, the LLM is automatically re-prompted with the failure output so it can fix the issue.

**Additional fields:**

| Field        | Type    | Default      | Description                                                       |
| ------------ | ------- | ------------ | ----------------------------------------------------------------- |
| `pattern`    | string  | **Required** | Glob pattern for file matching (e.g., `"*.go"`, `"src/**/*.ts"`)  |
| `ignore`     | string  | none         | Glob pattern to ignore for this hook (alias: `exclude`)           |
| `notify_llm` | boolean | `true`       | Whether to re-prompt the LLM on failure                           |

**Behavior:**

- Run after the LLM completion finishes and the SSE stream closes
- Only triggered when files matching the `pattern` have changed since the last evaluation
- Files matching a hook's `ignore`/`exclude` field are skipped for that hook
- Files matching `.discboeing/hooks/ignore` are skipped for all file hooks
- On failure with `notify_llm: true`: the LLM receives the hook output and attempts to fix the issue (up to 3 retries per user message)
- On failure with `notify_llm: false`: the hook runs silently — useful for auto-fixers like formatters
- Hooks that fail block subsequent hooks from running until fixed
- Hooks with `phase: review` are kept pending until the session phase is `review`
- If agent-go restarts while a file hook is running, Discboeing resets that hook to pending and re-runs it on the next eligible evaluation

**Environment variables:**

| Variable                 | Description                                                        |
| ------------------------ | ------------------------------------------------------------------ |
| `DISCBOEING_CHANGED_FILES` | Space-separated list of changed file paths (relative to workspace) |
| `DISCBOEING_SESSION_ID`    | Current session ID                                                 |
| `DISCBOEING_HOOK_TYPE`     | `file`                                                             |

**Glob pattern syntax** uses [picomatch](https://github.com/micromatch/picomatch) patterns:

| Pattern                       | Matches                                   |
| ----------------------------- | ----------------------------------------- |
| `"*.go"`                      | All `.go` files in any directory          |
| `"src/**/*.ts"`               | All `.ts` files under `src/`              |
| `"*.{ts,tsx}"`                | All `.ts` and `.tsx` files                |
| `"**/*.go"`                   | All `.go` files recursively               |
| `"{package.json,pnpm*.yaml}"` | `package.json` and any `pnpm*.yaml` files |

`.discboeing/hooks/ignore` is a plain text file with one glob pattern per line.
Blank lines and lines starting with `#` are ignored. It uses the same glob
syntax as `pattern`; gitignore-specific features such as `!` negation are not
supported.

**Example — Lint Go files:**

```bash
#!/bin/bash
#---
# name: Go format check
# type: file
# pattern: "*.go"
# ignore: "*_templ.go"
#---
gofmt -l $DISCBOEING_CHANGED_FILES
```

**Example — Auto-fix TypeScript (silent):**

```bash
#!/bin/bash
#---
# name: ESLint autofix
# type: file
# pattern: "*.{ts,tsx}"
# notify_llm: false
#---
npx eslint --fix $DISCBOEING_CHANGED_FILES
```

**Example — Reinstall dependencies when lockfile changes:**

```bash
#!/bin/bash
#---
# name: Install dependencies
# type: file
# pattern: "{package.json,pnpm*.yaml}"
#---
pnpm install --frozen-lockfile 2>&1 || pnpm install 2>&1
```

**Example — Run `go mod tidy` when go.mod changes:**

```bash
#!/bin/bash
#---
# name: Go mod tidy
# type: file
# pattern: "**/go.mod"
#---
for f in $DISCBOEING_CHANGED_FILES; do
	dir=$(dirname "$f")
	(cd "$dir" && go mod tidy 2>&1)
done
```

**Example — AI review hook:**

```markdown
---
name: Go reviewer
type: file
engine: ai
pattern: "**/*.go"
phase: review
subagent: reviewer
---
Review the changed Go files for correctness, missing tests, and maintainability.
Approve only when the code is ready to merge.
```

AI hooks run as task-style reviews in a stable thread dedicated to that hook.
Each run sends a new prompt into the same thread with the hook instructions,
matching changed files, and an inline diff when available. Active AI hook
reviews are registered with the same task/completion infrastructure used by the
`Task` and `TaskOutput` tools, so the thread is visible to normal completion
tracking while the hook waits for the final review output. Discboeing also writes
the complete run context
to `~/.discboeing/threads/{hookThreadId}/ai-hooks/{hookId}/context-{timestamp}.md`
and includes that path in the prompt so the AI can read the full diff if the
inline context is truncated.

Discboeing adds a standard response instruction to each AI hook prompt: output
`SUCCESS` when the hook passes, or `FEEDBACK: <actionable feedback>` when
changes need attention. Hook authors should write review-specific instructions
and do not need to include that success/feedback wording in the hook body.
`subagent` is optional and selects a configured session sub-agent, including its
prompt, model override, and tool restrictions. AI `pre-commit` hooks are not
supported.

### Review Phase Hooks

File hooks may include `phase: review` in front matter to defer execution until
the thread enters the review phase. This is useful for checks that should run only
after the agent believes the implementation is complete, such as final code
quality reviews.

Behavior, matching the `agent-go` hook manager:

- The only accepted non-empty `phase` value is `review`; other values make hook
  discovery fail for that hook set.
- Any hook with `phase: review` makes Discboeing expose the `ReadyForReview` tool
  to the agent. Calling it records the session phase as `review`.
- File hooks with `phase: review` still detect matching changed files during
  normal post-turn evaluation, but remain pending instead of executing while
  the session phase is empty or not `review`.
- Pending review-phase file hooks become eligible once the session phase is
  `review`, and then run through the normal file-hook flow, including
  `notify_llm` retries and blocking subsequent hooks on failure.

**Example — Final AI review hook:**

```markdown
---
name: Final code review
type: file
engine: ai
pattern: "**/*"
phase: review
---
Review the completed changes for correctness, maintainability, missing tests,
and alignment with repository guidance. Approve only when the change is ready.
```

### Pre-commit Hooks

Pre-commit hooks are installed as git pre-commit hooks. They run automatically when `git commit` is executed and block the commit if they fail.

**Additional fields:**

| Field        | Type    | Default | Description             |
| ------------ | ------- | ------- | ----------------------- |
| `notify_llm` | boolean | `true`  | Reserved for future use |

**Behavior:**

- On session startup, Discboeing generates a `.git/hooks/pre-commit` script that chains all `type: pre-commit` hooks
- If a `.git/hooks/pre-commit` already exists and wasn't created by Discboeing, it's preserved as `.git/hooks/pre-commit.original` and still runs
- When the LLM runs `git commit` and the hook fails, it sees the error output and can fix the issue and retry

**Example — Run CI checks before commit:**

```bash
#!/bin/bash
#---
# name: CI
# type: pre-commit
#---
pnpm run ci
```

**Example — Type check before commit:**

```bash
#!/bin/bash
#---
# name: Type check
# type: pre-commit
#---
pnpm typecheck
```

### How File Hooks Interact with the LLM

When a file hook fails with `notify_llm: true`, the LLM receives a message like:

```
[Discboeing Hook Failed] "Go format check" (pattern: *.go)

Files: internal/server/handler.go, internal/server/service.go
Exit code: 1

Output:
internal/server/handler.go: formatting differs from gofmt
internal/server/service.go: formatting differs from gofmt

Please fix the issues and ensure the hook passes. (Attempt 1/3)
```

For large output (over 200 lines), the output is saved to a file and the LLM is told to read it:

```
[Discboeing Hook Failed] "Go format check" (pattern: *.go)

Files: internal/server/handler.go, internal/server/service.go
Exit code: 1

Output is large (847 lines). Full output saved to:
  ~/.discboeing/threads/abc123/hooks/output/go-format-check.log

Please read the file to see the full output and address the issues. (Attempt 1/3)
```

After 3 consecutive hook-triggered retries, Discboeing stops re-prompting. The pending hooks will re-evaluate on the next user-initiated message.

When multiple file hooks are pending, Discboeing re-runs them in alphanumeric order by hook ID. If the earliest pending hook keeps failing, later pending hooks will wait until that earlier hook passes or is no longer pending.

### Hook Reloading

Hooks are automatically reloaded when files in `.discboeing/hooks/` change. Adding, removing, or editing hook files takes effect on the next evaluation cycle without restarting the session.

---

## Services

Services are background processes defined in `.discboeing/services/`. They allow you to run development servers, databases, or any long-running process alongside your AI agent session, with built-in HTTP proxying and output streaming.

Services receive only credentials marked visible to the **services** runtime
context. Credentials marked only for tools are not injected into background
services, hooks, SSH, or terminal sessions.

### Service Types

There are two types of services:

| Type           | Description                                  | Requirements                                 |
| -------------- | -------------------------------------------- | -------------------------------------------- |
| **Executable** | Scripts started/stopped by Discboeing          | Executable file with shebang and script body |
| **Passive**    | Declarations for externally-managed services | Front matter only, no script body            |

### Configuration Fields

| Field         | Type   | Required | Description                                                                        |
| ------------- | ------ | -------- | ---------------------------------------------------------------------------------- |
| `name`        | string | No       | Display name (defaults to filename)                                                |
| `description` | string | No       | Human-readable description                                                         |
| `order`       | number | No       | Optional UI sort order for service buttons and panels (lower numbers appear first) |
| `http`        | number | No       | HTTP port the service listens on                                                   |
| `https`       | number | No       | HTTPS port the service listens on                                                  |
| `path`        | string | No       | Default URL path for web preview (e.g., `"/app"`, `"/api/docs"`)                   |

### Service ID

The service ID is derived from the filename:

- Common script extensions are stripped (`.sh`, `.bash`, `.zsh`, `.py`, `.js`, `.ts`, `.rb`, `.pl`, `.php`)
- Remaining dots become hyphens
- Converted to lowercase
- Non-alphanumeric characters (except `-` and `_`) are removed

For example: `ui.sh` becomes `ui`, `api-server.sh` becomes `api-server`.

### Service Ordering

You can control the order of services in the Discboeing UI with the optional `order` field.

- Lower numbers appear first
- Services with the same `order` fall back to alphabetical ordering by name
- Services without an `order` appear after all ordered services, also alphabetically

This applies to the services shown in the active UI, including the service panel buttons.

### Executable Services

Executable services are scripts that Discboeing starts and stops as child processes. They must be executable files with a shebang line and a script body.

On Windows, Discboeing uses the service shebang to choose an interpreter. Bash-based services therefore require `bash` to be available on `PATH`.

**Example — Vite development server:**

```bash
#!/bin/bash
#---
# name: Discboeing UI
# description: Vite + React Router UI development server
# order: 10
# http: 3000
#---
pnpm install && pnpm dev
```

**Example — SQLite browser:**

```bash
#!/bin/bash
#---
# name: SQLite GUI
# description: sqlite-web browser for the Discboeing database
# http: 8080
#---
DB="${HOME}/.local/share/discboeing/discboeing.db"

if [ ! -f "$DB" ]; then
    echo "Database not found at: $DB"
    exit 1
fi

exec uvx --from sqlite-web sqlite_web "$DB" --port 8080 --host 0.0.0.0 --no-browser --read-only
```

**Lifecycle:**

```
stopped → starting → running → stopping → stopped
                        ↓
                      error → stopped
```

- Started via the UI or API (`POST /services/:id/start`)
- Stopped with SIGTERM, then SIGKILL after 5 seconds if still running
- Output (stdout/stderr) is captured and available via SSE streaming
- Output files are stored at `~/.discboeing/services/{id}/output.log` (JSONL, max 1MB, auto-truncated)

### Passive Services

Passive services declare an HTTP endpoint without providing a script to run. They're useful when a service is managed externally (e.g., by `devcontainer.json`, `docker-compose`, or a socket-activated systemd unit inside the sandbox).

**Example — Externally-managed web app:**

```yaml
---
name: Web App
description: Frontend dev server (started by devcontainer)
order: 10
http: 3000
---
```

Passive service files:

- Must have front matter with `http` or `https` defined
- Must have an empty body (no script content after the closing delimiter)
- Do **not** need a shebang line or execute permissions

Passive services always show as `status: "stopped"` in the API. Start/stop/output calls return `400`. The HTTP proxy still works if the declared port is accessible.

### HTTP Proxy

Services with an `http` or `https` port get automatic HTTP proxying. The proxy is accessible through the Discboeing UI as a web preview, or via subdomain-based URLs:

```
{session-id}-svc-{service-id}.{base-domain}
```

For example: `01HXYZ123456789ABCDEF-svc-ui.localhost:3000`

The proxy:

- Supports all HTTP methods and WebSocket connections
- Does not start executable services; start them explicitly from the UI or API
- Returns an auto-refreshing page if the service isn't ready yet
- Does not forward authentication credentials (services are considered public within the sandbox)

The `path` field in front matter sets the default URL path used by the web preview in the UI.

---

## Complete Example

Here's a full `.discboeing/` configuration for a Go + React project:

```
.discboeing/
├── hooks/
│   ├── 01-install-deps.sh      # Session: install all dependencies
│   ├── 02-go-mod-tidy.sh       # File: tidy go.mod when it changes
│   ├── 03-lint-frontend.sh     # File: lint TypeScript on change
│   ├── 04-lint-backend.sh      # File: lint Go on change
│   ├── 05-build.sh             # File: verify build on change
│   └── 06-ci.sh                # Pre-commit: full CI before commit
└── services/
    ├── ui.sh                   # Dev server on port 3000
    └── db.sh                   # SQLite browser on port 8080
```

**`.discboeing/hooks/01-install-deps.sh`:**

```bash
#!/bin/bash
#---
# name: Install dependencies
# type: session
#---
pnpm install --frozen-lockfile 2>&1 || pnpm install 2>&1
cd server && go mod download 2>&1
```

**`.discboeing/hooks/02-go-mod-tidy.sh`:**

```bash
#!/bin/bash
#---
# name: Go mod tidy
# type: file
# pattern: "**/go.mod"
#---
for f in $DISCBOEING_CHANGED_FILES; do
	dir=$(dirname "$f")
	(cd "$dir" && go mod tidy 2>&1)
done
```

**`.discboeing/hooks/03-lint-frontend.sh`:**

```bash
#!/bin/bash
#---
# name: Frontend lint
# type: file
# pattern: "**/*.{ts,tsx,js,jsx,json}"
#---
pnpm check:frontend:fix
```

**`.discboeing/hooks/04-lint-backend.sh`:**

```bash
#!/bin/bash
#---
# name: Backend lint
# type: file
# pattern: "**/*.go"
#---
pnpm check:backend:fix
```

**`.discboeing/hooks/05-build.sh`:**

```bash
#!/bin/bash
#---
# name: Build
# type: file
# pattern: "**/*.{ts,tsx,js,jsx,go}"
#---
pnpm build
```

**`.discboeing/hooks/06-ci.sh`:**

```bash
#!/bin/bash
#---
# name: CI
# type: pre-commit
#---
pnpm run ci
```

**`.discboeing/services/ui.sh`:**

```bash
#!/bin/bash
#---
# name: Dev UI
# description: Vite development server
# order: 10
# http: 3000
#---
pnpm install && pnpm dev
```

**`.discboeing/services/db.sh`:**

```bash
#!/bin/bash
#---
# name: SQLite GUI
# description: Database browser
# order: 20
# http: 8080
#---
exec uvx --from sqlite-web sqlite_web "$HOME/.local/share/app/app.db" \
    --port 8080 --host 0.0.0.0 --no-browser --read-only
```

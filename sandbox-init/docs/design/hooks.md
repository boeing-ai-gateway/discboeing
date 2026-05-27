# Session Hooks (Agent Init)

Session hooks are scripts in `.discobot/hooks/` with `type: session` that run once during container startup. They are executed by the Go sandbox init process (PID 1) before the agent-api starts.

## Execution Point

Session hooks run after the workspace and filesystem are fully set up but before the agent-api process is forked. This means:

- The workspace is cloned and available at `/home/discobot/workspace`
- The overlay filesystem is mounted
- Cache directories are mounted
- The proxy may or may not be started yet (hooks run before proxy/Docker startup)

## Implementation

Discobot supports two hook execution engines in agent-go:

- `engine: script` (default): executable files with a shebang are run as
  subprocesses.
- `engine: ai`: prompt files are run by agent-go as task-style AI reviews in
  a stable thread dedicated to that hook. The hook body after front matter is
  used as the hook instruction prompt. The first run introduces the hook review;
  later runs tell the same thread that new changes are available. Each run adds
  an input describing the files changed since that hook last ran, plus their
  diff, and the assistant response is written to the hook output log. AI hook
  runs use the same in-memory task runner as the Task/TaskOutput tools, so
  active review turns are registered with completion tracking and can be
  reported or cancelled through the normal task/completion path. Before each
  prompt, agent-go writes the full hook run context to
  `~/.discobot/threads/{hookThreadId}/ai-hooks/{hookId}/context-{timestamp}.md`
  and includes that path in the prompt so the agent can read the complete diff
  if inline prompt context is truncated. AI hooks may set `subagent: <name>` to
  run with a configured sub-agent's prompt, model, and tool restrictions.

AI hook responses are evaluated by a separate AI judgment prompt that decides
whether the hook passed and whether the main conversation should be notified.
This avoids relying on literal prefix matching of the review response. Runtime
hook state also records per-hook timestamp markers under
`~/.discobot/threads/{sessionId}/hooks/timestamps/` so subsequent runs can focus
on changes made since that hook last ran. The marker value persisted after a
run is the cutoff captured immediately before starting the hook, so file edits
that happen while the hook is running remain visible to the next evaluation.

### Hook Discovery

The Go agent scans `/home/discobot/workspace/.discobot/hooks/` for files that:
1. Are regular files (not directories, not hidden)
2. For script hooks, have the executable bit set and a shebang line (`#!`)
3. For AI hooks, have front matter with `engine: ai` (no executable bit or
   shebang required)
4. Have front matter with a supported `type`

### Front Matter Parsing

Minimal Go implementation of the same YAML front matter parser used by the TypeScript services module. Supports `#---` delimiters with `key: value` pairs:

```go
type HookConfig struct {
    Name        string // Display name
    Type        string // "session", "file", "pre-commit"
    Engine      string // "script" (default) or "ai"
    Subagent    string // Optional sub-agent name for AI hooks
    Description string // Human-readable description
    RunAs       string // "root" or "user" (default: "user")
}
```

### Execution

Hooks are sorted alphabetically by filename and executed sequentially:

```go
func runSessionHooks(workspaceDir string, userInfo *userInfo) error
```

For each hook:
- If `run_as: root` → execute as root (no credential switching)
- If `run_as: user` (default) → execute as discobot user via `syscall.Credential`
- Working directory: `/home/discobot/workspace`
- Timeout: 5 minutes per hook
- Retries: session hooks are retried immediately up to 10 total attempts before the overall run is marked as failed
- stdout/stderr captured and logged
- **On failure after all retries: log error and continue** (don't block session startup)

AI hooks are executed by agent-go rather than sandbox-init, because they need
the session's model/provider configuration and thread store. They use the same
session/file trigger decisions as script hooks once agent-go is running. AI
`pre-commit` hooks are not supported. When `subagent` is set, the name must
match a sub-agent configured for the session, such as one discovered from
`.claude/agents/*.md`.

### Environment Variables

Hooks receive the agent's environment plus:

| Variable | Description |
|----------|-------------|
| `DISCOBOT_SESSION_ID` | Current session ID |
| `DISCOBOT_WORKSPACE` | Workspace path (`/home/discobot/workspace`) |
| `DISCOBOT_HOOK_TYPE` | Always `session` |

### Error Handling

Session hook failures are retried up to 10 times, then logged without preventing the agent-api from starting. This ensures that transient startup issues can recover automatically while a broken hook still doesn't make the session permanently unusable.

Runtime hook state is persisted under `~/.discobot/threads/{sessionId}/hooks/`, including `status.json` and per-hook output logs in the `output/` subdirectory.

## Example

```bash
#!/bin/bash
#---
# name: Install system deps
# type: session
# run_as: root
#---
apt-get update && apt-get install -y postgresql-client
```

```bash
#!/bin/bash
#---
# name: Setup dev environment
# type: session
#---
pnpm install
cp .env.example .env
```

```markdown
---
name: Review Go changes
type: file
engine: ai
pattern: "**/*.go"
subagent: reviewer
---
Review the changed Go files for correctness, missing tests, and maintainability.
Respond with SUCCESS if there is nothing to fix, otherwise respond with
FEEDBACK: followed by concise actionable feedback.
```

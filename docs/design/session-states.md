# Session States Design

This document describes the session lifecycle states and commit states, which are tracked independently.

## Overview

Sessions have two independent state dimensions:

1. **Session Status** (`status`): Tracks the lifecycle of the session (initialization, running, stopped, etc.)
2. **Commit Status** (`commitStatus`): Tracks commit operations (orthogonal to session status)

This separation allows a session to be `ready` and `committing` at the same time, which correctly models that the sandbox continues running while a commit is in progress.

## Session Status (Lifecycle)

### State Diagram

```
                                    ┌──────────────┐
                                    │ initializing │
                                    └──────┬───────┘
                                           │
                    ┌──────────────────────┼──────────────────────┐
                    │                      │                      │
                    ▼                      ▼                      ▼
            ┌───────────┐          ┌──────────────┐       ┌───────────────────┐
            │  cloning  │          │ pulling_image│       │ creating_sandbox  │
            └─────┬─────┘          └──────┬───────┘       └─────────┬─────────┘
                  │                       │                         │
                  └───────────────────────┼─────────────────────────┘
                                          │
                                          ▼
                                    ┌───────────┐
                           ┌────────│   ready   │────────┐
                           │        └─────┬─────┘        │
                           │              │              │
                           ▼              │              ▼
                     ┌──────────┐         │        ┌──────────┐
                     │ stopped  │◄────────┘        │  error   │
                     └────┬─────┘                  └──────────┘
                          │
                          ▼
                   ┌────────────┐
                   │  removing  │
                   └──────┬─────┘
                          │
                          ▼
                    ┌──────────┐
                    │ removed  │
                    └──────────┘
```

### Status Values

| Status | Description |
|--------|-------------|
| `initializing` | Session just created, starting setup process |
| `reinitializing` | Recreating sandbox after it was deleted |
| `cloning` | Cloning git repository for the workspace |
| `pulling_image` | Pulling the runtime image |
| `creating_sandbox` | Creating the sandbox container environment |
| `ready` | Session is ready for use. Sandbox is running. |
| `stopped` | Sandbox is stopped. Will restart on demand. |
| `error` | Something failed during setup. Check `errorMessage`. |
| `removing` | Session is being deleted asynchronously |
| `removed` | Session has been deleted. |

### Prompt Submission Durability

Prompt delivery has its own durable handoff separate from session lifecycle and commit status. Before the server tries to create or reconcile a sandbox and forward a prompt, it stores a `PromptSubmission` record in the database. If the server restarts or sandbox creation fails mid-request, startup reconciliation re-enqueues any `pending` or stale `dispatching` submissions and retries delivery.

The persisted prompt payload is encrypted at rest while the submission is pending. Once the sandbox accepts the prompt, the submission moves to `accepted`, stores the returned `completionId` or `queuedPromptId`, and clears the encrypted payload so prompt contents are not retained longer than necessary.

---

## Commit Status (Orthogonal)

### State Diagram

```
    ┌─────────┐     commit()     ┌──────────┐  /discobot-commit   ┌────────────┐
    │  none   │ ───────────────► │ pending  │ ──────────────────► │ committing │
    └─────────┘                  └──────────┘                     └──────┬─────┘
         ▲                                                               │
         │                                                     ┌─────────┴─────────┐
         │                                                     │                   │
         │                                             success │           failure │
         │                                                     ▼                   ▼
         │                                             ┌────────────┐       ┌──────────┐
         └─────────────────────────────────────────────│ completed  │       │  failed  │
              (can commit again after completed/failed)└────────────┘       └──────────┘
```

### Status Values

| Status | Description |
|--------|-------------|
| `""` (empty) | No commit in progress (default state) |
| `pending` | Commit requested, job enqueued, waiting to send to agent |
| `committing` | Operation command (`/discobot-commit`) sent to agent, waiting for patches |
| `completed` | Commit completed successfully |
| `failed` | Commit failed. Check `commitError` for details. |

The commit and rebase toolbar actions are discovered from agent command metadata.
Discobot command frontmatter marks UI-visible commands with `discobot-ui: true`,
so the toolbar follows whatever command variant the sandbox installs for the
current workspace.

### Session Commit Fields

Internal session state stores only the operation state plus a stable merge target:

| Field | Type | Description |
|-------|------|-------------|
| `commitStatus` | string | Current commit state |
| `commitOperation` | string | Active operation (`commit`) |
| `commitError` | string | Error message if `commitStatus = "failed"` |
| `targetRef` | string | Merge target ref resolved at operation time. Defaults to `HEAD`. |
| `appliedCommit` | string | Final commit SHA after patches apply to the workspace (commit flow only) |

No rolling base or ancestry watermark fields are stored on the session. The concrete
target SHA is resolved fresh from the workspace whenever preview, commit, or rebase runs.

Successful commit pulls are also recorded in `session_commit_logs`, which stores the
operation type, `targetRef`, the resolved `targetCommit`, the sandbox `headCommit`,
requested commit/directory metadata, the applied workspace commit, and the raw patch
bundle for audit/debugging.

### REST API Projection

The REST API does not expose `commitStatus` or `commitError` directly on session responses.
Instead it flattens commit state into the existing session fields:

| Internal state | REST `status` | REST `errorMessage` |
|---|---|---|
| `commitStatus = "pending"` | `pending` | omitted |
| `commitStatus = "committing"` | `committing` | omitted |
| `commitStatus = "completed"` + `commitOperation = "commit"` | `committed` | omitted |
| `commitStatus = "failed"` | `error` | `commitError` |
| no commit in progress | session lifecycle `status` | session `errorMessage` when applicable |

---

## Commit Flow

### 1. User Clicks Commit Button

The commit button now sends `/discobot-commit` to the active thread. There is no public session commit API anymore.
For local workspaces, that slash command runs inside the sandbox, prepares local commit(s), and then uses the `RequestCommitPull` approval flow.
For git-URL workspaces cloned in the sandbox, `/discobot-commit` instead prepares commit(s), pushes a branch, and opens an upstream pull request directly.

The server-side session commit job is still used after a `RequestCommitPull` approval is accepted:

- The agent-side `RequestCommitPull` tool emits a specialized approval request
- The UI presents approve/reject controls to the user
- `POST /api/projects/{projectId}/sessions/{sessionId}/threads/{threadId}/answer/{questionId}` submits the decision
- When the server receives an approved `RequestCommitPull` answer, it enqueues the session commit job
- `PerformCommit` still first checks for an existing replay bundle and applies it without re-sending `/discobot-commit` when commits are already present

### 2. Job Execution (PerformCommit)

```go
func PerformCommit(ctx, projectID, sessionID) error {
    session := getSession(sessionID)

    // Idempotency: Skip if already completed
    if session.CommitStatus == "completed" {
        return nil
    }

    targetRef := session.TargetRef
    if targetRef == "" {
        targetRef = "HEAD"
    }

    session.CommitStatus = "pending"
    session.CommitOperation = "commit"
    session.AppliedCommit = ""
    updateSession(session)

    // Step 1: Opportunistically apply any patches that are already available.
    if session.CommitStatus == "pending" && session.AppliedCommit == "" {
        targetCommit := resolveWorkspaceTargetCommit(session.WorkspaceID, targetRef)
        patches, err := agentAPI.GetCommits(sessionID, targetCommit)
        if err == nil && len(patches.Data) > 0 {
            finalCommit := applyPatches(session.WorkspaceID, patches.Data)
            recordSessionCommitLog(sessionID, targetRef, targetCommit, patches.HeadCommit, finalCommit, patches.Data)
            session.AppliedCommit = finalCommit
        }
    }

    // Step 2: Ask the sandbox to prepare commits if needed.
    if session.CommitStatus == "pending" {
        err := sendChatMessage(sessionID, "/discobot-commit")
        if err != nil {
            setCommitFailed(session, "Failed to send commit command: " + err.Error())
            return nil
        }

        session.CommitStatus = "committing"
        updateSession(session)
        fireSessionUpdatedEvent(projectID, sessionID)
    }

    // Step 3: Resolve the target again and fetch/apply patches.
    if session.AppliedCommit == "" {
        targetCommit := resolveWorkspaceTargetCommit(session.WorkspaceID, targetRef)
        patches, err := agentAPI.GetCommits(sessionID, targetCommit)
        if err == ErrNoCommits && sandboxIsCleanRelativeToTarget() {
            markCompletedNoOp(session)
            return nil
        }
        if err != nil {
            setCommitFailed(session, "Failed to get commits: " + err.Error())
            return nil
        }

        finalCommit, err := applyPatches(session.WorkspaceID, patches.Data)
        if err != nil {
            setCommitFailed(session, "Failed to apply patches: " + err.Error())
            return nil
        }

        session.AppliedCommit = finalCommit
        updateSession(session)
        recordSessionCommitLog(sessionID, targetRef, targetCommit, patches.HeadCommit, finalCommit, patches.Data)
        fireSessionUpdatedEvent(projectID, sessionID)
    }

    // Step 4: Verify and complete
    if commitExistsInWorkspace(session.WorkspaceID, session.AppliedCommit) {
        session.CommitStatus = "completed"
        session.CommitError = ""
        updateSession(session)
        fireSessionUpdatedEvent(projectID, sessionID)
    } else {
        setCommitFailed(session, "Applied commit not found in workspace")
    }

    return nil
}

func setCommitFailed(session, errorMsg) {
    session.CommitStatus = "failed"
    session.CommitError = errorMsg
    updateSession(session)
    fireSessionUpdatedEvent(session.ProjectID, session.ID)
}
```

### Rebase Flow

Rebase is now a sandbox-local git action triggered through `/discobot-rebase`.
It no longer uses a server endpoint, background job, or session commit state.
The sandbox repository is expected to track its origin branch so the command can
fetch the tracked remote and rebase onto the configured upstream directly,
without asking the user for a separate target commit.

### 3. Agent-API Endpoint

```
GET /commits?target={resolvedTargetCommit}
```

**Response (success)**:
```json
{
    "patches": "<git format-patch output>",
    "commitCount": 2,
    "headCommit": "<sandbox head sha>"
}
```

**Response (error)**:
```json
{
    "error": "invalid_target" | "no_commits" | "not_git_repo"
}
```

- When the target is an ancestor of `HEAD`, uses `git format-patch target..HEAD`
- When the sandbox and target do not share a direct ancestry range, synthesizes a
  `format-patch` bundle against the target tree so commit pulls are still possible
- Preserves commit metadata and returns patches in order, ready for `git am`

### 4. Apply Patches to Workspace

```bash
# In workspace directory
git am --keep-cr < patches.patch
```

- Applies commits exactly as-is with original metadata
- Preserves commit signatures if present
- Returns the final commit SHA

---

## Idempotency

The job is designed to handle server restarts safely:

| Job restarts when... | State | Action |
|---------------------|-------|--------|
| Before sending to agent | `pending`, `appliedCommit=""` | Resolve current `targetRef`, try existing patches, send `/discobot-commit` if needed |
| After sending, before apply | `committing`, `appliedCommit=""` | Resolve current `targetRef`, fetch patches, apply |
| After apply, before complete | `committing`, `appliedCommit` set | Verify commit exists, mark `completed` |
| Already done | `completed` | No-op |
| Target moved before fetch | `pending`/`committing`, `appliedCommit=""` | Re-resolve the target commit and request a fresh bundle |

**Key idempotency checks**:
1. Always resolve `targetRef` to a concrete workspace commit immediately before preview, apply, or validation
2. `appliedCommit` being set indicates patches were applied
3. Agent patch generation is idempotent: repeated fetches return changes relative to the current target

---

## Error Handling

| Error | Result | User Action |
|-------|--------|-------------|
| Sandbox not running | Auto-reconcile (start sandbox), retry operation | None - handled automatically |
| Agent-api returns `no_commits` with a dirty working tree | `failed` + error message | Finish or discard sandbox changes, then retry |
| Agent-api returns `invalid_target` or `not_git_repo` | `failed` + error message | Reconcile the session, then retry |
| Patch application fails | `failed` + error message | Click Commit to retry |
| Verification fails | `failed` + error message | Click Commit to retry |

### Sandbox Reconciliation

If the sandbox is not running when a commit operation is attempted, the system automatically:
1. Detects sandbox unavailability errors (`ErrNotRunning`, `ErrNotFound`, or "sandbox not running" messages)
2. Updates session status to `reinitializing`
3. Starts the sandbox via `Initialize()`
4. Retries the original operation

This reconciliation happens transparently at three points in the commit flow:
- **Optimistic patch check** (`tryApplyExistingPatches`)
- **Sending commit prompt** (`sendCommitPrompt`)
- **Fetching patches** (`fetchAndApplyPatches`)

Only if the sandbox fails to start (enters `error` state) will the commit job fail. This ensures commits succeed even if the sandbox was stopped or deleted between sessions.

User can always click Commit again to retry - it starts fresh by resolving the
current `targetRef`.

---

## Chat Behavior

| Session Status | Commit Status | Chat Allowed |
|---------------|---------------|--------------|
| Any | `pending` | **No** - Input disabled |
| Any | `committing` | **No** - Input disabled |
| `ready` | `""` / `completed` / `failed` | Yes |
| `stopped` | `""` / `completed` / `failed` | Yes (restarts sandbox) |
| `error` | Any | No |

---

## SSE Events

All `commitStatus` changes fire `session_updated` SSE event:

```json
{
    "type": "session_updated",
    "data": {
        "sessionId": "abc123",
        "status": ""
    }
}
```

Client re-fetches session to get updated public `status`, `errorMessage`, and `appliedCommit`.

---

## Implementation Components

### Backend

| Component | File | Changes |
|-----------|------|---------|
| Model | `server/internal/model/model.go` | Add `CommitError`, `TargetRef`, `AppliedCommit` fields and target-aware commit logs |
| Service | `server/internal/service/session.go` | Update `CommitSession()`, `PerformCommit()`, and runtime target resolution |
| Job | `server/internal/jobs/session_commit.go` | Already exists, update executor |
| Git | `server/internal/service/git.go` | Add `ApplyPatches()` method and target commit helpers |
| Handler | `server/internal/handler/chat.go` | Approval answer handling, preview, and chat blocking during commit |

### Agent-API

| Component | File | Changes |
|-----------|------|---------|
| Handler | `agent-go/internal/handler/commits.go` | `GET /commits?target=...` endpoint |
| Git | `agent-go/internal/gitops/gitops.go` | Target-based `git format-patch` execution and synthetic bundle generation |

### Frontend

| Component | File | Changes |
|-----------|------|---------|
| Types | `ui/src/lib/api-types.ts` | Add `targetRef` and `appliedCommit` session fields |
| Tool renderer | `ui/src/lib/components/ai/tool-renderers/` | Show approval-time commit preview and raw patch/diff views |
| Session UI | `ui/src/lib/components/app/` | Surface public status/error states and commit action wiring |

---

## Database Schema

```sql
ALTER TABLE sessions ADD COLUMN commit_error TEXT DEFAULT '';
ALTER TABLE sessions ADD COLUMN target_ref TEXT;
ALTER TABLE sessions ADD COLUMN applied_commit TEXT DEFAULT '';
ALTER TABLE session_commit_logs ADD COLUMN target_ref TEXT;
ALTER TABLE session_commit_logs ADD COLUMN target_commit TEXT;
```

# Frontend / Server Compatibility Changes

This document describes the changes needed in the discobot server and frontend to work with the `agent-go` backend. These changes are **not yet applied** — `agent-go` is currently standalone and does not affect the existing system.

## Route Changes

The agent-go API uses different route naming than the original TypeScript agent-api:

| Original (agent-api) | New (agent-go) | Notes |
|---|---|---|
| `GET /session/{id}/{agent}/chat` | `GET /threads/{id}/chat/stream` | History replay and live deltas now share the SSE endpoint |
| `GET /session/{id}/{agent}/chat` (SSE) | `GET /threads/{id}/chat/stream` | Separate SSE endpoint |
| `POST /session/{id}/{agent}/chat` | `POST /threads/{id}/chat` | Removed `{agent}` path param; interrupted turns auto-resume server-side |
| `POST /session/{id}/{agent}/cancel` | `POST /threads/{id}/cancel` | Removed `{agent}` path param |
| `GET /session/{id}/{agent}/chat/status` | _(removed)_ | Server doesn't have equivalent |
| `GET /sessions` | `GET /threads` | Renamed |
| _(none)_ | `GET /threads/activity/stream` | Session-level activity SSE: initial snapshot, change snapshots, and periodic resnapshots |
| Question routes | `GET /threads/{id}/chat/question` and `GET /threads/{id}/chat/question/{questionId}` | Current pending question or question-by-id |
| Answer routes | `POST /threads/{id}/chat/answer/{questionId}` | Path param instead of body |

### Key differences

1. **No `{agent}` path parameter** — routes are `/threads/{id}/...` not `/session/{id}/{agent}/...`
2. **`sessions` → `threads`** — a session can have multiple threads
3. **History replay happens on the SSE endpoint** — `GET /chat/stream` replays persisted messages and then continues with live deltas
4. **Chat SSE is long-lived** — `GET /chat/stream` no longer closes after a single completion; it stays connected and emits `ping` events while idle, without a terminal `done` event
5. **Sessions stay `ready` while chat streams** — completion activity is projected through `/threads/activity` snapshots and `/threads/activity/stream`, not by changing the sandbox session lifecycle status
6. **Activity sync is snapshot-plus-events** — the server can fetch `/threads/activity` once or connect to `/threads/activity/stream`, which emits an initial aggregate snapshot, another `activity` event whenever completion lifecycle, queue, thread, or answer state changes, and periodic resnapshots while subscribed
7. **Interrupted-turn recovery is stream-anchored** — resume streams now always start with either `start` or `data-thread-resume`, a new user prompt closes the interrupted turn before starting a fresh completion, preserves any recoverable partial assistant content in history, and queued follow-ups such as hook-failure re-prompts resume interrupted turns instead of surfacing a raw `interrupted turn requires resume` error

## Server Changes Needed

### `server/internal/sandbox/sandboxapi/types.go`

Update sandbox API types to match the new route structure:
- Rename session-based types to thread-based types
- Update question/answer request/response types to use path params

### `server/internal/service/sandbox_client.go` and `sandbox_session_client.go`

Update HTTP client calls to use new routes:
- `/session/{id}/{agent}/chat` → `/threads/{id}/chat`
- `/session/{id}/{agent}/cancel` → `/threads/{id}/cancel`
- `/sessions` → `/threads`
- Question/answer routes to use path params

### `server/internal/handler/chat.go`

Update handler to proxy to new agent-go routes and adapt request/response formats.
The downstream proxying logic also needs to treat chat SSE as a persistent
connection with periodic `ping` events between completions, and it should not
expect a terminal `done` event from the stream itself.

### `server/internal/service/chat.go`

Update service layer to match new route structure.

## Frontend Changes Needed

### `lib/api-types.ts`

- Update thread summary types to include `cwd` for the thread-specific tool working directory
- Update thread summary types to include `phase`; `phase: "review"` marks a thread ready for review and unlocks hooks with `phase: review`
- Update thread summary types to include `pendingQuestion`
- Update create/update thread request types to accept optional `cwd`
- Update chat request types to accept optional `freshContext`, `subagentType`, and `maxTurns`
- The chat start response and question-answer response may include `completionId`
  so clients can attach to the resumed stream after a pending answer is accepted
- Update types for question/answer to use `questionId` path param
- Thread-based naming where applicable

### `lib/api-client.ts`

- Update API client methods to call new routes
- Question/answer endpoints use path params instead of query/body
- Keep chat SSE subscriptions open across multiple completions and ignore
  `ping` events except for connection liveness

### `components/ai-elements/tool-renderers/ask-user-question-tool.tsx`

- Update to use new question ID-based routes

### `components/ide/ask-question-dialog.tsx`

- Update to use new answer submission route

## Message Format

The agent-go message format is compatible with the existing UI. Key points:

- Messages use the same `role` + `parts[]` structure
- Part types (`text`, `tool-call`, `tool-result`, `reasoning`, etc.) are the same
- UI projection (`ProjectUIMessages`) produces the same JSON format
- SSE streaming uses the same chunk format
- Response timing is exposed through standard `message.metadata`
  fields (for example `startedAt` / `finishedAt`) rather than custom
  top-level `UIMessage` fields or custom chunk shapes
- Chat SSE additionally emits `ping` events with `{}` payloads to keep the
  connection alive between completions
- Chat SSE may emit `data-thread-name` chunks before the assistant reply when
  the backend auto-generates or refines a friendly thread title from the first
  user prompt using agent-go's internal supporting-model selection

## System Prompt Delivery

Agent-go delivers the system prompt and reminder content differently than a
simple concatenation:

1. **System prompt** (behavioral instructions) → injected as `role: "system"` root message
2. **User instructions** (`AGENTS.md`, with provider-specific fallbacks like `CLAUDE.md` and `GEMINI.md`, plus rules) → injected as `role: "user"` message with `<system-reminder>` tags, matching Discobot's agent instruction delivery format
3. **Runtime/bootstrap reminders** (runtime snapshot, recent threads, visible
   skills/scripts) → injected as synthetic `role: "user"` reminder messages at
   thread bootstrap
4. **Per-turn reminders** (for example mode changes, credential changes, and
   workspace files changed since the prior completed turn) → injected as
   synthetic `role: "user"` prelude messages immediately before the new user
   prompt

This is transparent to the frontend — these messages appear in the thread history like any other messages.

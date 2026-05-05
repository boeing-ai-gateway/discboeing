# ACP-backed agent implementation plan

This plan treats ACP as an internal backend protocol for one `agent.Agent`
implementation. Discobot should not expose the entire ACP feature surface just
because ACP supports it. The design goal is to prove and maintain that ACP is
sufficient to satisfy Discobot's existing agent interface.

Tests for ACP behavior should use a real fake ACP server process or listener and
make actual JSON-RPC calls to it. Avoid pure mocks for protocol behavior; keep
mocks only for narrow local dependencies that are not part of the ACP boundary.

## Current implementation status

The first ACP integration slice is complete. It proves ACP as an internal backend
protocol boundary with generated schema-backed types, a low-level JSON-RPC client,
and an ACP-backed `agent.Agent` adapter that can initialize, create/load/resume
sessions, prompt, cancel, synchronize sessions, and project replayed ACP updates
into Discobot-local thread history.

Completed:

- Removed model listing from the generic `agent.Agent` boundary.
- Added `agent-go/acp/client` for ACP JSON-RPC calls over MCP transports.
- Added generated `agent-go/acp/protocol` types from the checked-in schema
  snapshot.
- Added the custom ACP schema generator under `agent-go/acp/internal/cmd/acpgen`.
- Added `agent-go/acp/agent`, an ACP-backed `agent.Agent` implementation.
- Required a `ThreadStore` for the ACP adapter so local projection persistence is
  always available.
- Persisted ACP session state in typed `thread.ConfigMetadata.ACPSession`.
- Implemented `Prompt`, `Cancel`, `Messages`, and `ListThreads` for the ACP
  adapter.
- Implemented capability-gated `session/load`, `session/resume`, and
  `session/list` support.
- Implemented unknown ACP session import during `ListThreads` by generating a
  Discobot thread ID, calling `session/load`, and saving any replayed updates as
  local messages.
- Added integration-style tests with fake ACP servers over real JSON-RPC/network
  transport plus focused unit tests for update streaming, load projection, and
  metadata updates.

Still pending:

- ACP permission request bridging through `PendingQuestion`, `SubmitAnswer`, and
  `Resume`.
- Durable interrupted-turn state beyond the current default `false` result.
- `FinalResponse` computed from projected local messages.
- Command and mode state exposed through `ListCommands` or future mode APIs.
- ACP filesystem and terminal capability support.
- ACP `session/close` support.

## Phase 0: define the compatibility target

Document how each Discobot agent interface method maps to ACP or to local
Discobot state:

| Discobot method | Current ACP-backed source/status |
| --- | --- |
| `Prompt` | Implemented with ACP `session/prompt` plus `session/update` notifications. |
| `Cancel` | Implemented with ACP `session/cancel` for mapped sessions. |
| `Messages` | Implemented from Discobot-local persistence of translated ACP updates. |
| `ListThreads` | Implemented from local thread/session mapping plus ACP `session/list` reconciliation when available. |
| `Resume` | Not implemented for ACP permission/interrupted-turn resume; session restore internally uses ACP `session/load`/`session/resume` when prompted. |
| `HasInterruptedTurn` | Currently returns `false`; durable interrupted-turn state is pending. |
| `PendingQuestion` | Currently returns `nil`; ACP permission request bridging is pending. |
| `SubmitAnswer` | Currently unsupported; ACP permission request bridging is pending. |
| `FinalResponse` | Currently unsupported; computing it from local projected messages is pending. |
| `ListCommands` | Currently returns no commands; ACP command/mode state is pending. |

`ListModels` should not be part of the generic agent interface. Model discovery
belongs to provider/configuration code, not the conversation boundary that ACP
implements.

## Phase 1: remove or isolate model listing

Status: complete.

Remove `ListModels` from `agent.Agent` and from `CompletionManager`. The
sandbox client does not call the agent-go thread-scoped `/models` endpoint, and
the product UI uses the server's project-level model endpoint instead, so remove
the unused agent-go HTTP endpoint rather than preserving a model-listing path on
the agent boundary.

This prevents ACP-backed agents from having to fake provider model discovery.
If ACP model selection is needed later, represent it through ACP session config
or a Discobot-specific configuration layer instead of the generic agent
interface.

## Phase 2: add an ACP adapter skeleton

Status: mostly complete. The ACP client, generated protocol package, adapter, update
translation, typed metadata persistence, and local projection are implemented.
The permission broker and command/mode persistence are still pending.

Create an ACP-backed implementation in `agent-go/acp/agent`, backed by the
low-level ACP client and protocol packages in `agent-go/acp/client` and
`agent-go/acp/protocol`, that satisfies `agent.Agent` but initially stubs
unsupported behavior clearly.

Internal pieces should include:

- JSON-RPC transport, starting with stdio or a local network listener for tests.
- Session mapper for `Discobot threadID <-> ACP sessionId`.
- Update translator for ACP `session/update` to Discobot message chunks.
- Permission broker for ACP `session/request_permission` to Discobot pending
  questions.
- Local persistence for messages, thread metadata, commands, modes, and ACP
  session mapping.

## Phase 3: initialize and create ACP sessions

Status: complete for `initialize` and `session/new`. Authentication is not
implemented because the current ACP adapter does not advertise or require ACP
authentication.

Implement only the ACP lifecycle needed to start a prompt:

- `initialize`
- `session/new`
- optional `authenticate` only if the target ACP server requires it

For the initial implementation, advertise no filesystem or terminal capability
surface. The current initialize request sends an empty ACP client capability set
rather than enabling optional ACP filesystem or terminal methods.

Do not expose ACP filesystem or terminal methods in the MVP.

Tests should launch the fake ACP server and verify that the client sends real
`initialize` and `session/new` JSON-RPC requests.

## Phase 4: implement prompt turns

Status: complete for prompt submission, content conversion, stateful text and
reasoning streaming, tool call/result chunks, finish chunks, and session info /
config option metadata updates. Plan, command, and mode updates are recognized by
the generated protocol but are not yet surfaced through Discobot APIs.

Map `agent.Agent.Prompt` to ACP `session/prompt`:

1. Ensure the Discobot thread has an ACP session.
2. Convert Discobot user parts to ACP content blocks.
3. Send `session/prompt`.
4. Receive ACP `session/update` notifications.
5. Translate known updates to Discobot chunks. Local message history is persisted
   by the existing completion/thread pipeline for prompt turns, and by the ACP
   load projection when `session/load` replays updates during import.
6. Emit a finish chunk from the ACP prompt response stop reason.

Support the update variants needed by Discobot UI first:

- `agent_message_chunk`
- `agent_thought_chunk`, if present
- `tool_call`
- `tool_call_update`
- `plan`
- `current_mode_update`
- `available_commands_update`
- `session_info_update`, if present

Unknown updates should not crash the turn.

## Phase 5: implement cancellation

Status: complete for mapped ACP sessions and prompt context cancellation.

Map `agent.Agent.Cancel(threadID)` to ACP `session/cancel` for the active ACP
session/prompt, cancel local context, and finish local stream state consistently
with the existing API.

## Phase 6: bridge permissions

Status: pending. The current ACP client only handles server notifications that
arrive while waiting for a response; ACP client-side JSON-RPC request handling
for permission prompts has not been implemented.

ACP permission requests are synchronous JSON-RPC calls from agent to client.
Discobot's approval UI is asynchronous.

When the ACP server calls `session/request_permission`:

1. Create a pending approval for the Discobot thread.
2. Expose it through `PendingQuestion`.
3. Block the ACP JSON-RPC response until the user answers, cancels, or the
   prompt context ends.
4. `SubmitAnswer` resolves the pending approval and returns the selected ACP
   option to the server.

MVP limitation: if `agent-go` crashes while an ACP permission request is
pending, the in-flight JSON-RPC request cannot be resumed unless the ACP server
supports a durable resume flow. Completed local history should remain intact.

## Phase 7: implement local history methods

Status: partially complete. `Messages` and `ListThreads` are implemented from
Discobot-local state. `ListThreads` also reconciles ACP `session/list` when
available and imports unknown ACP sessions. `FinalResponse` and durable
`HasInterruptedTurn` state are still pending.

Satisfy non-streaming methods from Discobot-local state rather than requiring ACP
optional history APIs:

- `Messages`
- `ListThreads`
- `FinalResponse`
- `HasInterruptedTurn`

For ACP-backed agents, Discobot's thread/message store is a local projection of
ACP session state. ACP remains the source of truth for ACP sessions, but it is
only available while the ACP server is running. Synchronization therefore happens
at demand boundaries:

- `Prompt` creates or restores the mapped ACP session and persists updated ACP
  session metadata.
- `ListThreads` calls ACP `session/list` when supported. Known ACP session IDs
  refresh their stored Discobot thread config. Unknown ACP session IDs are
  imported by generating a new Discobot thread ID, calling `session/load`, and
  saving any replayed `session/update` notifications as local messages.
- `Messages` reads the local message projection from the thread store.

`session/load` returns session UI state such as config options and modes. Message
history is only hydrated when the ACP server replays history as `session/update`
notifications during load. If an ACP server does not replay those updates,
Discobot can still persist the thread/session mapping but cannot reconstruct
older messages from ACP alone.

## Phase 8: implement commands and modes minimally

Status: pending. ACP config option updates are persisted in thread metadata, but
available commands and current mode changes are not yet surfaced through
Discobot's agent APIs.

Use ACP `available_commands_update` to populate command state, but keep command
semantics scoped to Discobot's interface. Avoid exposing ACP commands that cannot
be invoked through the Discobot prompt flow.

For modes, map Discobot `plan` and `build` to ACP `session/set_mode` or config
options only when the ACP server advertises compatible support. Otherwise ignore
or reject mode changes with a clear unsupported error, depending on product
requirements.

## Phase 9: decide separately on ACP filesystem and terminal

Status: pending by design. The ACP adapter still advertises no filesystem or
terminal capability surface.

Keep ACP filesystem and terminal capabilities disabled until the prompt,
permission, history, command, and cancellation path works.

If enabled later, implement them with explicit workspace path validation,
permission policy, output limits, and tests for path escape and command-control
edge cases.

## Phase 10: optional durable ACP session support

Status: partially complete. Capability-gated `session/list`, `session/load`, and
`session/resume` are implemented. `session/close` is still pending.

Only after the MVP works, add capability-gated support for:

- `session/list`
- `session/load`
- `session/resume`
- `session/close`

These improve durability and interoperability but are not required to prove that
ACP can satisfy Discobot's current interface.

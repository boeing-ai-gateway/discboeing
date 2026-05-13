# Sandbox Agent Control Socket

## Summary

Discobot maintains one persistent sandbox agent control WebSocket for each
running agent sandbox. The server initiates the connection to the agent-api
WebSocket endpoint inside the sandbox; the agent accepts that connection and
uses it as the shared server-agent control socket.

The socket abstraction is intentionally generic:

- low-volume state can be published as named control changes on the `control`
  channel in future features, and
- byte-oriented features use named streams whose channel name identifies the
  feature and request, such as `git:<id>`.

Feature-specific logic lives outside the generic socket plumbing. Git smart HTTP
is the only current production feature and is implemented as a feature layered
on top of named byte streams. Session activity and thread status continue to use
the existing sandbox HTTP/SSE endpoints.

## Goals

- Use a single persistent server-agent WebSocket per sandbox.
- Support multiple logical feature streams over that socket.
- Keep the user's workspace repository as the source of truth.
- Avoid a Discobot-owned bare repository or repo synchronization layer.
- Avoid Git hooks for branch authorization.
- Let the sandbox use normal Git commands: `clone`, `fetch`, `pull`, and `push`.
- Restrict sandbox pushes to a session-specific branch.
- Keep business logic out of the generic control socket transport.

## Non-goals

- Implement the Git smart HTTP protocol ourselves.
- Expose the local workspace repository directly to untrusted network clients.
- Provide branch-level authorization from HTTP paths alone.
- Guarantee semantic validity of pushed commits before accepting them. The
  allowed branch is still treated as untrusted input after a push.

## Architecture

```text
Sandbox
┌──────────────────────────────────────────────────────────┐
│ git client                                                │
│   ↓ HTTP                                                  │
│ agent localhost Git endpoint                             │
│   ↓ named byte stream (`git:<id>`)                       │
│ sandbox agent control socket endpoint                    │
└───────────────────────┬──────────────────────────────────┘
                        │ server-initiated persistent WebSocket
                        │ generic frames
                        ▼
Server
┌──────────────────────────────────────────────────────────┐
│ sandbox agent control socket dialer                      │
│   ↓ generic dispatcher                                   │
│ `git:<id>` stream handler                                │
│   ↓ streamed request                                     │
│ git http-backend against user's workspace repo           │
└──────────────────────────────────────────────────────────┘
```

The control socket is authenticated as a specific sandbox session. Feature code
must derive workspace identity, authorization, and policy from that socket
identity rather than trusting values supplied by the sandbox client.

## Control socket framing

The socket carries typed frames. Each frame belongs to a named channel. The
`control` channel carries named changes. Any other channel can represent a
feature-owned stream.

Conceptual envelope:

```go
type Frame struct {
    Version int
    ID      string
    Channel string
    Type    string
    Name    string
    Payload []byte
    Data    []byte
}
```

Frame types currently used by the generic transport:

| Type | Purpose |
|------|---------|
| `change` | A low-volume named control change. `Name` identifies the change. |
| `stream.open` | Opens a named stream and may carry feature metadata in `Payload`. |
| `stream.data` | Carries raw bytes for a named stream in `Data`. |
| `stream.close_write` | Half-closes the sender's write side of a stream. |
| `stream.close` | Closes/cancels a stream. |
| `error` | Reports a stream or control error. |

The current implementation encodes each frame as JSON, including `Data`. If Git
traffic grows enough for encoding overhead to matter, the same abstraction can
move to binary WebSocket messages or a compact binary frame format without
changing feature ownership.

## Named control changes

The `control` channel is reserved for low-volume notifications and state
snapshots in future features. A change frame uses:

```text
Channel: control
Type:    change
Name:    <feature-owned name>
Payload: <feature-owned JSON>
```

No current production feature publishes named control changes. In particular,
session activity is not routed over the control socket; `SessionThreadStatusSyncer`
continues to read activity snapshots through the sandbox agent's existing
`/threads/activity` and `/threads/activity/stream` HTTP/SSE endpoints.

## Named byte streams

Stream channels are feature-owned names. A Git HTTP request uses a channel like:

```text
git:<id>
```

The generic agent-side control socket API exposes the stream as a byte-oriented
object. Feature code can write request or response bytes while using
`stream.open` payloads for feature metadata.

## Git HTTP stream

The sandbox agent listens on localhost inside the sandbox, for example:

```text
http://127.0.0.1:<port>/workspace.git
```

The agent configures the sandbox checkout with a Discobot remote whose push
refspec targets the session branch:

```bash
git remote add discobot http://127.0.0.1:<port>/workspace.git
git config remote.discobot.push HEAD:refs/heads/discobot/<session-id>
```

For each local HTTP request, the Git feature opens a stream named `git:<id>`.
The opening payload contains the HTTP method, path, query string, and a
sanitized set of headers needed by Git smart HTTP, such as `Content-Type`,
`Content-Length`, and `Git-Protocol`. The request body is sent as
`stream.data`, followed by `stream.close_write`.

The server runs `git http-backend`, sends response status and headers as a
`stream.open` payload on the same stream, streams the response body as
`stream.data`, and ends the response with `stream.close_write`.

Request and response bodies must stream. The proxy must not buffer an entire
push or fetch in memory. If the local Git client disconnects, the agent closes
the stream; if the WebSocket drops, both sides cancel all active streams and
terminate any associated `git http-backend` process.

## Server-side Git backend

The server handles tunneled Git HTTP requests by invoking Git's built-in smart
HTTP backend:

```bash
git http-backend
```

The server maps the authenticated control socket to the workspace path and sets
CGI-style environment variables for `git http-backend`, including:

```text
GIT_PROJECT_ROOT
GIT_HTTP_EXPORT_ALL=1
REQUEST_METHOD
PATH_INFO
QUERY_STRING
CONTENT_TYPE
CONTENT_LENGTH
REMOTE_USER
```

The response from `git http-backend` is a CGI response. The server parses the
CGI headers, then streams the response status, headers, and body back over the
same `git:<id>` stream.

## Push authorization without hooks

The server does not rely on HTTP routes to determine which ref a push updates.
For smart HTTP pushes, target refs are inside the Git receive-pack protocol
body. Branch authorization is therefore enforced by `git receive-pack` itself.

For sandbox pushes, the server invokes `git http-backend` with process-local Git
configuration injected through environment variables:

```text
http.receivepack=true
receive.hideRefs=refs/
receive.hideRefs=!refs/heads/discobot/<session-id>
receive.denyDeletes=true
receive.denyNonFastForwards=true
```

`receive.hideRefs` applies to `receive-pack`; attempts to update or delete a
hidden ref by `git push` are rejected. Hiding `refs/` and then un-hiding the
session branch means the sandbox can only update:

```text
refs/heads/discobot/<session-id>
```

The injected config is scoped to the `git http-backend` process for the current
request. It is not written into the user's repository config.

Fetch and clone requests use the normal upload-pack path so the sandbox can see
the workspace's normal branches and tags unless a future feature requires
narrower read access.

## Git state modified by sandbox pushes

With the push policy above, the sandbox can modify only:

- objects uploaded as part of the push, and
- `refs/heads/discobot/<session-id>`.

It should not be able to update `main`, tags, `HEAD`, Git config, hooks, the
index, or working tree files through Git smart HTTP. Uploaded objects are still
untrusted input and may be inspected by Discobot before being applied or
surfaced in privileged flows.

## Checked-out branch behavior

Because the sandbox is only allowed to push to its dedicated Discobot branch, it
should not push to the branch checked out in the user's workspace. This avoids
Git's normal refusal to update the currently checked-out branch in a non-bare
repository.

If a future mode intentionally allows pushing to the checked-out branch, the
server can consider `receive.denyCurrentBranch=updateInstead`, but that is not
part of the dedicated-branch policy.

## Security model

- The control socket is authenticated as a sandbox session.
- The sandbox may request Git HTTP operations, but the server derives the
  workspace path and allowed push ref from the authenticated session.
- The local Git endpoint binds only to localhost inside the sandbox.
- Pushes are allowed only to the session branch by `receive.hideRefs`.
- Deletes and non-fast-forwards are denied by default.
- No hooks or repo config mutation are required for the authorization policy.

## Implementation phases

1. Add the authenticated control socket with generic named changes and streams.
2. Add the Git feature using `git:<id>` streams and the agent localhost HTTP
   endpoint for clone/fetch/push.
3. Add server-side `git http-backend` support for receive-pack with
   process-local `receive.hideRefs` push authorization.
4. Configure the sandbox remote push refspec to target the session branch.
5. Surface pushed session-branch changes in the existing session/workspace UI.

## Open questions

- Should the JSON frame encoding be replaced with a binary format before large
  Git operations are enabled by default?
- Should Git streams be available immediately on socket connection or negotiated
  through a `control` feature handshake?
- What branch naming should be used for sessions with user-visible names,
  deleted sessions, or recreated sandboxes?
- How should the UI present the session branch and any pushed changes?
- Should Discobot validate the pushed branch after receive and mark it trusted,
  rejected, or pending review?

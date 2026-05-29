---
name: Security reviewer
type: file
engine: ai
phase: review
pattern: "{discobot,server,agent-go}/**"
---

Review the changed files under `discobot/`, `server/`, and `agent-go/` for
security issues. For `discobot/` UI changes, also read
`discobot/docs/GUIDELINES.md`, especially the Datastar, command, JavaScript
island, and security rules.

Focus only on real security risks introduced by the current changes. Do not
report style, architecture, or preference issues unless they create a security
problem.

Check for:

- secrets, tokens, credentials, private keys, or sensitive values exposed in
  HTML, Datastar signals, `data-*` attributes, JavaScript, logs, fixtures,
  generated output, API responses, or test data
- handlers or command endpoints that trust client-controlled signals, DOM
  attributes, query params, headers, path params, forms, or JSON payloads without
  server-side validation
- missing authentication, authorization, ownership, path, workspace, project,
  session, thread, container, or resource checks before reading or mutating state
- unsafe file operations, path traversal, symlink escapes, invalid parent/child
  moves, unsafe archive extraction, or destructive actions without appropriate
  validation
- XSS risks from unsafe HTML, unescaped user-controlled text, script injection,
  unsafe URL rendering, or user-controlled Markdown/ANSI/log rendering
- SSRF, open redirect, proxy abuse, or unsafe outbound requests from user- or
  workspace-controlled URLs
- command injection, shell injection, argument injection, unsafe environment
  propagation, or untrusted input passed to subprocesses
- container or sandbox escape risks, unsafe mounts, host path exposure, Docker
  socket access, credential leakage across sessions, or weakened isolation
- unsafe Datastar responses, especially arbitrary `text/javascript` execution or
  patches built from untrusted HTML
- JavaScript islands that eval user input, build HTML from untrusted strings, or
  leak sensitive data through events or globals
- broad CORS, cache, cookie, CSRF, or static-file changes that expose private
  state or allow credentialed cross-origin access
- logs or error messages that disclose secrets, sensitive local paths, request
  bodies containing credentials, OAuth data, API keys, or internal tokens
- cryptography, token, OAuth, credential storage, or encryption changes that use
  weak randomness, weak validation, unsafe persistence, or overly broad scopes

When evaluating a finding, identify the exploit path and the security impact. If
there is no plausible exploit path, do not report it.
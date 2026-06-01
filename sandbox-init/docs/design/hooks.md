# Session Hooks

Sandbox init no longer runs session hooks. The setup script only prepares the
filesystem and writes `DISCOBOT_HOOKS_ENABLED=true` into
`/run/discobot/agent-env` so `discobot-agent-api` can manage hook execution.

Hook files still live under `.discobot/hooks/` in the workspace. The agent API is
responsible for parsing, running, retrying, and recording hook status after the
sandbox setup service has completed.

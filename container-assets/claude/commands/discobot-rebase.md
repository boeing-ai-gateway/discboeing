---
name: discobot-rebase
description: Rebase session changes onto the tracked upstream branch
discobot-ui: true
discobot-label: Rebase
discobot-order: 20
---

Rebase this session's git history onto whatever upstream branch the current branch tracks.

1. **Inspect current state:** Run `git status` and `git log --oneline -5` to confirm current HEAD and working tree state.

2. **Prepare changes for rebase as needed:**
   - If there are uncommitted local changes, choose the safest path to make rebase possible.
   - You may commit changes when appropriate, but you are **not required** to fully finalize all staged content for workspace transfer.
   - Keep changes staged if that is the best path for resolving rebase conflicts.

3. **Resolve the tracked upstream and rebase onto it:**
   - Determine the upstream branch with `git rev-parse --abbrev-ref --symbolic-full-name @{upstream}`.
   - If no upstream is configured, explain that clearly and stop instead of guessing a branch or commit.
   - Fetch the latest refs for that remote before rebasing.
   - Run `GIT_EDITOR=true git rebase --autostash <upstream>` to rebase current history onto the tracked upstream while preserving uncommitted work.
   - Keep rebase-related git commands non-interactive in this environment so Git does not block waiting for an editor.

4. **Resolve conflicts when needed:**
   - If rebase conflicts occur, surface them clearly to the user.
   - Use `git status` to list conflicts.
   - After resolving conflicts, continue with `GIT_EDITOR=true git rebase --continue`.
   - If the user asks to stop, use `git rebase --abort`.

5. **Verify:**
   - Confirm the sandbox history is rebased onto the tracked upstream branch.
   - Ensure git state is coherent before finishing.

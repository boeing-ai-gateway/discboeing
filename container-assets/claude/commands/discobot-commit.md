---
name: discobot-commit
description: Commit session changes back to the parent workspace
---

Commit the changes from this session back to the parent workspace.

1. **Inspect the current repo state first:**
   - Run `git status` and `git log --oneline -5`
   - Understand the current HEAD, uncommitted changes, and any existing local commits

2. **Prepare the sandbox commit(s):**
   - Review the diff before committing
   - Create one or more logical git commits for the work that should be pulled back
   - Use imperative commit messages with a short subject line
   - Leave the worktree clean before moving on

3. **Make sure the prepared commits are based correctly:**
   - Confirm the prepared commits are based on the current sandbox branch state you intend to pull back
   - If they are not, rebase non-interactively onto the correct base before continuing
   - Keep any rebase-related git commands non-interactive in this environment so Git does not wait for an editor
   - If conflicts occur, stop and work with the user to resolve them before continuing

4. **Request the host pull:**
   - After the sandbox commits are ready, call `RequestCommitPull`
   - Include optional additional notes that may be important for the end user to know
   - Do not claim the work was pulled yet; the tool result is authoritative

5. **Wait for the tool result before responding:**
   - If the tool reports success, confirm the pull succeeded and include the pertinent result details
   - If the tool reports failure or rejection, explain that clearly and include the returned reason
   - Do not say the changes landed in the host workspace unless the tool explicitly reports success

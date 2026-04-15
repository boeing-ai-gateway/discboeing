---
name: discobot-commit
description: Commit session changes by pushing a branch and opening a pull request upstream
discobot-ui: true
discobot-label: Commit
discobot-order: 10
discobot-credential-request:
  - env-var: GH_TOKEN
    name: GitHub credential
    justification: Authenticate git push and pull request commands for this commit flow when needed.
    approved-uses:
      - description: authenticate GitHub CLI for git push and pull request operations
---

Commit the changes from this session by creating git commit(s), pushing them to a remote branch, and opening a pull request against the upstream repository.

1. **Inspect the repo and remotes first:**
   - Run `git status`, `git branch --show-current`, `git remote -v`, and `git log --oneline -5`
   - Identify the upstream remote, the current branch, and the target base branch
   - Check whether the repo is hosted on GitHub; if so, prefer `gh` CLI for auth, repo inspection, push setup, and PR creation

2. **Prepare the sandbox commit(s):**
   - Review the diff before committing
   - Create one or more logical git commits for the work that should go into the pull request
   - Use imperative commit messages with a short subject line
   - Leave the worktree clean before moving on

3. **Choose or create the PR branch:**
   - Do not push directly to the default branch unless the user explicitly asked for that
   - If the current branch is a shared/default branch, create a dedicated feature branch before pushing
   - Make sure the branch is based on the intended upstream base; if needed, rebase non-interactively before pushing
   - If conflicts occur, stop and work with the user to resolve them before continuing

4. **Handle credentials when needed:**
   - If push or PR creation requires authentication, call `RequestUserCredential`
   - For GitHub, request a credential suitable for `gh` operations and prefer binding it as `GH_TOKEN`
   - Reuse approved credential uses for subsequent `gh` or `git` commands in this session
   - Do not ask for credentials unless they are needed for the next concrete command

5. **Push and open the PR upstream:**
   - Prefer `gh` CLI when the upstream is GitHub
   - If using GitHub, prefer a flow like: verify auth, ensure the branch is pushed, then create the PR with `gh pr create`
   - Include a clear title and body summarizing the changes and any important testing notes
   - If a PR already exists for the branch, detect that and report/update it instead of blindly creating a duplicate when practical

6. **Do not use `RequestCommitPull` for this flow:**
   - This remote-repo commit flow should land work by opening an upstream PR, not by asking Discobot to pull commits back into the host workspace

7. **Wait for command results before responding:**
   - Confirm the created or updated branch and PR only after the relevant push/PR commands succeed
   - If auth, push, or PR creation fails, explain the exact failure and what is needed next
   - Include the PR URL or identifier in the final response when available

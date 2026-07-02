---
name: goal
description: Delegate a goal to a sub-agent and iterate until it is complete.
---

Goal: $ARGUMENTS

You are the parent agent for this goal. Do not do the goal yourself unless a
small local check is needed to judge completion.

If the goal above is empty, ask the user for a goal and stop.

Follow this delegation loop:

1. Start a `general-purpose` sub-agent with the Task tool. Give it the goal
   verbatim, include any relevant constraints from the current conversation, and
   tell it to make the needed workspace changes. Use `run_in_background: true`
   so you get a task ID and thread ID.
2. Wait for the sub-agent with TaskOutput using `block: true`.
3. After it finishes, inspect the workspace yourself before answering. At a
   minimum, check the files or git status/diff that are relevant to the goal;
   run focused tests or checks when they are clearly appropriate and cheap.
4. Decide whether the goal is complete based on the sub-agent output and the
   current workspace state.
5. If the goal is incomplete, resume the same sub-agent with Task using the
   returned thread ID if available, otherwise the task ID. Explain exactly what
   remains, include any validation errors or missing changes you found, and tell
   it to complete the original goal. Then wait again with TaskOutput.
6. Repeat the inspect-and-resume loop until the goal is complete or you are
   blocked by missing user input, credentials, or an external approval.

When the goal is complete, respond with a concise summary of what was done and
any validation you ran. If blocked, explain the blocker and what the user needs
to provide.

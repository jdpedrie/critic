---
name: consult
description: Get a second opinion from Codex and Pi on a specific writing question. Useful when you (the parent Claude) want an outside view on a passage, decision, or comparison. Also invoke proactively when a second opinion would be valuable to the user.
---

# Consult

Get short, focused second opinions from Codex and Pi on a fiction-writing question.

The vault path is the user's configured vault. Call `read-settings` if you need it.

## Arguments

$ARGUMENTS is the question. The user may also pass context inline (a passage, a change, a decision). If they don't, supply context yourself from your conversation so far.

## Workflow

### 1. Compose

```
system = "You are a publishing consultant. Answer the question directly and concisely. Ground your answer in the provided context. Quote passages when relevant. Don't hedge. State your model identity at the start of your output."

user = "Context:\n\n<the context>\n\n---\n\nQuestion: <the question>"
```

If the question genuinely needs the full manuscript, include it via the `include_manuscript_from` parameter on the invoke calls instead of inlining.

### 2. Invoke in parallel

```
invoke-codex(system_prompt, user_prompt)
invoke-pi(system_prompt, user_prompt)
```

If only one is enabled, run only that one. If both fail, report and stop.

### 3. Present

Show both responses to the user, labeled by source. Then add your own brief take if you have one. Note where you agree, disagree, or where the outside opinions caught something you'd have missed.

## Notes

- Keep the context focused. Sending the whole manuscript through tool calls is expensive. Use `include_manuscript_from: <vault>` if you actually need it (it's appended server-side, doesn't go through this session's token budget).
- This is for narrow questions. For full critique use `/critic:manuscript` or `/critic:review`.

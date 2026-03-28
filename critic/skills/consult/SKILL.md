---
name: consult
description: Get a second opinion from Codex and Gemini on a writing question.
---

# Consult

Get outside opinions from other AI models on a specific question.

## When to use

- The user asks you to evaluate a passage, change, or decision and you think a second opinion adds value
- The user explicitly asks for other perspectives
- You're uncertain about a craft judgment and want to check your instinct
- The user asks "what do you think about X" and the question is concrete enough to send to another model

## How to use

Call the `consult` MCP tool with:
- `question`: The specific question to ask. Be precise — "Does Kael's motivation in paragraph 3 feel earned?" not "What do you think of this chapter?"
- `context`: The relevant passage, diff, or background. Keep it focused — send the relevant paragraphs, not the whole chapter.

The tool sends the question to all enabled non-Claude models (Codex, Gemini) and returns their responses.

## After receiving responses

Present the outside opinions naturally in conversation. Don't dump them verbatim — synthesize:
- Where the other models agree with your assessment, note the consensus briefly
- Where they disagree, present the disagreement and the reasoning
- Where they caught something you missed, acknowledge it
- Give your own final take, informed by the outside opinions

## Important

- This is for **specific questions**, not full reviews. For full reviews use `/critic:review` or `/critic:manuscript`.
- Include everything relevant as context — excerpts, full chapters, world-building notes — whatever the other models need to give an informed answer. But don't overshare irrelevant material. Use judgment.
- The user doesn't need to ask for this explicitly. If you judge that a second opinion would be valuable, use it proactively.

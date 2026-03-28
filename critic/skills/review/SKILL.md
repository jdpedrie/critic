---
name: review
description: Run multi-agent fiction review on a chapter. Dispatches reviewer tools in parallel, runs cross-review, synthesizes into readable critique. Use when the user asks to review, critique, or get feedback on a chapter.
---

# Fiction Review

The vault path for all tool calls is: ${user_config.vault_path}

Run a structured multi-agent review of the specified chapter using the critic MCP tools.

## Arguments

$ARGUMENTS should be a chapter name (e.g., "chapter-01") or a chapter filename.

## Workflow

### Round 1 — Independent Reviews

Call ALL FOUR review tools in parallel using the Agent tool:
- `review-analytical` with the chapter name
- `review-immersive` with the chapter name
- `review-structural` with the chapter name
- `review-adversarial` with the chapter name

Wait for all four to complete.

Present a brief summary to the user:
- Total issue count across all reviewers
- Breakdown by severity (critical / moderate / minor)
- Any issues flagged by multiple reviewers (overlapping claims)
- Notable strengths mentioned

Then ask: **"Run cross-review, skip to synthesis, or re-run a specific reviewer?"**

### Round 2 — Cross-Review (if requested)

Call the `cross-review` tool twice in parallel:
1. Text-only pair: pass the analytical and immersive review outputs, with model_a="claude" and model_b="codex"
2. Full-context pair: pass the structural and adversarial review outputs, with model_a="claude" and model_b="codex"

Present:
- Number of agreements and disagreements per pair
- Key contested points (summarize the disagreement, don't dump JSON)

Then ask: **"Synthesize, or discuss specific points first?"**

### Round 3 — Synthesis

Call the `synthesize` tool with:
- `reviews`: JSON object mapping role names to their review JSON strings
- `rebuttals`: JSON object mapping pair names to their rebuttal JSON strings (if cross-review was run)

The synthesize tool returns a JSON object with a `markdown` field containing the full human-readable report.

**Present the markdown report directly in conversation.** This is the primary output.

### After Synthesis

Offer the user these options:
- **Discuss**: "Want to dig into any specific issue?"
- **Re-run**: "Re-run a reviewer with different focus?"
- **Memory**: "Update reviewer memory with your decisions?" (call `update-memory` tool)
- **Save**: "Save this report to the vault?" (write the markdown to the reviews directory)

## Important Notes

- Each MCP tool call returns JSON. Do NOT show raw JSON to the user — always summarize or render as readable text.
- The synthesized markdown report is the final deliverable. Present it as-is.
- If any tool call fails, report the error clearly and ask how to proceed.
- You can run selective reviews — if the user asks for "just text-only reviewers", only call analytical and immersive.

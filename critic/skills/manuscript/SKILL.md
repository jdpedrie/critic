---
name: manuscript
description: Review the full manuscript using Claude (with rejection pass), Codex, and an adversarial model. Cross-review, synthesis, save. Use when the user wants honest, agency-level feedback on the whole book.
---

# Manuscript Review

The vault path for all tool calls is: ${user_config.vault_path}

Run the full manuscript review pipeline non-interactively. Two primary reviewers (Claude with rejection pass, Codex), one adversarial reviewer, cross-review, synthesis, save.

## Arguments

$ARGUMENTS is optional. If provided, it can specify focus areas.

## Workflow

Run all steps without stopping for user input.

### Step 0 — Prior Review + Review Number

Call these in parallel:
- `summarize-review` with prefix "manuscript-critic" and the vault path
- `next-review-number` with the vault path

Hold the prior review synthesis (if any) and the review number for subsequent steps.

### Step 1 — Independent Reviews

Call ALL available review tools in parallel:
- `review-manuscript-claude-rejection` with the vault path (and prior_review_summary if available)
- `review-manuscript-codex` with the vault path (and prior_review_summary if available)
- `review-manuscript-grok` with the vault path (and prior_review_summary if available)
- `review-manuscript-adversarial` with the vault path (no prior_review_summary — it gets the raw text only)

**Claude rejection tool** returns JSON with `review`, `rejection`, and `session_id`. Parse all three.
**Codex tool** returns JSON with `review` and `session_id`.
**Grok tool** returns JSON with `review` and `session_id`.
**Adversarial tool** returns prose directly (no session — one-shot rejection framing).

If any tool errors or is unavailable, proceed with the others. At least two of the three primary reviewers (Claude, Codex, Grok) must succeed to continue. The adversarial rejection pass is valuable but optional.

### Step 2 — Cross-Review

Call `cross-review-manuscript` with:
- `claude_review`: Claude's review text (not the rejection pass)
- `codex_review`: Codex's review text
- `gemini_review`: Grok's review text (pass via the gemini_review param — it's the third reviewer slot)
- `claude_session_id`: Claude's session ID
- `codex_session_id`: Codex's session ID
- `gemini_session_id`: Grok's session ID (pass via the gemini_session_id param)

Do NOT include the adversarial rejection in cross-review. It feeds directly into synthesis as a one-way input.

### Step 3 — Synthesis

Call the `synthesize` tool with:
- `reviews`: JSON object with "claude", "codex", "grok" keys mapping to their review text. Include the adversarial rejection as "adversarial". Include Claude's rejection pass as "claude_rejection".
- `rebuttals`: JSON object mapping model names to their rebuttal text from the cross-review
- `review_number`: the number from step 0

### Step 4 — Save

Call `save-review` with:
- `vault`: the vault path
- `prefix`: "manuscript-critic"
- `content`: a single markdown file structured as:

```
[synthesis output here]

<!-- RAW AGENT OUTPUTS BELOW — NOT INCLUDED IN FUTURE REVIEW CONTEXT -->

# Claude Review

[Claude's raw review]

---

# Claude Rejection Pass

[Claude's rejection pass output]

---

# Codex Review

[Codex's raw review]

---

# Grok Review

[Grok's constructive review, if available]

---

# Adversarial Rejection

[Adversarial rejection output, if available]

---

# Cross-Review

[full cross-review output]
```

Use H1 (`#`) for agent section headings. The sentinel MUST be included exactly as shown.

### Step 5 — Present

Tell the user where the file was saved and the review number. Present the synthesis in conversation. After the synthesis, separately highlight the rejection pass findings — these are the most important corrective signal.

## Important Notes

- Run all steps without asking. Do not stop between rounds.
- This reads the ENTIRE manuscript — story only, no world-building notes or plot outlines.
- The literary agent framing means all reviewers are evaluating for publishability, not providing encouragement.
- Three primary reviewers: Claude (with automatic rejection pass), Codex, and Grok. Grok uses the same constructive framing but is less aligned, which produces different observations.
- The adversarial rejection is a separate, fourth input — Grok again but with a "this was rejected, explain why" prompt. It is deliberately harsh.
- The synthesis weighs the rejection pass and adversarial rejection heavily as correctives against constructive bias.
- If any reviewer errors or is unavailable, proceed with the others. Minimum: two of three primary reviewers.
- All outputs are prose except the initial review steps (which wrap review + session_id in JSON).

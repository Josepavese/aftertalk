# Incremental minutes finalization loses earlier session content

## Summary

During a Mondopsicologi production recovery on 2026-05-07, Aftertalk successfully regenerated cloud minutes for previously failed sessions. The technical pipeline completed: API regeneration, cloud LLM provider selection, incremental batches, retries, finalization, DB persistence, and webhook delivery.

However, the long-session output is not clinically or operationally useful. The final minutes are valid JSON and structurally well formed, but they describe almost only the closing part of the meeting and lose most of the earlier session content.

This is a product-quality issue in the incremental minutes pipeline, not a deployment or Mondopsicologi integration issue.

## Production Evidence

### Main case: long session

- Project: Mondopsicologi production
- Session ID: `77794a9f-640a-4348-a7c9-04e9b7e30554`
- Regenerated on: 2026-05-07
- STT profile: `cloud`
- LLM profile: `cloud`
- LLM provider recorded in DB: `openai`
- OpenRouter model observed in logs: `minimax/minimax-m2.7`
- Session status after regeneration: `completed`
- Minutes status after regeneration: `ready`
- Webhook event: `delivered`
- Transcript rows: `240`
- Transcript characters: `15753`
- Transcript time range: `2464ms` to `1497230ms` (~24m55s)
- Minutes content characters: `2895`

The generated minutes are structurally valid and contain:

- top-level keys: `summary`, `sections`, `citations`
- section keys: `themes`, `contents_reported`, `professional_interventions`, `progress_issues`, `next_steps`
- citations count: `5`

The problem is semantic coverage.

### Actual output problem

The final `summary.phases` contains a single phase:

- start: `1369412ms`
- end: `1496930ms`

That means the generated high-level phase covers only the final ~2m07s of a ~24m55s session.

The final themes are concentrated on closing topics, such as:

- monthly management costs;
- collaboration with a named stakeholder;
- gradual system ramp-up;
- final greetings.

The earlier and central parts of the transcript are largely absent from the final minutes, even though they contain clinically relevant context and patient-reported material.

Examples of missing early/mid-session content, paraphrased from the transcript to avoid copying sensitive clinical data:

- patient introduces age and current situation;
- patient reports anxiety and frustration;
- patient describes professional uncertainty and business/economic exposure;
- patient reports sleep disturbance and night-time rumination;
- patient describes family/work/property pressure and uncertainty about the future;
- therapist responds with reframing and normalization around complexity, gradual change, and emotional impact.

These topics appear in the transcript well before the final minutes window, especially in the first minutes of the session. They should not disappear from the final report.

### Why this is serious

For a therapy/minutes product, a technically valid JSON payload is not enough. The output must preserve the relevant clinical arc of the conversation.

In this case, the generated minutes could mislead the user into thinking the session was mainly about closing logistics and platform/cost discussion, while the transcript contains broader emotional, professional, and personal context.

This is worse than an explicit generation failure because it looks successful:

- status is `ready`;
- provider is correct;
- webhook is delivered;
- UI can show a minute;
- but the content is incomplete and biased toward the end of the session.

## Additional Short Sessions

Two short production sessions regenerated successfully:

### Session `4e351ae5-03ba-417a-a609-3b8a13110265`

- Transcript rows: `5`
- Transcript characters: `346`
- Minutes content characters: `1743`
- Status: `completed`
- Minutes: `ready`
- Provider: `openai`
- Webhook: `delivered`

The output is structurally coherent, but the session is too short to validate long-session quality.

### Session `b1837e16-1ac0-46a1-b68f-bab1e3a0f55c`

- Transcript rows: `4`
- Transcript characters: `362`
- Minutes content characters: `2028`
- Status: `completed`
- Minutes: `ready`
- Provider: `openai`
- Webhook: `delivered`

The output is also structurally coherent, but again this is not a meaningful benchmark for incremental summarization.

The defect is exposed by long multi-batch sessions.

## Relevant Runtime Logs

For session `77794a9f-640a-4348-a7c9-04e9b7e30554`, the service generated 10 incremental batches and then performed finalization:

- `Generating minutes batch 1/10`
- ...
- `Generating minutes batch 10/10`
- `Finalizing minutes after 10 incremental batches`
- `Minutes generated successfully for session 77794a9f-640a-4348-a7c9-04e9b7e30554`

Some late cloud calls timed out and retried:

- batch 8: first attempt timed out at the configured request timeout, retry succeeded;
- batch 10: multiple attempts timed out, later retry succeeded.

Despite these retries, the generation eventually completed. Therefore the issue is not simply "OpenRouter failed". The final output was successfully produced but semantically incomplete.

## Hypothesis

The likely failure is in the incremental state handoff or finalization prompt.

Possible causes:

1. The finalization pass may overweight the latest batch or recent state.
2. The compact state passed between batches may be too aggressively compressed.
3. Earlier `summary.phases`, themes, and clinically relevant observations may be overwritten instead of merged.
4. The final pass may be asked to "clean/finalize" but not explicitly required to preserve chronological coverage.
5. The parser/normalizer may accept a valid but under-covered JSON object without coverage checks.
6. Citation selection may be biased toward the latest transcript chunks, causing the model to anchor on the ending.

## Required Investigation

The team should inspect the full incremental pipeline for long sessions:

- batch prompt;
- compact state schema;
- merge strategy between old state and new chunk;
- finalization prompt;
- final JSON validation;
- citation selection;
- whether `summary.phases` can shrink to only the final chunk without failing validation.

The key question is not whether the model returned valid JSON. It did.

The key question is whether the final minutes preserve a representative and chronologically complete summary of all transcript chunks.

## Suggested Reproduction

Use a local/sanitized fixture modeled after the production shape:

- ~25 minute transcript;
- ~240 segments;
- ~15k-20k characters;
- early section with core personal/contextual material;
- middle section with therapeutic exchange;
- final section with logistics/closing comments.

Run it through incremental generation with enough chunking to produce at least 8-10 batches.

The test should fail if:

- `summary.phases` covers only the final part of the session;
- themes only reflect the last chunk;
- citations all come from the final minutes;
- clinically relevant early themes are absent;
- `next_steps` and `progress_issues` are structurally present but semantically empty/generic despite transcript evidence.

## Proposed Quality Guards

Add post-generation coverage checks before accepting a minute as `ready`.

Possible checks:

- chronological coverage: `summary.phases` should cover meaningful portions of early, middle, and late transcript windows for long sessions;
- citation distribution: citations should not all come from the final chunk unless the transcript is actually short;
- theme retention: final themes should include high-signal themes extracted from earlier batches;
- state preservation: finalization must not reduce a multi-phase intermediate state to one final phase unless explicitly justified;
- content ratio warning: very short final minutes for long transcripts should produce a warning or `quality_degraded` marker.

These checks do not need to block all outputs initially, but they should at least be logged and surfaced for review.

## Acceptance Criteria

- A long multi-batch session preserves relevant content from early, middle, and late transcript windows.
- Final `summary.phases` does not collapse to only the closing segment when the transcript contains substantial earlier content.
- Citations are distributed across the session timeline where relevant.
- The finalization pass is tested with a fixture where important early content must survive.
- A generated minute that is valid JSON but has poor chronological coverage is flagged.
- The implementation avoids masking this issue as a successful, fully reliable generation.

## Operational Impact

This issue directly affects trust in Aftertalk. Operators may see `ready` and assume the minute is clinically useful, while the content may be incomplete.

For production customers, this creates a high-risk failure mode:

- no visible backend error;
- no failed webhook;
- no invalid JSON;
- but a low-quality or misleading clinical document.

The expected fix is not project-specific. It belongs in Aftertalk core quality controls for incremental minutes generation.

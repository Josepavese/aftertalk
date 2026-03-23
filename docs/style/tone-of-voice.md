# Tone of Voice

This guide defines how Aftertalk should sound across README, wiki pages, SDK docs, release notes, and website copy.

## Target Reader

- Developers evaluating whether to adopt Aftertalk.
- Integrators implementing it in production systems.
- Maintainers debugging behavior under real constraints.

Write for a technical reader who wants fast signal, not storytelling.

## Voice Principles

- Direct: lead with what the component does.
- Technical: use correct terms, not metaphors.
- Concrete: include behavior, interfaces, and limits.
- Evidence-based: prefer claims that can be tested quickly.
- Calm confidence: no hype, no defensive language.

If a sentence could fit any generic SaaS homepage, rewrite or remove it.

## Communication Pattern

Use this default order for high-traffic docs (README, landing wiki pages):

1. One-line value proposition with architecture context.
2. Fast path (`install`, `configure`, `run`) in copy-paste form.
3. Security/privacy model in explicit terms.
4. Integration boundary (who holds API keys, who gets JWT, what is public/private).
5. Links to deep docs.

## Writing Rules

- Prefer active voice and short sentences.
- Name real components and interfaces (`WebRTC`, `STT`, `LLM`, `webhook`, `JWT`).
- State defaults and side effects explicitly.
- Separate stable facts from roadmap intent.
- When possible, attach measurable qualifiers (`in N lines`, `no persisted audio`, `HTTP 401`, `idempotent`).

## Claims: Allowed vs Not Allowed

Allowed:

- "Aftertalk captures WebRTC audio server-side and produces structured minutes."
- "No raw audio is persisted; webhook delivery is retryable."
- "Browser clients use short-lived JWT tokens; API keys stay on the backend."

Not allowed:

- "Revolutionary AI-powered meeting intelligence platform."
- "Seamlessly integrates with your existing workflow."
- "Enterprise-grade privacy-first solution."

## Vocabulary

Prefer:

- "session minutes" over "insights"
- "structured output" over "magic"
- "provider" over "engine"
- "retry policy" over "self-healing"
- "public endpoint" / "protected endpoint" over "secure by default" (unless explained)

## Documentation-Specific Guidance

README:

- Optimize for first 30 seconds.
- Keep only decision-critical details.

Wiki/API docs:

- Favor completeness and unambiguous behavior.
- Include request/response shape and error modes.

Release notes:

- Start with user-visible change.
- Mention migration impact and compatibility.

## Language

- Primary language: English.
- Keep terminology consistent across README, wiki, and SDK docs.

## Reference Repositories

Patterns in this guide were benchmarked against communication style used in:

- https://github.com/openai/openai-python
- https://github.com/ollama/ollama
- https://github.com/fastapi/fastapi
- https://github.com/hashicorp/terraform
- https://github.com/supabase/supabase

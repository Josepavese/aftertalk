# Project Philosophy

## Domain-Agnostic Core for Conversation Analysis and Minutes Generation

Conceptual document for **architects, developers and technical stakeholders**.

---

## 1. Vision

**Aftertalk** is a **domain-agnostic technology core**, designed to:

* analyze voice conversations,
* extract a structured textual representation,
* produce a meaningful summary (minutes) to support a human operator.

The core is **not tied** to a specific domain (psychology, medicine, coaching, business); it is intended as a **reusable engine**, adaptable through configuration and application-layer integration.

The initial integration is in the context of *mondopsicologi*, which is **one concrete use case**, not the boundary of the core.

---

## 2. Guiding principle: agnosticism

### 2.1 What "agnostic" means

The Aftertalk core:

* has no knowledge of clinical domains,
* applies no domain-specific rules,
* makes no semantic decisions bound to a particular context,
* does not expose a final UI to business users.

The core only provides:

* structured audio acquisition,
* transcription with verified roles,
* consistent timeline,
* configurable summarization pipeline.

Every **domain interpretation** happens *downstream*, in the application layer.

---

## 3. Core / Application Separation

### 3.1 Core (Aftertalk Engine)

Responsibilities:

* conversational session management
* WebRTC audio ingestion
* server-side timestamping
* transcription normalization
* STT orchestration
* generic LLM pipeline
* neutral structured output

The core:

* has no UI dependencies
* knows nothing about "doctor", "patient", "client", "coach" (they are **abstract roles**)
* implements no clinical policies

---

### 3.2 Application Layer (e.g. Mondopsicologi)

Responsibilities:

* role mapping (e.g. doctor/patient)
* legal validation and consent
* prompt specialization
* final UI/UX
* retention policies
* governance and audit

The application layer **adapts** the core without modifying it.

---

## 4. Design for Reuse

The core must be designed from day one as:

* an **autonomous package**
* with clear boundaries
* stable APIs
* declarative configuration

Goals:

* future dedicated Git repository
* independent versioning
* usable in other projects without forking

---

## 5. Design for Extension (not modification)

New use cases must be handled via:

* configuration
* adapters
* plugins
* external prompts

Not via:

* domain `if/else` branches
* application branching inside the core
* hardcoded vertical logic

---

## 6. Semantic neutrality

The core:

* **does not formulate diagnoses**
* **does not interpret emotions**
* **does not draw conclusions**

It only produces:

* reorganization of expressed content
* faithful summaries
* temporal references

Interpretive responsibility remains **human**.

---

## 7. Human-in-the-loop by design

Aftertalk does not replace the professional.

The system is designed to:

* reduce cognitive load
* increase traceability
* improve follow-up quality

The minutes are:

* always editable
* never final
* never "absolute truth"

---

## 8. Future evolution

Thanks to core/application separation, Aftertalk can evolve towards:

* coaching and mentoring
* professional consulting
* business meetings
* training
* advanced customer support

Without modifying the core.

---

## 9. Development philosophy

* small, composable services
* API-first
* explicit over implicit
* configuration over code
* secure by default
* privacy-first

---

## 10. Summary

Aftertalk is:

* a **conversation understanding engine**
* designed to be **reusable, neutral and autonomous**
* adaptable to real use cases without losing architectural integrity

The *mondopsicologi* case is the first user, not the owner of the core.

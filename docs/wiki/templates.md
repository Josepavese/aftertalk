# Templates

Templates define the structure of meeting minutes — what roles participate, and what sections the AI generates.

## Built-in Templates

Two templates are bundled with the server (from `config.DefaultTemplates()`):

### `therapy` — Psychotherapy session

Roles: `therapist`, `patient`

| Section key | Type | Description |
|---|---|---|
| `themes` | `string_list` | Main themes of the session |
| `contents_reported` | `content_items` | Patient-reported content with timestamps |
| `professional_interventions` | `content_items` | Therapist interventions with timestamps |
| `progress_issues` | `progress` | Progress and issues |
| `next_steps` | `string_list` | Agreed next steps |

### `consulting` — Business consulting session

Roles: `consultant`, `client`

| Section key | Type | Description |
|---|---|---|
| `topics_discussed` | `string_list` | Main topics discussed |
| `key_points` | `content_items` | Key discussion points with timestamps |
| `decisions` | `string_list` | Decisions made |
| `action_items` | `content_items` | Action items with responsible party |
| `next_meeting` | `string_list` | Next meeting agenda |

---

## Section Types

Each section has a `type` that determines the JSON shape in the minutes response:

**`string_list`** — simple list of strings:
```json
["Item one", "Item two", "Item three"]
```

**`content_items`** — list of objects with text and timestamp:
```json
[
  {"text": "Patient reports difficulty sleeping", "timestamp": 1200},
  {"text": "Mentions anxiety at work", "timestamp": 3450}
]
```

**`progress`** — object with two sub-lists:
```json
{
  "progress": ["Sleep has improved"],
  "issues": ["Still struggling with work relationships"]
}
```

---

## Custom Templates

Add custom templates in your config file:

```yaml
templates:
  - id: medical-intake
    name: Medical Intake
    description: General practitioner intake assessment
    roles:
      - key: doctor
        label: Doctor
      - key: patient
        label: Patient
    sections:
      - key: chief_complaint
        label: Chief Complaint
        description: Main reason for the visit, as reported by the patient
        type: string_list
      - key: medical_history
        label: Medical History
        description: Relevant past medical history mentioned in the conversation
        type: content_items
      - key: assessment_plan
        label: Assessment and Plan
        description: Doctor's assessment and treatment plan
        type: string_list
```

Custom templates are merged with built-in templates. If you define a template with an existing ID (e.g. `therapy`), it overrides the built-in.

---

## Using Templates via API

### Specify on session creation

```bash
curl -X POST http://localhost:8080/v1/sessions \
  -H "X-API-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "participant_count": 2,
    "template_id": "medical-intake",
    "participants": [
      {"user_id": "dr-smith", "role": "doctor"},
      {"user_id": "patient-42", "role": "patient"}
    ]
  }'
```

If `template_id` is omitted, the server uses the default template (`therapy` by default, or the first configured template).

### Get available templates

```bash
curl -H "X-API-Key: $KEY" http://localhost:8080/v1/config
# → {"templates":[...],"default_template_id":"therapy"}
```

### Minutes structure follows the template

The `sections` object in a minutes response mirrors the template's section keys. The LLM output is validated and parsed per the template schema — if a section is missing from the LLM response, it defaults to an empty value of the correct type.

---

## LLM Prompt Generation

The prompt sent to the LLM is generated dynamically from the template. It includes:

1. **Role definitions** — which roles participated and their labels
2. **Section schema** — for each section: key, label, description, and expected JSON type
3. **Language rule** — detect the transcript language and output the minutes in the same language
4. **JSON schema example** — shows the expected output structure

This means adding a new template with well-described section `description` fields is sufficient to get accurate minutes — no code changes needed.

---

## Changing the Default Template

Set via environment variable or config:

```bash
# via env
AFTERTALK_LLM_DEFAULT_TEMPLATE_ID=consulting

# via config.yaml
llm:
  default_template_id: consulting
```

The default template is used when `template_id` is omitted from session creation.

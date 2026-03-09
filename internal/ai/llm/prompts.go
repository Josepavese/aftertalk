package llm

import (
	"fmt"
	"strings"
)

func GenerateMinutesPrompt(transcriptionText string, roles []string) string {
	return fmt.Sprintf(`Analyze the following conversation transcript and generate structured meeting minutes in JSON format.

PARTICIPANT ROLES:
- %s

TRANSCRIPT:
%s

Generate a JSON response with the following structure:
{
  "themes": ["main theme 1", "main theme 2", ...],
  "contents_reported": [
    {"text": "content reported by participants", "timestamp": 0}
  ],
  "professional_interventions": [
    {"text": "professional's intervention", "timestamp": 0}
  ],
  "progress_issues": {
    "progress": ["progress item 1", "progress item 2"],
    "issues": ["issue 1", "issue 2"]
  },
  "next_steps": ["next step 1", "next step 2"],
  "citations": [
    {"timestamp_ms": 0, "text": "cited text", "role": "role_name"}
  ]
}

STRICT RULES — MUST FOLLOW:
- NEVER invent, fabricate, or assume any content not present in the TRANSCRIPT above.
- If the transcript is empty or too short, ALL arrays must be empty and ALL strings must be empty — do NOT generate placeholder content.
- Every citation must be a verbatim quote from the transcript; if there are no quotes, citations must be an empty array.
- Do NOT make diagnoses or clinical assessments.
- Maintain neutral, factual tone.
- Use the same language as the transcript.

OUTPUT FORMAT:
- themes: list of main topics explicitly discussed (empty array if transcript is empty)
- contents_reported: key points from the transcript with their timestamp in ms
- professional_interventions: interventions by the professional role only
- progress_issues.progress: progress items mentioned
- progress_issues.issues: issues or problems mentioned
- next_steps: action items explicitly stated
- citations: verbatim quotes with timestamp_ms and role (empty array if no quotes)

Respond ONLY with valid JSON, no extra text.`, formatRoles(roles), transcriptionText)
}

func formatRoles(roles []string) string {
	if len(roles) == 0 {
		return "Unknown roles"
	}
	if len(roles) == 1 {
		return roles[0]
	}
	return strings.Join(roles[:len(roles)-1], ", ") + " and " + roles[len(roles)-1]
}

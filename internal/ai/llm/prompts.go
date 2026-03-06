package llm

import "fmt"

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

REQUIREMENTS:
1. themes: 3-5 main themes discussed
2. contents_reported: key points reported by participants
3. professional_interventions: important interventions by the professional
4. progress_issues: separate progress and issues identified
5. next_steps: actionable next steps
6. citations: at least 5 timestamped citations with exact quotes

IMPORTANT:
- Do NOT make diagnoses or clinical assessments
- Report only what was explicitly stated in the conversation
- Maintain neutral, factual tone
- Include timestamp in milliseconds for each citation
- Quote exact words from the transcript for citations

Respond ONLY with valid JSON.`, formatRoles(roles), transcriptionText)
}

func formatRoles(roles []string) string {
	if len(roles) == 0 {
		return "Unknown roles"
	}
	if len(roles) == 1 {
		return roles[0]
	}
	return roles[0] + " and " + roles[1]
}

# 19 — Installer: YAML Template Incompleto per LLM Cloud Providers

## Problema

`config_write.go` genera `aftertalk.yaml` con la sezione LLM parziale. Solo per Ollama
vengono scritte le sezioni complete (base_url + model). Per i provider cloud (OpenAI,
Anthropic, Azure) il template scrive solo `LLM_API_KEY` → `openai.api_key`:

```go
// config_write.go:47-49 — attuale
{{ range $k,$v := .LLMConfig }}{{ if eq $k "LLM_API_KEY" }}  openai:
    api_key: "{{ $v }}"
{{ end }}{{ end }}
```

Questo significa:
- Anthropic: nessuna sezione `anthropic:` nel YAML → app legge solo env vars
- Azure: nessuna sezione `azure:` con `endpoint:` + `deployment:` → incompleto
- OpenAI: solo `api_key`, manca `model:` nel YAML

Funziona perché le chiavi vengono anche scritte nell'env file (`/etc/aftertalk/aftertalk.env`),
ma il YAML risultante è fuorviante e incompleto per chi lo legge manualmente.

---

## Modifiche richieste

### `cmd/installer/steps/config_write.go` — Template LLM completo

Sostituire il blocco `llm:` con gestione provider-specifica:

```yaml
llm:
  provider: "{{ .LLMProvider }}"
{{ if eq .LLMProvider "ollama" }}  ollama:
    base_url: "{{ .OllamaURL }}"
    model:    "{{ .OllamaModel }}"
{{ end }}{{ if eq .LLMProvider "openai" }}  openai:
    api_key: "{{ index .LLMConfig "LLM_API_KEY" }}"
    model:   "{{ index .LLMConfig "LLM_MODEL" }}"
{{ end }}{{ if eq .LLMProvider "anthropic" }}  anthropic:
    api_key: "{{ index .LLMConfig "LLM_API_KEY" }}"
    model:   "{{ index .LLMConfig "LLM_MODEL" }}"
{{ end }}{{ if eq .LLMProvider "azure" }}  azure:
    api_key:  "{{ index .LLMConfig "LLM_API_KEY" }}"
    endpoint: "{{ index .LLMConfig "AZURE_OPENAI_ENDPOINT" }}"
    model:    "{{ index .LLMConfig "LLM_MODEL" }}"
{{ end }}
```

### `cmd/installer/steps/config_write.go` — Template STT cloud completo

Analogamente per STT, aggiungere sezioni per Google, AWS, Azure:

```yaml
stt:
  provider: "{{ .STTProvider }}"
{{ if eq .STTProvider "whisper-local" }}  whisper_local:
    url: "{{ .WhisperURL }}"
{{ end }}{{ if eq .STTProvider "google" }}  google:
    credentials_file: "{{ index .STTConfig "GOOGLE_APPLICATION_CREDENTIALS" }}"
{{ end }}{{ if eq .STTProvider "aws" }}  aws:
    access_key_id:     "{{ index .STTConfig "AWS_ACCESS_KEY_ID" }}"
    secret_access_key: "{{ index .STTConfig "AWS_SECRET_ACCESS_KEY" }}"
    region:            "{{ index .STTConfig "AWS_REGION" }}"
{{ end }}{{ if eq .STTProvider "azure" }}  azure:
    key:    "{{ index .STTConfig "AZURE_SPEECH_KEY" }}"
    region: "{{ index .STTConfig "AZURE_SPEECH_REGION" }}"
{{ end }}
```

**Attenzione**: Le chiavi segrete finiscono nel YAML (permessi 0640, owner = service user).
Questo è accettabile perché:
- Il file è già protetto da permessi stretti
- L'alternativa (solo env file) è più fragile per la leggibilità
- L'env file rimane come override per ambienti containerizzati

### `cmd/installer/config/config.go` — Aggiungere LLMModel a InstallConfig

Il modello LLM per OpenAI/Anthropic viene chiesto nel prompt e salvato in `LLMConfig["LLM_MODEL"]`
ma non è accessibile direttamente come campo tipizzato. Considerare:

```go
// In InstallConfig:
LLMModel string // modello selezionato per provider cloud
```

Oppure lasciare in `LLMConfig` e accedere via template `index`.

---

## Impatto

- YAML generato è auto-documentante e completo per tutti i provider
- Operatore può modificare il YAML post-install senza dover trovare l'env file
- Nessuna confusione su "perché la sezione anthropic: non c'è nel file?"

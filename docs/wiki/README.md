# Aftertalk Wiki

Documentazione operativa verificata sul codice sorgente.

## Indice

| Pagina | Contenuto |
|---|---|
| [installation.md](installation.md) | Requisiti, installer, primo avvio |
| [configuration.md](configuration.md) | Tutti i parametri di configurazione con default reali |
| [rest-api.md](rest-api.md) | Ogni endpoint con esempi curl completi |
| [sdk.md](sdk.md) | TypeScript SDK: quickstart, WebRTC, polling |
| [webhook.md](webhook.md) | Push vs notify_pull, verifica HMAC, esempi |
| [templates.md](templates.md) | Template di sessione: built-in e custom |
| [architecture.md](architecture.md) | Flussi interni, pipeline audio→minuta |

---

## In 30 secondi

```bash
# Installa
curl -fsSL https://raw.githubusercontent.com/flowup/aftertalk/master/scripts/install.sh | bash

# Configura (copia il template)
cp .env.example .env && nano .env   # cambia JWT_SECRET, API_KEY, LLM_PROVIDER

# Avvia
./bin/aftertalk
```

La UI demo è disponibile su `http://localhost:8080` se `AFTERTALK_DEMO_ENABLED=true`.

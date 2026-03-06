# Protocollo di Sviluppo Sicuro

## Regole Fondamentali

### 1. Build Sempre Prima di Committare
```bash
cd sdk && npm run build
cd test-platform && npm run build
go build -o bin/aftertalk ./cmd/aftertalk
```

### 2. Commit Dopo Ogni Modifica
- Mai piu di 5 file senza commit
- Sempre push dopo commit

### 3. Branch di Lavoro
- Mai lavorare direttamente su master
- Usare feature/* o fix/*

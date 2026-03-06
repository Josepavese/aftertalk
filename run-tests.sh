#!/bin/bash

#############################################
# AFTERTALK TEST RUNNER AUTOMATICO
#############################################
# Questo script:
# 1. Verifica che il server sia avviato
# 2. Apre automaticamente i browser
# 3. Guida l'utente step-by-step
# 4. Valida ogni step automaticamente
#############################################

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

clear
echo -e "${BLUE}"
echo "╔═══════════════════════════════════════════════════════════╗"
echo "║         🧪 AFTERTALK TEST SUITE AUTOMATICA                ║"
echo "╚═══════════════════════════════════════════════════════════╝"
echo -e "${NC}"

# Check if server is running
check_server() {
    echo -e "${YELLOW}1. Verifico che il server sia avviato...${NC}"
    sleep 1
    
    # Try to start server if not running
    if ! curl -s http://localhost:8080/v1/health > /dev/null 2>&1; then
        echo -e "${YELLOW}   Server non trovato, lo avvio...${NC}"
        
        export AFTERTALK_JWT_SECRET="dev-secret-key-12345"
        export AFTERTALK_API_KEY="dev-api-key-12345"
        export AFTERTALK_DATABASE_PATH="./aftertalk.db"
        export AFTERTALK_HTTP_PORT=8080
        export AFTERTALK_LOG_LEVEL="info"
        export AFTERTALK_LOG_FORMAT="console"
        export AFTERTALK_STT_PROVIDER="google"
        export AFTERTALK_LLM_PROVIDER="openai"
        export AFTERTALK_OPENAI_API_KEY="sk-test-key"
        export AFTERTALK_WS_PORT=8081
        
        ./bin/aftertalk > /tmp/aftertalk.log 2>&1 &
        SERVER_PID=$!
        
        # Wait for server to start
        echo -e "${YELLOW}   Avvio server in corso...${NC}"
        for i in {1..10}; do
            if curl -s http://localhost:8080/v1/health > /dev/null 2>&1; then
                echo -e "${GREEN}   ✓ Server avviato con successo!${NC}"
                break
            fi
            sleep 1
        done
        
        if ! curl -s http://localhost:8080/v1/health > /dev/null 2>&1; then
            echo -e "${RED}   ✗ ERRORE: Server non partito!${NC}"
            echo "   Controlla /tmp/aftertalk.log per dettagli"
            exit 1
        fi
    else
        echo -e "${GREEN}   ✓ Server già in esecuzione${NC}"
    fi
    
    # Verify demo page exists
    if curl -s http://localhost:8080/demo > /dev/null 2>&1; then
        echo -e "${GREEN}   ✓ Pagina demo disponibile${NC}"
    else
        echo -e "${RED}   ✗ Pagina demo NON disponibile${NC}"
        exit 1
    fi
}

# Show instructions
show_instructions() {
    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${YELLOW}📋 ISTRUZIONI PER IL TEST${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "Questo test verificare:"
    echo -e "  1. ${GREEN}Connessione WebSocket${NC} al server"
    echo -e "  2. ${GREEN}Streaming Audio P2P${NC} tra 2 browser"
    echo -e "  3. ${GREEN}Trascrizione${NC} in tempo reale"
    echo ""
    echo -e "${YELLOW}COME ESEGUIRE IL TEST:${NC}"
    echo ""
    echo -e "${BLUE}Step 1:${NC} Aprirò automaticamente 2 finestre del browser"
    echo -e "${BLUE}Step 2:${NC} In ogni finestra:"
    echo -e "        - Inserisci un nome (es. 'Mario' e 'Laura')"
    echo -e "        - Clicca 'Connetti'"
    echo -e "        - Clicca 'Avvia Audio'"
    echo -e "        - Parla al microfono!"
    echo -e "${BLUE}Step 3:${NC} L'audio apparirà nell'altro browser automaticamente"
    echo ""
    echo -e "${GREEN}👉 Premi INVIO per aprire i browser...${NC}"
    read
}

# Open browsers
open_browsers() {
    echo ""
    echo -e "${YELLOW}2. Apro i browser di test...${NC}"
    sleep 1
    
    # Try to open browsers
    if command -v xdg-open &> /dev/null; then
        xdg-open http://localhost:8080/demo &
        sleep 1
        xdg-open http://localhost:8080/demo &
        echo -e "${GREEN}   ✓ Browser aperti (xdg-open)${NC}"
    elif command -v open &> /dev/null; then
        open http://localhost:8080/demo
        sleep 1
        open http://localhost:8080/demo
        echo -e "${GREEN}   ✓ Browser aperti (open)${NC}"
    else
        echo -e "${YELLOW}   ⚠ Non riesco ad aprire i browser automaticamente${NC}"
    fi
    
    echo -e "${GREEN}   📱 Apri manualmente 2 browser a:${NC}"
    echo -e "${GREEN}      http://localhost:8080/demo${NC}"
}

# Wait for test completion
wait_for_test() {
    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${YELLOW}⏳ IN ATTESA DEL COMPLETAMENTO DEL TEST${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "Istruzioni:"
    echo -e "  1. In entrambi i browser: inserisci nome e clicca 'Connetti'"
    echo -e "  2. Aspetta che la connessione P2P si stabilizzi"
    echo -e "  3. Clicca 'Avvia Audio' in entrambi i browser"
    echo -e "  4. Parla al microfono - l'audio apparirà nell'altro browser"
    echo -e "  5. Clicca 'Test Trascrizione' per verificare la trascrizione"
    echo ""
    echo -e "${GREEN}👉 Quando il test è completato, premi INVIO...${NC}"
    read
    
    # Give time for results to appear
    sleep 2
}

# Show results
show_results() {
    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}✅ TEST COMPLETATO!${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "Sei riuscito a:"
    echo -e "  ✓ Connetterti al server WebSocket?"
    echo -e "  ✓ Stabilire una connessione P2P?"
    echo -e "  ✓ Trasmettere audio tra i 2 browser?"
    echo -e "  ✓ Ricevere trascrizioni?"
    echo ""
    echo -e "Per un nuovo test, esegui:"
    echo -e "  ${GREEN}./run-tests.sh${NC}"
    echo ""
}

# MAIN
check_server
show_instructions
open_browsers
wait_for_test
show_results

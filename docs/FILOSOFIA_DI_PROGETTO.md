# Filosofia di Progetto

## Core Agnostico per Analisi e Minuta di Conversazioni

Documento concettuale destinato a **progettisti, sviluppatori e stakeholder tecnici**.

---

## 1. Visione

Il progetto **Aftertalk** nasce come **core tecnologico agnostico**, progettato per:

* analizzare conversazioni vocali,
* estrarne una rappresentazione testuale strutturata,
* produrre una sintesi di senso (minuta) a supporto di un operatore umano.

Il core **non è legato** a un dominio specifico (psicologia, medicina, coaching, business), ma è pensato come **motore riutilizzabile**, adattabile tramite configurazione e integrazione applicativa.

L’integrazione iniziale avverrà nel contesto *mondopsicologi*, che rappresenta **un caso applicativo concreto**, non il perimetro del core.

---

## 2. Principio guida: agnosticità

### 2.1 Cosa significa “agnostico”

Il core Aftertalk:

* non conosce il dominio clinico,
* non applica regole specifiche di settore,
* non prende decisioni semantiche vincolate a un contesto,
* non espone UI finale all’utente business.

Il core si limita a fornire:

* acquisizione audio strutturata,
* trascrizione con ruoli certi,
* timeline coerente,
* pipeline di sintesi configurabile.

Ogni interpretazione **di dominio** avviene *a valle*, nel livello applicativo.

---

## 3. Separazione Core / Applicazione

### 3.1 Core (Aftertalk Engine)

Responsabilità:

* gestione sessioni conversazionali
* ingestione audio WebRTC
* timestamping server-side
* normalizzazione trascrizioni
* orchestrazione STT
* pipeline LLM generica
* output strutturato neutro

Il core:

* non ha dipendenze UI
* non conosce “medico”, “paziente”, “cliente”, “coach” (sono **ruoli astratti**)
* non implementa policy cliniche

---

### 3.2 Layer Applicativo (es. Mondopsicologi)

Responsabilità:

* mapping ruoli (es. doctor/patient)
* validazione legale e consenso
* prompt specialization
* UI/UX finale
* policy di retention
* governance e audit

Il layer applicativo **adatta** il core senza modificarlo.

---

## 4. Design for Reuse

Il core deve essere progettato fin dall’inizio come:

* **pacchetto autonomo**
* con confini chiari
* API stabili
* configurazione dichiarativa

Obiettivo:

* futuro repository Git dedicato
* versioning indipendente
* possibilità di utilizzo in altri progetti senza fork

---

## 5. Design for Extension (non per modifica)

Nuovi casi d’uso devono essere gestiti tramite:

* configurazione
* adapter
* plugin
* prompt esterni

Non tramite:

* `if/else` di dominio
* branching applicativo nel core
* hardcoding di logiche verticali

---

## 6. Neutralità semantica

Il core:

* **non formula diagnosi**
* **non interpreta emozioni**
* **non trae conclusioni**

Produce solo:

* riorganizzazione del contenuto espresso
* sintesi fedele
* riferimenti temporali

La responsabilità interpretativa resta **umana**.

---

## 7. Human-in-the-loop by design

Aftertalk non sostituisce il professionista.

Il sistema è progettato per:

* ridurre il carico cognitivo
* aumentare la tracciabilità
* migliorare la qualità del follow-up

La minuta è:

* sempre modificabile
* mai definitiva
* mai “verità assoluta”

---

## 8. Evoluzione futura

Grazie alla separazione core/applicazione, Aftertalk potrà evolvere verso:

* coaching e mentoring
* consulenza professionale
* meeting aziendali
* formazione
* customer support avanzato

Senza modificare il core.

---

## 9. Filosofia di sviluppo

* small, composable services
* API-first
* explicit over implicit
* configuration over code
* secure by default
* privacy-first

---

## 10. Sintesi

Aftertalk è:

* un **motore di comprensione delle conversazioni**
* progettato per essere **riutilizzabile, neutro e autonomo**
* adattabile a casi reali senza perdere integrità architetturale

Il caso *mondopsicologi* è il primo utilizzatore, non il proprietario del core.

---

Fine documento di filosofia.

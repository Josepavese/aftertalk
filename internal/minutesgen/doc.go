// Package minutesgen contains the isolated minutes-generation orchestration
// layer. It deliberately has no database, session, HTTP, or webhook ownership.
//
// The core contract is Orchestrator. DefaultOrchestrator wires a small
// deterministic pipeline:
//
//	transcript chunking -> prompt building -> LLM runner/repair -> reducer
//	-> finalization -> quality guard
//
// Integrations can replace PromptBuilder, Runner, Reducer, QualityGuard, or the
// whole Orchestrator to test another backend without changing minutes.Service.
package minutesgen

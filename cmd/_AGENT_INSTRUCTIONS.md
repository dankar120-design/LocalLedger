# 🤖 AGENT INSTRUCTIONS: CMD

Denna mapp innehåller Go-applikationens entrypoints (enligt standard Go layout).

1. **Minimal logik:** Filer i `cmd/localledger/` ska enbart hantera initialisering: läsa miljövariabler, starta databasanslutningen, starta HTTP-servern och hantera graceful shutdown.
2. **Ingen affärslogik:** Skriv absolut ingen affärslogik, BFL-validering eller routing direkt i `main.go`. Detta delegeras till `internal/`.

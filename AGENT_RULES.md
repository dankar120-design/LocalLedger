# 🤖 AGENT RULES (GLOBAL)

1. **Service-lager:** All affärslogik (moms, BFL-validering, balans, sekvenser) sker i `internal/ledger`. `internal/db` är strikt dum.
2. **Säkerhet:** Servern MÅSTE bindas till `127.0.0.1`. Aldrig `0.0.0.0`.
3. **BFL (Immutability):** Databasen och Go-koden får aldrig tillåta UPDATE eller DELETE på en bokförd verifikation. Rättelser sker via nya transaktioner.
4. **Data:** Inga floats för valuta. Endast `int64` (ören).
5. **Kontext:** Läs ALLTID `ARCHITECTURE.md` och relevanta PRD:er i `docs/` innan kodning.

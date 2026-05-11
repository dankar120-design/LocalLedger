# 🤖 AGENT INSTRUCTIONS: INTERNAL

Detta är systemets backend. All kod här är privat för Go-applikationen.

## Strikta Arkitektoniska Regler
1. **`ledger/` (Domänmotor):** Detta är hjärnan. All affärslogik, validering (Debet=Kredit), sekvenshantering för verifikationsnummer och momsberäkning sker BARA här.
2. **`db/` (Databas):** Detta lager är "dumt". Det utför endast C.R.U.D-operationer mot SQLite. `db/` får ALDRIG innehålla affärslogik eller validera summor.
3. **Bokföringslagen (Immutability):** Databasen och koden får ALDRIG tillåta `DELETE` eller `UPDATE` på befintliga verifikationer eller transaktionsrader. Allt styrs med ACID-transaktioner med `ON DELETE RESTRICT`.
4. **Belopp:** Använd ALDRIG floats/decimals. Alla monetära värden hanteras som `int64` (ören).
5. **`api/` (API):** Alla endpoints returnerar strikt JSON.
6. **`sie/` (SIE-4 Export):** Följer den tekniska specifikationen. Lita på `ledger/` för att hämta korrekt och validerad data.

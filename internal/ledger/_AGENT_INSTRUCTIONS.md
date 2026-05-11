# 🤖 AGENT INSTRUCTIONS: LEDGER

Detta är hjärtat av applikationen. Här implementeras redovisningslogiken.

1. **Enda valideraren:** Ingenting får sparas i databasen om det inte passerat genom detta lager.
2. **Debet=Kredit:** Varje verifikation som registreras MÅSTE valideras så att `sum(debet) + sum(kredit) == 0`.
3. **Moms:** All beräkning för moms (25%, 12%, 6%) utförs här.
4. **Sekvensnummer:** Logik för att hämta och allokera nästa lediga verifikationsnummer utförs i detta lager, i tätt samarbete med databasens transaktionslåsning.
5. **Räkenskapsår:** All logik för att kontrollera vilket räkenskapsår en transaktion tillhör ligger här.

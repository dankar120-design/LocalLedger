# Master PRD (Product Requirements Document) - Phase 2 (V2.0)

> [!NOTE]
> Detta är det styrande dokumentet för LocalLedger Phase 2. Fas 1 (Append-only bokföring, Moms, OCR, WORM och SIE-4) är genomförd och i produktion.

## 1. Vision & Scope för Fas 2
Visionen för Fas 2 är att utöka LocalLedger från en passiv "Huvudbok" till en proaktiv **Ekonomimotor**, utan att kompromissa med den lokala, "anarkistiska" offline-först-filosofin. Fokuset ligger på att eliminera dubbelarbete genom inbyggd fakturering och framtidssäkra underlagsinsamlingen.

## 2. Nya Funktionella Krav (Phase 2)

### A. Inbyggd Fakturamodul (Native Invoicing)
- **Funktion:** Ett inbyggt gränssnitt för att skapa och skicka PDF-fakturor (genereras server-side via t.ex. `gofpdf`).
- **Livscykel & Kreditering:** Spårar status (Utkast, Skickad, Delvis Betald, Betald). En skickad faktura kan inte makuleras, utan måste krediteras via en formell *Kreditfaktura* (enligt BFL).
- **Nummerserie:** Fakturor har en egen obruten nummerserie med gap-handling, precis som verifikationer.
- **Zero Double-Entry (Dynamisk):** Modulen anropar `ledger.CreateEntry()`. Konteringen är dynamisk (frågar `internal/vat` om rätt momskonto beroende på momssats) och hanterar partiella inbetalningar/öresavrundning.
- **Kundregister (GDPR):** Kunders kontaktuppgifter sparas i en `customers`-tabell. Till skillnad från låsta verifikationer lyder denna tabell under GDPR och kunders personuppgifter ska kunna anonymiseras vid begäran.

### B. API-driven Inkorg (The Inbox Pattern)
- **Funktion:** En ny vy i GUI:t ("📥 Inkorg") som agerar som en väntsal för inkommande underlag.
- **Teknik (Staging-mapp):** Ett HTTP-API (`POST /api/inbox`) i Go-servern tar emot filer. FIlerna sparas fysiskt i en temporär staging-mapp (`workspace/inbox/`), medan endast filsökväg och metadata sparas i en databas-tabell (`inbox_items`). Inga tunga BLOBs i SQLite!
- **Framtidssäkring:** Detta möjliggör att framtida mobila WebApps kan pusha kvitton direkt in i LocalLedger över nätverket.

### C. AES-Krypterad Molnsynk (Semi-Online)
- **Funktion:** Möjlighet att aktivera AES-256 kryptering på den automatiska ZIP-backupen (nyckel härleds via PBKDF2 från ett användarvalt lösenord).
- **Google Drive Integration:** Användaren väljer en lokal mapp (t.ex. sin Google Drive-mapp) där backupen kontinuerligt dumpas.
- **BFL-Varning:** Gränssnittet måste innehålla en gul varningstext som förklarar att amerikanska molntjänster endast är *komplement* till fysiska EU-lagrade backuper enligt Bokföringslagen 7 kap 1§.

## 3. Explicit Non-Goals (Vad vi INTE bygger)
- **Ingen separat Faktura.exe:** Att bryta ut faktureringen i ett eget program är strikt förbjudet då det skapar en "Split Brain" för kundfordringar och saboterar den atomära backup-driften.
- **Ingen molndatabas:** Databasen ska ALDRIG synkas levande mot molnet (risk för SQLite-låsningar). Endast den stängda `.zip`-filen får synkas.
- **Ingen mapp-övervakning:** Vi använder inte `fsnotify` för att titta i Windows-mappar som en inkorg, då detta är för skört och osäkert. Inkorgen drivs stenhårt via API/Databas.

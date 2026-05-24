# LocalLedger - Systemarkitektur & Domänkarta

Detta dokument beskriver den övergripande systemarkitekturen för LocalLedger, anpassad för att strikt följa svensk Bokföringslag (BFL), Bokföringsnämndens normering (BFNAR 2013:2) och SIE-4-specifikationen.

## 🌳 Trädstruktur

```text
LocalLedger/
├── cmd/                # Entrypoints för Go-binären
│   └── localledger/    # Huvudapplikationen (Stateless). Injicerar Company Workspace via miljö/args.
├── docs/               # Dokumentation, PRD:er och specifikationer
│   └── research/       # Legala krav och standarder.
├── frontend/           # HTML/JS/CSS klientkod
│   ├── static/         # CSS, bilder, statiska JS-filer
│   └── views/          # HTML-mallar
├── internal/           # Privat Go-kod (Backend-logik)
│   ├── api/            # HTTP Handlers och routing (REST).
│   ├── db/             # Append-Only Event Storage & CQRS-Saldotabeller.
│   ├── inbox/          # API-driven Inkorg (Phase 2).
│   ├── invoice/        # Native Faktureringsmodul (Phase 2).
│   ├── ledger/         # DOMÄNMOTOR: Bokföringslogik, validering, migrations. Inkluderar Audit Trail.
│   ├── models/         # Go structs och domänmodeller.
│   ├── reports/        # PDF och Excel generering (Balans- och Resultaträkning).
│   ├── sie/            # Bidirektionell SIE-4 parser/encoder (UTF-8 ↔ CP437).
│   ├── storage/        # Pragmatisk WORM-lagring för digitala underlag (Hash + Readonly).
│   └── vat/            # Isolerad momsberäkning (K1/MVCA).

COMPANY_WORKSPACE_DIR/  # En portabel mapp per företag som användaren själv placerar.
├── ledger.db           # Databasfilen exklusivt för detta bolag (Single-Tenant).
└── underlag/           # WORM-säkrade kvitton (strikt isolerade från andra bolags data).

examples/
└── DemoFöretaget_AB/   # Pre-genererat Sandbox-Workspace. ledger.db har `is_sandbox=true` för att tillåta testning.
```

## ⚙️ Komponent-index & Ansvarsområden

| Komponent | Ansvarsområde & Legala Krav | Beroenden |
| :--- | :--- | :--- |
| `cmd/localledger` | **Applikationsstart (Stateless)**. Öppnar en `Company Workspace`-mapp som användaren valt. Tvingar appen att vara portabel/install-agnostisk. | `internal/*` |
| `internal/db` | **Append-Only Lagring & Light-CQRS**. Databaslagret för produktionsdata. Förbjuder `UPDATE` och `DELETE`. Hanterar materialiserade vyer i samma atomära transaktion. **Kräver PRAGMA journal_mode=WAL;** för konkurrent läsning. | `internal/models` |
| `internal/ledger` | **Hjärnan & Transaktionsägaren**. Orkestrerar affärslogik och schema-migreringar. Tvingar **YearLocks** och **PeriodLocks** (för inlämnad moms). Driver den lagstadgade **Audit Trail**-loggen över användaraktioner. | `internal/db`, `internal/vat`, `internal/models` |
| `internal/invoice`| **Fakturering (Phase 2)**. Hanterar fakturans livscykel. Anropar `ledger.CreateEntry(...)` för att generera verifikationer dynamiskt via `internal/vat` för att förhindra Split Brain. Begär Kreditfakturor vid makulering. Hanterar delbetalningar. | `internal/ledger`, `internal/db`, `internal/models` |
| `internal/inbox`  | **Databasdriven Inkorg (Phase 2)**. Väntsal för externa kvitton. Sparar inkommande filer i en lokal staging-mapp och lagrar enbart filsökväg och metadata i DB innan de WORM-låses av `storage`. Inga DB-blobs. | `internal/db`, `internal/storage` |
| `internal/vat` | **Skatte- & Momsregler**. En tjänst underställd `ledger`. Utför momsomföring och begär **PeriodLocks** på låsta kvartal. Skriver aldrig till DB själv. | `internal/models` |
| `internal/sie` | **Import/Export (SIE-4)**. Bygger och tolkar SIE-filer. Tvingar **CP437 (IBM PC8)** kodning enligt standard. Bidirektionell: Konverterar från UTF-8 till CP437 vid export, och tvärtom vid import (Typ 4I). | `internal/ledger`, `internal/models` |
| `internal/reports` | **Analys och Utskrift**. Genererar balansräkning, resultaträkning och kvittokopior till PDF/Excel. | `internal/ledger`, `internal/models` |
| `internal/storage` | **Kryptografisk Underlagshantering (WORM)**. Tar emot kvitton, beräknar en SHA-256 hash och lagrar filen lokalt med en OS-readonly-flagga inuti aktuell Company Workspace. | `internal/models`, OS fs |
| `internal/api` | **Klientgränssnitt**. Exponerar systemet för webbklienten (JSON REST API). Inga legala domänbeslut fattas här. Serverar även Dashboard-aggregeringar. | `internal/ledger`, `internal/sie`, `internal/storage`, `internal/reports`, `internal/invoice`, `internal/inbox` |
| `frontend/views` | **Användargränssnitt (Alpine.js)**. Tillhandahåller reaktivitet och UX-mönster (Dark Mode, Toasts, Modaler) via Vanilla CSS, isolerad från databaslagret. | `internal/api` |

## 🔄 Kritiska Logiska Flöden (BFL-Compliant)

1. **Registrering av Verifikation (Atomär Fil/DB-låsning):**
   - Klient laddar upp kvitto. `api` skickar till `storage` som sparar filen och returnerar en Hash (WORM).
   - `api` ber `ledger` boka konteringen med Hashen.
   - `ledger` startar en `DB Transaction` och sparar kontering, hash och ev. moms.
   - **Atomicitet:** Om `DB Transaction` misslyckas (rollback), raderas den uppladdade filen omedelbart för att förhindra orphana filer. `COMMIT`.

2. **Rättelse (Stornopost):**
   - Systemet tillåter *inte* redigering av befintlig post.
   - Om användare begär "Rätta verifikation X", skapar `ledger` en fullständig reversering (stornopost) av X i en ny verifikation (Y), följt av en ny korrekt verifikation (Z). Alla får nya datumstämplar.

3. **Momsomföring & Årsavslut (YearLocks):**
   - Vid årsavslut orkestrerar `ledger` överföringen av Utgående Balans (UB) till Ingående Balans (IB) för nästa år.
   - När bokslutet är signerat sätter `ledger` en tvingande `locked_at` tidsstämpel i `fiscal_years`-tabellen. Därefter avvisar backend alla inserts relaterade till det året.

4. **Gap Handling (Nummerserier):**
   - Vid eventuella krascher där en `AUTOINCREMENT` sekvens konsumerats utan att sparas (t.ex. DB Rollback), uppstår en lucka i verifikationsserien. `ledger` skannar serien och genererar automatiskt en "Makuleringspost" utan belopp för att förklara luckan för framtida revisorer.

5. **Backup, Export & BFL Molnsynk (The Backup Drift):**
   - Databas och digitala underlag sparas separat. För att undvika "Missing File"-fel vid återställning, tvingar systemet fram att en backup alltid använder `SQLite Backup API` för att atomärt kopiera databasen samtidigt som kvitto-mappen komprimeras till ett gemensamt Zip-arkiv.
   - **Phase 2 (AES-ZIP):** Arkivet kan valfritt AES-256 krypteras lokalt innan det exporteras till en "Dumb Cloud Folder" (t.ex. Google Drive). Denna molnsynk måste markeras som ett *komplement* då BFL 7 kap 1§ kräver lagring inom EU. All fakturahistorik åker automatiskt med i denna atomära export eftersom fakturorna lagras inuti LocalLedger.

6. **Invoice Lifecycle & Nummerserier (Anti Split-Brain):**
   - Fakturor har en egen obruten nummerserie (BFL-krav). De skapas som inbyggda dokument. De WORM-låses vid status "Skickad" och utlöser dynamisk kontering via `ledger.CreateEntry()`.
   - Vid inbetalning uppdateras fakturans status (delvis eller fullt) och utlöser inbetalningskontering via `ledger`.
   - Makulering av skickad faktura *kräver* att en Kreditfaktura utfärdas för att reversera originalet.

7. **GDPR vs Bokföringslagen (Arkitekturpolicy):**
   - Enligt BFL 7 kap måste bokföring sparas i 7 år, vilket trumfar GDPR:s "Rätt att bli glömd" för *låsta verifikationer*.
   - **Phase 2 Tillägg:** `customers`-tabellen innehåller personuppgifter (PII) och berörs av GDPR. Om en kund begär radering får kontaktuppgifterna i `customers` anonymiseras, men den historiska PDF-fakturan och konteringen är för evigt BFL-låsta. All gallring av underlag sker tidigast när 7-årsperioden löpt ut.

## 🤖 Agent-instruktioner (Hard Constraints för AI)

Följande instruktioner MÅSTE följas av alla kod-agenter som arbetar med LocalLedger:

1. **Ingen Extern Faktura-App:** Du FÅR INTE föreslå eller bygga faktureringen som ett separat program (t.ex. `Faktura.exe`). Det ska byggas som en native modul i `internal/invoice` med egna tabeller i SQLite. Målet är att förhindra "Split Brain" för kundfordringar.
2. **Inkorgen är Filbaserad:** Du FÅR INTE spara kvitton som `BLOB` i databasen. Inkorgen (via `POST /api/inbox`) ska spara filer i en fysisk `workspace/inbox/` mapp, och spara filsökvägen i `inbox_items`-tabellen. Du får inte heller använda `fsnotify` för att skanna mappen automatiskt.
3. **Inga Hårdkodade Konton:** Du FÅR INTE hårdkoda momskonton (`2611`, `2621`, etc.) i `internal/invoice`. Konteringen MÅSTE delegeras dynamiskt till `internal/vat` baserat på fakturans momsrad.
4. **Självförsörjande Backup:** Du behöver INTE bygga separata backup-script för fakturor. Eftersom fakturorna lever inuti `ledger.db` (och som PDF:er i underlagsmappen) täcks de redan 100% av det atomära ZIP-backup-verktyget.
5. **BFL Molnsynk-Disclaimer:** Om du bygger UI:t för AES-256 ZIP-export till Google Drive, SKA du inkludera en tydlig text om att det bryter mot BFL 7 kap 1§ om det används som *enda* backup-plats (eftersom servrarna kan ligga utanför EU). Lösenord måste härledas säkert (t.ex. PBKDF2).
6. **Ingen levande DB i molnet:** Du FÅR ALDRIG uppmana användaren att köra `ledger.db` direkt från en synkad molnmapp (p.g.a. fil-låsningar). Endast den exporterade `.zip`-filen får synkas till molnet.
7. **First Run Setup:** Du FÅR INTE gissa 'Documents' som defaultmapp (risk för OneDrive-låsning). Setup Mode triggas av saknad --workspace flagga och UI:t måste rekommendera C:\LocalLedger_Data.
8. **Server Heartbeat:** Du FÅR INTE bygga zombie-processer. MÅSTE använda `time.Since` ping-tracker för shutdown.


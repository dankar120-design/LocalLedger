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
| `internal/vat` | **Skatte- & Momsregler**. En tjänst underställd `ledger`. Utför momsomföring och begär **PeriodLocks** på låsta kvartal. Skriver aldrig till DB själv. | `internal/models` |
| `internal/sie` | **Import/Export (SIE-4)**. Bygger och tolkar SIE-filer. Tvingar **CP437 (IBM PC8)** kodning enligt standard. Bidirektionell: Konverterar från UTF-8 till CP437 vid export, och tvärtom vid import (Typ 4I). | `internal/ledger`, `internal/models` |
| `internal/reports` | **Analys och Utskrift**. Genererar balansräkning, resultaträkning och kvittokopior till PDF/Excel. | `internal/ledger`, `internal/models` |
| `internal/storage` | **Kryptografisk Underlagshantering (WORM)**. Tar emot kvitton, beräknar en SHA-256 hash och lagrar filen lokalt med en OS-readonly-flagga inuti aktuell Company Workspace. | `internal/models`, OS fs |
| `internal/api` | **Klientgränssnitt**. Exponerar systemet för webbklienten (JSON REST API). Inga legala domänbeslut fattas här. | `internal/ledger`, `internal/sie`, `internal/storage`, `internal/reports` |

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

5. **Backup & Export (The Backup Drift):**
   - Databas och digitala underlag sparas separat. För att undvika "Missing File"-fel vid återställning av osynkade backuper, tvingar systemet fram att en backup alltid använder `SQLite Backup API` för att atomärt kopiera databasen samtidigt som kvitto-mappen komprimeras till ett gemensamt Zip-arkiv.

6. **GDPR vs Bokföringslagen (Arkitekturpolicy):**
   - Enligt BFL 7 kap måste bokföring sparas i 7 år, vilket trumfar GDPR:s "Rätt att bli glömd". Applikationen är stenhårt Append-Only. Det är **arkitektoniskt förbjudet** för framtida utvecklare att implementera någon form av raderings- eller redigeringsfunktion (GDPR-redaction) på låsta verifikationer, då detta bryter mot grundvalarna i BFL. All gallring sker tidigast när 7-årsperioden löpt ut.

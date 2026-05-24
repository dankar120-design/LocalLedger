# PRD: Phase 3 - Rollout & Onboarding

## Mål
Göra LocalLedger redo för pilotanvändare. Målet är noll friktion vid onboarding, bombsäker dataintegritet vid uppdateringar och ett strikt avgränsat testprotokoll.

## Scope Boundaries (Ur Avgränsning)
Följande funktioner ingår **INTE** i Phase 3 och får inte implementeras för att undvika scope creep:
- Ingen egenutvecklad mobilapp eller lokal WiFi-server för OCR.
- Ingen AES-256 krypterad molnexport (vi exporterar vanliga .zip till valfri mapp).
- Inget kodsigneringscertifikat (Windows Defender-varningar accepteras och rundgås med dokumentation).
- Ingen automatisk uppdatering (`LocalLedger.exe` ersätter inte sig självt).

## Etapper (Slices)

### Slice 16: E2E Validation (Flash)
- **Problem:** O-validerad kodbas från Phase 2 SPA-migrering.
- **Krav:** En oberoende testkörning av den existerande kodbasen måste genomföras via Browser Subagent.
- **Protokoll:** Alla 12 testfall i `docs/Flash_E2E_Protocol.md` måste lysa grönt innan ny infrastruktur byggs.

### Slice 17: Server Lifecycle
- **Problem:** Zombie-processer, blinda fel, och riskabla uppdateringar.
- **Krav:**
  - **Heartbeat:** Frontend pingar `/api/ping`. Servern sparar tidsstämpel och kör en timer som dödar `os.Exit(0)` om `time.Since(lastPing) > 15s`.
  - **Loggfil:** `log.Println` styrs till `workspace/LocalLedger.log`.
  - **Pre-Migration Backup:** `VACUUM INTO` körs automatisk innan `migrations.go` uppgraderar schema.

### Slice 18: Setup Wizard & Sandbox
- **Problem:** Systemet kraschar om en Workspace saknas.
- **Krav:**
  - Om systemet startas utan `--workspace` argument, bootar det i Setup Mode och öppnar `localhost:8080/setup`.
  - **OneDrive-skydd:** Användaren måste explicit rekommenderas `C:\LocalLedger\` för att undvika fil-låsningar från OneDrive (vi gissar inte `Documents`).
  - **Auth:** Efter skapad mapp genererar systemet `.session_token` och ger användaren inloggningsuppgifterna.
  - **Sandbox:** Start med `--sandbox` flaggan packar upp en demo-mapp till `%TEMP%/LocalLedger_Sandbox_[GUID]`.

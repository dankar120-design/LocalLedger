# PROJECT PULSE: LocalLedger
*Generated: 2026-05-12 09:20:04*

## Technical Health
- **Ollama**: ONLINE (Local GPU available)
- **System RAM Usage (Shared/Integrated)**: 35799 / 64981 MB

## Git Context
### Status

```text
?? ledger.db ?? localledger.db ?? sandbox_e2e/check.go ?? sandbox_e2e/ledger.db-shm ?? sandbox_e2e/ledger.db-wal
```

### Recent Changes
```text
b98db27 feat: stabiliserat validering, state-synk och list-rendering i UI 805c177 fix(ui): Hotfix 3.2 - Inline Trollstav prompt and x-ref refactor 48ba677 fix(ui): Hotfix 3.1 - Scoped numpad navigation and secure Ctrl+Enter
```

## Project Blueprint
```text

Name                      Mode   LastWriteTime      
----                      ----   -------------      
bin                       d----- 2026-05-11 13:27:35
cmd                       d----- 2026-05-07 15:15:42
docs                      d----- 2026-05-07 14:40:02
examples                  d----- 2026-05-07 16:27:30
frontend                  d----- 2026-05-08 10:46:05
internal                  d----- 2026-05-08 12:00:17
sandbox_e2e               d----- 2026-05-11 21:22:03
scratch                   d----- 2026-05-11 10:06:43
ux_mockups                d----- 2026-05-11 10:41:01
.gitignore                -a---- 2026-05-07 14:12:53
AGENT_RULES.md            -a---- 2026-05-07 14:12:52
ARCHITECTURE.md           -a---- 2026-05-08 12:07:44
go.mod                    -a---- 2026-05-11 14:49:59
go.sum                    -a---- 2026-05-11 14:49:59
ledger.db                 -a---- 2026-05-11 21:21:07
localledger.db            -a---- 2026-05-11 19:24:25
localledger.exe           -a---- 2026-05-11 18:13:19
mockup_receipt.png        -a---- 2026-05-09 15:08:25
PROJECT_PULSE.md          -a---- 2026-05-11 08:39:43
PROJECT_PULSE_SURGICAL.md -a---- 2026-05-11 08:39:43
README.md                 -a---- 2026-05-07 14:15:55
server_error.txt          -a---- 2026-05-11 10:06:25
server_output.txt         -a---- 2026-05-11 10:06:25
_AGENT_INSTRUCTIONS.md    -a---- 2026-05-07 14:07:04
LocalLedger.exe           -a---- 2026-05-11 13:27:35
localledger               d----- 2026-05-07 14:12:54
sandbox_gen               d----- 2026-05-07 15:15:42
_AGENT_INSTRUCTIONS.md    -a---- 2026-05-07 14:07:10
research                  d----- 2026-05-11 08:58:55
Compliance_Spec.md        -a---- 2026-05-07 14:07:14
Master_PRD.md             -a---- 2026-05-10 21:00:15
system_documentation.md   -a---- 2026-05-07 14:46:15
_AGENT_INSTRUCTIONS.md    -a---- 2026-05-07 14:07:07
DemoForetaget_AB          d----- 2026-05-07 16:27:30
DemoFöretaget_AB          d----- 2026-05-07 16:19:39
static                    d----- 2026-05-08 09:37:41
views                     d----- 2026-05-10 14:55:08
embed.go                  -a---- 2026-05-08 10:46:05
_AGENT_INSTRUCTIONS.md    -a---- 2026-05-07 14:07:13
api                       d----- 2026-05-10 17:02:36
cli                       d----- 2026-05-08 08:58:16
ledger                    d----- 2026-05-11 15:35:57
models                    d----- 2026-05-08 12:00:17
_AGENT_INSTRUCTIONS.md    -a---- 2026-05-07 14:15:55
attachments               d----- 2026-05-11 10:06:25
.server_port              -a---- 2026-05-11 10:06:25
.session_token            -a---- 2026-05-11 10:06:25
check.go                  -a---- 2026-05-11 21:23:11
ledger.db                 -a---- 2026-05-11 10:06:49
ledger.db-shm             -a---- 2026-05-11 21:23:04
ledger.db-wal             -a---- 2026-05-11 21:22:03
check.go                  -a---- 2026-05-11 10:06:43
theme1_light.html         -a---- 2026-05-11 10:40:20
theme2_dark.html          -a---- 2026-05-11 10:40:42
theme3_enterprise.html    -a---- 2026-05-11 10:41:01



```

## Active TODOs
- [\docs\research\Bokföringslagen_ Digitala Systemkrav.md:208] [image1]: <data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAmwAAAIACAYAAAA/ozAEAACAAElEQVR4Xuy9d6AmRZX/Peof++5vZXD3NRF0...


## Architecture Sync
- **ARCHITECTURE.md**: Found (Last updated: 2026-05-08 12:07:44)

### 🟡 Slice 16: E2E Validation (Flash)
**Mål:** Verifiera befintlig kodbas innan ny infrastruktur byggs.

### 🟡 Slice 17: Server Lifecycle
**Mål:** Eliminera zombie-processer och säkra uppdateringar. Inkluderar Heartbeat, Loggfil och Pre-migration Backup.

### 🟡 Slice 18: Setup Wizard & Sandbox
**Mål:** Smärtfri onboarding för pilotanvändare utan OneDrive-kollisioner, samt isolerat övningsläge.

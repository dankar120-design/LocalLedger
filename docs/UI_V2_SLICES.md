# Implementeringsplan: UI V2 Slices

Detta dokument styr exekveringen av `PRD_UI_V2.md`. Arbetet är uppdelat i sekventiella slices där varje slice bygger på föregående för att säkerställa stabilitet och felhantering från grunden.

## 🛡️ Muskelanalys (Hybrid Delegation)
| Komponent | Typ | Delegeras till Lokal GPU? | Notering |
| :--- | :--- | :--- | :--- |
| Alpine.js Toast Component | Hjärna | NEJ | Infrastruktur som kräver exakt integration med befintliga `x-data`-scopes i `index.html`. Skrivs av Antigravity. |
| CSS Variabler (Dark Mode) | Muskler | JA | Repetitiv inläggning av CSS-tokens från `theme2_dark.html` till den befintliga applikationens styles. Bör köras via `local_agent_bridge`. |
| SVG Ikon-injektion | Muskler | JA | Rå HTML/SVG-inklistring i navbaren. Perfekt för lokal GPU. |
| Go Dashboard Endpoint | Hjärna | NEJ | Kräver strikt SQL/domänlogik-fokus för att räkna ut Årets Resultat korrekt från event-saldotabellen utan att bryta WORM. |

## Slices

### Slice 1: Globalt Toast-system
*   **Mål:** Skapa en infrastruktur för asynkron feedback (Krävs för de andra stegen).
*   **Filer:** `frontend/views/index.html` (lägga till `x-data="toastManager()"`), stilmall (animationer).

### Slice 2: Dark Mode Engine
*   **Mål:** Applicera design-tokens från `theme2_dark.html`.
*   **Filer:** Stilmallar (definiera `:root` och `.dark`), `frontend/views/index.html` (Alpine `x-data` för localStorage och Theme Toggle-knapp).

### Slice 3: Egna Ikoner i Navbar
*   **Mål:** Ersätta text i sidomenyn med premium-SVG-ikoner.
*   **Filer:** `frontend/views/index.html`.

### Slice 4: Dashboard Metrics
*   **Mål:** Hämta levande data från Go-backend till färgkodade UI-kort.
*   **Filer:** `internal/api/` (Ny handler), `internal/ledger/` (Ny DB-query för Intäkter/Kostnader), `frontend/views/index.html` (`x-init fetch`).

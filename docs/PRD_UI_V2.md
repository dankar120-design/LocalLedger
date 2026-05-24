# Product Requirements Document: LocalLedger UI V2 ("Salvage")

## Bakgrund
Efter en arkitektonisk utvärdering av prototypen "OpenCode" framkom det att även om backend-arkitekturen i OpenCode var fundamentalt osäker (förlitade sig på DELETE-anrop och saknade WORM-lagring), var frontend-designen mycket högkvalitativ. Syftet med UI V2 är att "plundra" de bästa UX-mönstren från OpenCode och applicera dem på LocalLedgers säkra Go/Alpine-arkitektur.

## Målsättning (Scope)
Att höja det upplevda värdet av LocalLedger genom att införa moderna UX-komponenter som ger omedelbar visuell feedback och en tydlig överblick, utan att kompromissa med den underliggande säkerheten eller införa tunga externa beroenden (inget Tailwind, inget React).

## Kravställning (Features)

### 1. Globalt Toast-system (Infrastruktur)
- Måste ersätta statiska felmeddelanden (`alert` eller in-line röda texter).
- Ska drivas av Alpine.js via en global event-lyssnare (t.ex. `@notify.window`).
- Måste stödja typerna `success`, `error` och `info`.
- Animationer (slide-in från kanten, fade-out efter 3-5 sekunder).

### 2. Dark Mode Engine
- Ska använda ren Vanilla CSS med CSS-variabler (design-tokens hämtade från `ux_mockups/theme2_dark.html`).
- State ska hanteras av Alpine.js och persisteras i `localStorage` (så att temat överlever sidladdningar).
- Navigationsbaren ska ha en tydlig ikon (Sol/Måne) för toggling.

### 3. Egna Ikoner
- Ersätta tråkiga text-länkar i navbaren med skarpa, enhetliga SVG-ikoner för att "höja UI-värdet".
- Ikonerna ska hantera Dark/Light mode färgskiftningar via CSS `stroke` eller `fill`.

### 4. Dashboard Metrics Cards
- Överst i UI:t (under header) ska det finnas färgkodade överstrykningskort.
- Måste visa: Kassa/Bank (Likviditet) och Årets Resultat.
- Drivs av en ny säker Go-endpoint (t.ex. `/api/dashboard`) som dynamiskt räknar ut summan av Intäktskonton (3000-3999) minus Kostnadskonton (4000-8999) filtrerat på innevarande räkenskapsår, utan att byta ut Append-Only logiken.

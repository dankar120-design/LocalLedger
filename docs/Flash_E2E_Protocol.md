# Flash E2E Protocol (Subagent Test Checklist)

Detta protokoll måste exekveras och alla punkter markeras med `[x]` innan vi påbörjar kodning av nya infrastrukturella funktioner. Syftet är att frystesta SPA-ombyggnationen.

## Instruktioner till Subagent (Flash)
1. Följ listan uppifrån och ner.
2. För varje steg, ta en skärmdump och bifoga resultatet.
3. Om ett steg kraschar, avbryt och returnera felrapporten.

---

## Testfall

- [ ] **1. Navigation & State:** Klicka igenom alla 8 sidomeny-knapparna. Verifiera att ingen vy överlappar en annan, och att `.active` CSS-klassen uppdateras korrekt på navigeringslänkarna.
- [ ] **2. Skapa Räkenskapsår:** Navigera till Inställningar -> Nytt Räkenskapsår. Skapa år 2026. Verifiera att året dyker upp i dropdownen i Huvudboken.
- [ ] **3. Bokför Verifikation:** I Huvudboken, ladda upp ett mock-kvitto (valfri bild) via magiska staven, välj konto 6110 och klicka "Bokför Verifikation". Verifiera att den dyker upp i tabellen under.
- [ ] **4. Storno-test:** Klicka på "Makulera/Storno" på den nyskapade verifikationen. Verifiera att en reverserande post skapas.
- [ ] **5. Fakturering Lifecycle:** Navigera till Fakturering. Skapa en testfaktura på 1000 kr exkl moms. Lägg till en kund. Klicka "Spara", sedan "Slutför och WORM-lås". Verifiera att statusen ändras till Skickad och en PDF skapas.
- [ ] **6. Kreditfaktura:** Försök radera den WORM-låsta fakturan. Systemet ska tvinga dig att utfärda en Kreditfaktura. Gör detta.
- [ ] **7. Momsredovisning:** Navigera till Momsredovisning. Välj rätt tidsperiod. Verifiera att momsbeloppet från testfakturan syns korrekt (t.ex. konto 2611).
- [ ] **8. Excel-export:** Navigera till Verktyg & Export. Klicka på "Exportera Finansiell Rapport". Verifiera att en CSV laddas ner och formatteras enligt svensk standard.
- [ ] **9. Backup-systemet:** Klicka på "Ladda ner Säkerhetskopia (.zip)".
- [ ] **10. WORM Inbox:** Öppna Inkorgen (dra ut från höger). Verifiera att listan öppnas smidigt och att du kan stänga den genom att klicka någon annanstans.
- [ ] **11. Årsavslut (Lås):** I Huvudboken, klicka "Lås Detta År". Skriv in PIN-koden (oftast 1234 eller 0000 i test). Verifiera att bokföringsformuläret döljs/avaktiveras och ersätts med "Räkenskapsåret är låst".
- [ ] **12. Tema-byte:** Klicka på sol/måne-ikonen. Verifiera mörkt/ljust tema utan att någon layout bryts.

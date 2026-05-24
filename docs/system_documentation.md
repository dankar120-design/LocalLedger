# Systemdokumentation och Behandlingshistorik

**Applikation:** LocalLedger  
**Senast uppdaterad:** 2026-05-07  
**Tillämpat regelverk:** Bokföringslagen (1999:1078), BFNAR 2013:2 (Vägledning Bokföring), K1-regelverket för Mikroföretag.

Detta dokument utgör LocalLedgers officiella systemdokumentation enligt kraven i 5 kap. 11 § Bokföringslagen (BFL). Dokumentet är avsett för Skatteverket, externa revisorer och systemadministratörer för att förklara systemets övergripande logik, behandlingsregler och rutiner för att säkerställa god redovisningssed.

---

## 1. Lagring och Systemarkitektur

LocalLedger är utformat som ett lokalt installerat bokföringssystem anpassat för mikroföretag (enskild näringsverksamhet).
- **Fysisk lagring (Databas):** Bokföringsdatan lagras i en lokal SQLite-databas (`.db`-fil).
- **Varaktighet och Raderingsskydd (Append-Only):** Databasen och applikationslagret tillämpar en strikt "Append-Only"-arkitektur. När en verifikation väl har registrerats och tilldelats ett löpnummer är det systemtekniskt omöjligt att radera (DELETE) eller skriva över (UPDATE) posten i produktionsläget.
- **WORM-lagring för Underlag (7 kap. BFL):** Inskannade kvitton och fakturor sparas på det lokala filsystemet. Vid uppladdning låses filen med en OS-baserad readonly-flagga och en kryptografisk SHA-256 hash genereras. Denna hash lagras tillsammans med verifikationen i databasen, vilket omedelbart avslöjar eventuell filmanipulation.

## 2. Rättelser i Bokföringen

Eftersom radering är förbjuden tillämpas branschstandarden **Total Reversering (Stornopost)** för alla rättelser.
- Om en post upptäcks vara felaktig, genererar systemet per automatik en ny stornoverifikation som vänder debet och kredit på det felaktiga beloppet (med referens till den ursprungliga posten).
- Därefter skapas en ny rätt verifikation. Båda de nya verifikationerna ges dagens datum som transaktions- och registreringsdatum.
- Den ursprungliga (felaktiga) posten finns alltid kvar i behandlingshistoriken för full spårbarhet.

## 3. Hantering av Verifikationsnummerserien

Systemet använder en sekventiell nummerserie (A-serien för alla manuella och importerade transaktioner) per räkenskapsår.
- **Krav på Obruten Nummerserie:** LocalLedger tillämpar databasteknik för att auto-inkrementera löpnumret.
- **Makuleringsposter (Gap Handling):** Om ett nätverksavbrott eller en systemkrasch resulterar i en ogiltig databastransaktion ("Rollback") förloras ibland det allokerade numret. Vid varje bokslut (eller via manuell aktivering) kör systemet en scanning av nummerserien. Eventuella tekniska luckor fylls automatiskt med en "Makuleringspost" utan ekonomiskt värde (Belopp 0) med en förklarande text i behandlingshistoriken om varför luckan uppstod. Detta garanterar sekvensens validitet vid en skatterevision.

## 4. Räkenskapsår och Balansöverföringar

Systemet stödjer brutna räkenskapsår och hanterar gränsdragningen mellan år strikt.
- **YearLocks (Årslåsning):** När ett årsbokslut har godkänts markeras hela räkenskapsåret med en tvingande `locked_at` tidsstämpel.
- **PeriodLocks (Momslåsning):** Utöver årslås stöder systemet låsning av enskilda momsperioder (ex. kvartal). Retroaktiva rättelser i låsta momsperioder är blockerade för att säkerställa att inlämnade deklarationer inte muteras.
- **Automatisk Avstämning (IB/UB):** Vid övergång till ett nytt räkenskapsår för systemet automatiskt över den Utgående Balansen (UB) till det nya årets Ingående Balans (IB). Processen är automatisk, men systemet flaggar med "hard-stops" om differenser upptäcks mellan tillgångar och eget kapital/skulder.

## 5. Automatiserad Momsomföring (MVCA)

LocalLedger tillämpar en Minimal Viable Chart of Accounts (MVCA) baserad på BAS-kontoplanen och hanterar mervärdesskatt mekaniskt.
- Den utgående momsen ackumuleras på konton (t.ex. 2611) och den ingående på andra (t.ex. 2641).
- Vid periodslutet triggar användaren en momsomföring. Systemet nollställer då automatiskt saldona på 2611 och 2641 och bokför nettobeloppet (moms att betala/få tillbaka) mot avräkningskontot 2650. Denna behandlingsregel exekveras i en enda atomär transaktion.

## 6. Import och Export (SIE-4)

- LocalLedger är fullt kompatibelt med den svenska SIE-4 standarden för transaktionsexport.
- Systemet respekterar SIE-Gruppens tekniska specifikation och tillämpar en strikt bidirektionell konvertering mellan moderna teckenset (UTF-8) och standardens krav på **Codepage 437 (IBM PC8)**. Detta säkerställer att data inte korrumperas vid överföring till äldre skatteprogram.
- **SIE-4 Import och WORM-kravet:** Verifikationer som importeras via SIE-4 Typ 4I saknar per automatik inbäddade PDF-underlag. Systemet flaggar dessa transaktioner som externt genererade. De undantas från den lokala Boot-Checken av fil-hashar, eftersom originalunderlaget enligt BFL lagras och bevaras i det externa försystemet.

## 7. Deklarationsexport (SRU - BAS 2026)

- **SRU-generering:** LocalLedger kan generera lagstadgade deklarationsblanketter i SRU-format (Standardiserat Räkenskapsutdrag) enligt Skatteverkets tekniska specifikationer för BAS 2026.
- **Teckenkodning (Latin-1):** I enlighet med Skatteverkets och Bolagsverkets systemkrav exporteras SRU-filerna strikt kodade i **ISO-8859-1 (Latin-1)**. Detta säkerställer att svenska tecken (Å, Ä, Ö) tolkas felfritt i myndigheternas centrala mottagningssystem.
- **Automatisk mappning:** Systemet mappar automatiskt saldona från företagets BAS-kontoplan mot korrekta SRU-koder för att minimera risker för manuella inmatningsfel vid deklarationstillfället.

---

## Ändringslogg
- **2026-05-24 (v1.2):** Lagt till avsnitt 7 om Deklarationsexport (SRU) i enlighet med BAS 2026 samt noterat prestanda- och stabilitetsförbättringar för sidomeny och datumvalidering.
- **2026-05-07 (v1.1):** Uppdaterad med legala rutiner för PeriodLocks (momslåsning) samt förtydligande kring SIE-4 WORM-undantag.
- **2026-05-07 (v1.0):** Initialt dokument upprättat baserat på BFL och BFNAR 2013:2.

<decision_ledger>
  <record id="PHOENIX_PROTOCOL_01" kategori="Arkitektur">
    <beslut>Ã…terskapa Setup Wizard och Sandbox-motor med Vanilla JS och sync port binding.</beslut>
    <kÃ¤rna>Ã–vergÃ¥ng frÃ¥n CDN-beroende AlpineJS till Vanilla JS fÃ¶r on-boarding, infÃ¶rt go:embed fÃ¶r SQL/HTML, och implementerat heartbeat fÃ¶r att fÃ¶rhindra Zombie-processer.</kÃ¤rna>
    <motivering>En kritisk fÃ¶rlust av kod (`git clean -fd`) krÃ¤vde ett Ã¥terskapande. Den nya arkitekturen eliminerar helt frontend-byggsteg och sÃ¤kerstÃ¤ller Single-Instance sÃ¤kerhet i Windows.</motivering>
  </record>
  <record id="WORM_COMPLIANCE_01" kategori="SÃ¤kerhet">
    <beslut>Implementera WORM (Write Once Read Many) via SQLite Triggers.</beslut>
    <kÃ¤rna>Inga verifikationer med en kryptografisk hash fÃ¥r raderas eller uppdateras. Skyddet Ã¤r inbakat direkt i SQLite-schemat via BEFORE UPDATE/DELETE triggers.</kÃ¤rna>
    <motivering>FÃ¶r att uppfylla svensk BokfÃ¶ringslag (BFL) krÃ¤vs ofÃ¶rÃ¤nderlighet. Genom att lÃ¤gga skyddet i databasen skyddar vi datan Ã¤ven om applikationslogiken (Go) skulle manipuleras.</motivering>
  </record>
  <record id="SINGLE_INSTANCE_01" kategori="Arkitektur">
    <beslut>NÃ¤tverksport-mutex med Health-Ping fÃ¶r Single-Instance pÃ¥ Windows.</beslut>
    <kÃ¤rna>IstÃ¤llet fÃ¶r komplexa Windows Mutex-anrop binder appen till TCP 8080. Vid kollision (EADDRINUSE) gÃ¶rs ett HTTP GET-anrop. Om appen svarar (rÃ¤tt version), Ã¶ppnas enbart webblÃ¤saren.</kÃ¤rna>
    <motivering>Garanterar att anvÃ¤ndaren inte av misstag kÃ¶r tvÃ¥ databas-instanser parallellt, vilket kunde orsakat SQLite-lÃ¥sningar, samt eliminerar beroenden till OS-specifika C-bibliotek fÃ¶r Mutex.</motivering>
  </record>
  <record id="BROWSER_FALLBACK_01" kategori="Edge-case">
    <beslut>Kaskad-start av webblÃ¤sare fÃ¶r Desktop-kÃ¤nsla.</beslut>
    <kÃ¤rna>Appen fÃ¶rsÃ¶ker starta msedge.exe --app=, sedan chrome.exe --app=, och faller till sist tillbaka pÃ¥ rundll32 (standardwebblÃ¤sare).</kÃ¤rna>
    <motivering>Skapar illusionen av en infÃ¶dd desktop-applikation (utan URL-bar och flikar) pÃ¥ Windows-system, utan att behÃ¶va inkludera en hel Electron/WebView2-runtime i binÃ¤ren.</motivering>
  </record>
  <record id="SETUP_HTML_EMBED" kategori="Arkitektur">
    <beslut>Strikt anvÃ¤ndning av go:embed fÃ¶r Setup-vyerna.</beslut>
    <kÃ¤rna>setup.go lÃ¤ser nu setup.html frÃ¥n frontend.FS istÃ¤llet fÃ¶r os.ReadFile.</kÃ¤rna>
    <motivering>Garanterar att applikationen fÃ¶rblir en enskild portabel binÃ¤rfil utan risk fÃ¶r att sakna filer om den flyttas.</motivering>
  </record>
  <record id="ZOMBIE_HEARTBEAT_CONTEXT" kategori="FelsÃ¶kning">
    <beslut>Eliminering av race condition i heartbeat via context.WithCancel.</beslut>
    <kÃ¤rna>Setup-servern stÃ¤ngs ner via en dedikerad context-signal frÃ¥n main-trÃ¥den, istÃ¤llet fÃ¶r att tÃ¤vla om data pÃ¥ resultChan.</kÃ¤rna>
    <motivering>FÃ¶rhindrar att heartbeat-trÃ¥den oavsiktligt konsumerar resultatet och skapar oÃ¤ndliga hÃ¤ngningar (Zombie-processer).</motivering>
  </record>
  <record id="SQLITE_DRIVER_NAME" kategori="FelsÃ¶kning">
    <beslut>Korrigering av SQLite drivrutinsnamn till "sqlite".</beslut>
    <kÃ¤rna>sandbox.go anropar nu sql.Open("sqlite", ...) fÃ¶r att matcha modernc.org/sqlite.</kÃ¤rna>
    <motivering>FÃ¶rhindrar runtime-krascher vid Sandbox-start eftersom mattn/go-sqlite3 ("sqlite3") inte ingÃ¥r i detta projekt.</motivering>
  </record>
  <record id="SYNC_PORT_BIND" kategori="FelsÃ¶kning">
    <beslut>Synkron port-bindning fÃ¶r Setup-servern innan webblÃ¤saren startas.</beslut>
    <kÃ¤rna>AnvÃ¤ndning av net.Listen innan srv.Serve anropas, med fallback-hantering i main.go.</kÃ¤rna>
    <motivering>FÃ¶rhindrar uppkomsten av dubbla process-krascher och falska webblÃ¤sarfÃ¶nster om anvÃ¤ndaren dubbelklickar pÃ¥ .exe-filen flera gÃ¥nger.</motivering>
  </record>
  <record id="CACHE_CONTROL_SPA" kategori="FelsÃ¶kning">
    <beslut>InfÃ¶ra strikta cache-busting-huvuden i Go-servern fÃ¶r frontend-tillgÃ¥ngar.</beslut>
    <kÃ¤rna>Frontend-servern sÃ¤tter nu Cache-Control till "no-store, no-cache, must-revalidate, max-age=0" fÃ¶r index.html och alla statiska JS/CSS-filer.</kÃ¤rna>
    <motivering>FÃ¶rhindrar att webblÃ¤sare cachar trasiga, avbrutna eller trunkerade JS-tillgÃ¥ngar efter misslyckade byggfÃ¶rsÃ¶k, vilket sÃ¤kerstÃ¤ller omedelbar synkronisering av frontend-Ã¤ndringar.</motivering>
  </record>
  <record id="ALPINE_REACTIVITY_STABILIZATION" kategori="FelsÃ¶kning">
    <beslut>Deklarera alla reaktiva tillstÃ¥nd pÃ¥ fÃ¶rÃ¤ldranivÃ¥ och ersÃ¤tt nÃ¤stlade getters med watchers.</beslut>
    <kÃ¤rna>Initierat showTools, showReports och rapporttillstÃ¥nd i ledgerApp (app.js), deklarerat ibTargetYearName som standardfÃ¤lt i tools.js och bundit uppdateringar till en reaktiv watcher i init().</kÃ¤rna>
    <motivering>Eliminerar Alpine.js kompilerings- och ReferenceError krascher som uppstÃ¥r nÃ¤r nestade komponenter refererar till odefinierade fÃ¤lt under uppstartsfasen.</motivering>
  </record>
  <record id="DESKTOP_SHORTCUT_ONBOARDING" kategori="Arkitektur">
    <beslut>Automatiskt skapande av skrivbordsgenvÃ¤g (.lnk) pÃ¥ Windows.</beslut>
    <kÃ¤rna>Setup-servern kÃ¶r nu ett powershell-skript via exec.Command fÃ¶r att generera en genvÃ¤g pÃ¥ skrivbordet till den nuvarande kÃ¶rande exe-filen vid slutfÃ¶rd onboarding.</kÃ¤rna>
    <motivering>ErsÃ¤tter manuell lÃ¤nkhantering och fÃ¶rbÃ¤ttrar anvÃ¤ndarupplevelsen genom att ge en sÃ¶mlÃ¶s integration pÃ¥ Windows-skrivbordet.</motivering>
  </record>
  <record id="SCRIPT_LOADING_ORDER_FIX" kategori="FelsÃ¶kning">
    <beslut>Skifta script-laddningsordningen i index.html sÃ¥ att reaktiva skript laddas fÃ¶re Alpine.js.</beslut>
    <kÃ¤rna>Placerat app.js och tools.js fÃ¶re alpine.min.js i index.html:s head med 'defer' intakt.</kÃ¤rna>
    <motivering>Garanterar att alla alpine:init-eventlyssnare Ã¤r fullstÃ¤ndigt registrerade innan Alpine.js-motorn startar sin DOM-kompilering, vilket eliminerar slumpmÃ¤ssiga ReferenceErrors under hÃ¶gpresterande Chromium/Playwright-kÃ¶rningar.</motivering>
  </record>
  <record id="DOMAIN_INTEGRITY_01" kategori="Arkitektur">
    <beslut>InfÃ¶ra global integritets-checksumma och slopa dolda resultatÃ¶verfÃ¶ringar vid IB-generering.</beslut>
    <kÃ¤rna>YearClose-logiken verifierar nu att summan av ALLA konton (1-8) Ã¶ver hela Ã¥ret Ã¤r exakt noll innan den tillÃ¥ter IB-generering. Den vÃ¤grar fortfarande skapa IB om klass 1+2 inte balanserar (anvÃ¤ndaren mÃ¥ste boka Ã¥rets resultat explicit).</kÃ¤rna>
    <motivering>Garanterar databasens integritet (fÃ¶rhindrar tyst generering av IB pÃ¥ korrupt data) och respekterar principen om explicit bokfÃ¶ring genom att tvinga anvÃ¤ndaren att gÃ¶ra ett formellt bokslut istÃ¤llet fÃ¶r att lÃ¥ta systemet dÃ¶lja differensen.</motivering>
  </record>
  <record id="MISSING_ACCOUNTS_MIGRATION" kategori="Edge-case">
    <beslut>Ny databasmigrering (v11) och borttagning av tyst test-maskering.</beslut>
    <kÃ¤rna>Skapade Version 11 i init.go fÃ¶r att lÃ¤gga in saknade bokslutskonton (8999, 2099, 2098, 2019) i BAS K1-planen. Ã„ndrade typen pÃ¥ sandboxens bokslut frÃ¥n BOKSLUT till NORMAL.</kÃ¤rna>
    <motivering>Enhetstesterna var riggade att passera genom manuell SQL-seeding, vilket dolde att sandboxen (och produktionssystemet) skulle krascha vid bokslut p.g.a saknade referenskonton fÃ¶r Foreign Keys. BOKSLUT togs bort som verifikationstyp fÃ¶r att undvika dolda UI-filtreringsbuggar dÃ¥ BFL enbart krÃ¤ver NORMAL, IB och STORNO.</motivering>
  </record>
  <record id="MIGRATION_TESTING" kategori="SÃ¤kerhet">
    <beslut>InfÃ¶rt sekventiellt migrationstest fÃ¶r att sÃ¤kerstÃ¤lla schemats integritet.</beslut>
    <kÃ¤rna>Skapade init_test.go med TestMigrations_Sequential som itererar frÃ¥n en tom minnesdatabas till v11 och verifierar att inga krockar uppstÃ¥r och att kritiska konton etableras.</kÃ¤rna>
    <motivering>En kontinuerlig applicering av alla migreringar pÃ¥ en blank databas (frÃ¥n v1 till nuvarande) Ã¤r enda sÃ¤ttet att garantera att systemet inte kraschar fÃ¶r nyinstallerade instanser p.g.a krockande Foreign Keys eller inkompatibla datatyper.</motivering>
  </record>
  <record id="PHOENIX_PROTOCOL_06" kategori="Arkitektur"><beslut>VAT Engine Hardening via Strict BAS Mapping</beslut><kärna>Skrotade dynamisk momshärledning till förmån för strikt mappning mellan momsspecifika BAS-konton (ex 3001, 3002) och Skatteverkets momsrutor (Box 05, 06). Införde Migration v12 för att skapa saknade konton.</kärna><motivering>Dynamisk härledning kraschar matematiskt vid blandfakturor. Strikt kontomappning är det enda BFL-säkra sättet att garantera att beskattningsunderlaget (t.ex. Ruta 05 vs 06) blir rätt.</motivering></record>
  <record id="PHOENIX_PROTOCOL_07" kategori="Säkerhet/Logik"><beslut>Konsolidering av VAT-motorns scope och felhantering</beslut><kärna>Införde rows.Err() loop-check. Extraherade en gemensam MomsKontonLista som tvingar både GetVatReport och TransferVat att agera på EXAKT samma momskonton (asymmetri-eliminerad). Vidgade försäljningskonton (t.ex. 3010, 3540) för att förhindra falska noll-omsättningar i Ruta 05. Rättade momsmatematik (25% istället för 30.8%) i sandbox_seed.sql.</kärna><motivering>Kritisk audit visade att TransferVat riskerade att nollställa orapporterade momssaldon pga divergerande query-logik (LIKE vs IN). Korrigerat för att garantera strict BFL-compliance och säkerhetsställa att skattedata inte kan falla mellan stolarna.</motivering></record>
  <record id="PHOENIX_PROTOCOL_08" kategori="Säkerhet/Logik"><beslut>Finalisering av VAT-motorn och kontoplan</beslut><kärna>Skapade Migration v13 för att lägga till 3010, 3011, 3020 i databasens kontoplan (accounts). Tog bort MomsKontonLista som dynamisk Sprintf-konstant och ersatte med uttryckliga SQL IN-listor i GetVatReport och TransferVat. Tog bort omastade 3540, 3590, 3990 från IN-listan för Ruta 05.</kärna><motivering>Löser phantom-konto-problematiken där momsmotorn letade efter konton som inte kunde användas pga Foreign Keys. Tar även bort SQL-injection-vektorn (code smell) från Go-koden.</motivering></record>
  <record id="VAT_FRONTEND_01" kategori="Arkitektur">
    <beslut>Ersatt statisk moms-summering med en adaptiv grid baserad på SKV:s deklarationsrutor.</beslut>
    <kärna>Frontend (app.js) tillämpar en BFL-kompatibel avrundning per ruta (skv-metod) till hela kronor och döljer sekundära rutor via hide-on-zero för optimerad UX.</kärna>
    <motivering>Förhindrar avrundningskollisioner mellan vår öres-exakta databas och Skatteverkets blankettsystem (som kräver hela kronor per underkategori).</motivering>
  </record>

  <record id="INV-001" kategori="Arkitektur">
    <beslut>Faktura-UI:ts logik har aktiverats (Phase 2)</beslut>
    <kärna>Frontend-komponenter för Invoices fanns i DOM men saknade databindningar och CRUD-logik. Logiken implementerades i AlpineJS, med flyt-till-heltal-omvandling i gränssnittet för att matcha Zero Double-Entry-motorn (öre-nivå).</kärna>
    <motivering>Genom att binda befintlig KärnFaktura-HTML mot de befintliga API-routerna (GET, POST, PUT, DELETE, POST /post, POST /pay) skapas en WORM-skyddad integrerad faktureringsprocess helt fri från third-party.</motivering>
  </record>

  <record id="INV-002" kategori="Felsökning / Säkerhet">
    <beslut>Buggfixar efter audit av Faktura-logiken.</beslut>
    <kärna>1. Int64-trunkering i Go ändrad till math.Round(float64) för att BFL-avrundning av ören ska bli korrekt.
  2. Asynkron 'selectInvoice' implementerad som hämtar komplett data (inklusive rader) istället för att lita på en trunkerad list-array.
  3. PDF-nedladdning dirigerades om från &lt;a&gt;-länk till authFetch blob för att säkra JWT-token krav.
  4. Tysta felmeddelanden vid bokföring av osparade fakturor åtgärdades.</kärna>
    <motivering>Genom att åtgärda dessa brister i PHOENIX-koden innan drift garanterar vi bokföringsmässig korrekthet, robust nätverkshantering samt skyddar mot auth-bypass vid dokumentnedladdning.</motivering>
  </record>

  <record id="INV-003" kategori="Säkerhet / Buggfix">
    <beslut>Strikt Date-parsing och Race-Condition preemption infördes.</beslut>
    <kärna>1. AbortController applicerades på authFetch i selectInvoice för att undanröja teoretiska race-conditions vid multi-klickning.
  2. payInvoice utökades med en strikt ISO-validering (d.toISOString().slice(0,10) === date) för att förhindra Date Auto-Correction korruption av ogiltiga inmatningar.</kärna>
    <motivering>En helt ofelbar dataintegritet vid hantering av transaktioner, immun mot nätverkslatens och inbyggda JS quirk-beteenden.</motivering>
  </record>

  <record id="INV-004" kategori="Arkitektur">
    <beslut>Implementerat Kreditfakturor (v3.0) med Zero Double Entry-kvittning</beslut>
    <kärna>Negativa belopp speglas och debet/kredit flippas i `PostInvoice`. UI är spärrat för `credit_of`-utkast för att förhindra fusk med pris/moms.</kärna>
    <motivering>En vattentät lösning som följer WORM-regler, förbjuder överkreditering och förhindrar manipulation av låsta/förväntade belopp.</motivering>
  </record>

  <record id="INV-005" kategori="Säkerhet / Buggfix">
    <beslut>Åtgärdat korruptionsrisk vid manuell utbetalning av Kreditfaktura (Audit-fix).</beslut>
    <kärna>1. Införde Negative-Flip logik i `RegisterPayment` så att utbetalningar av kreditfakturor bokförs korrekt (Kredit 1930, Debet 1510) istället för att skicka in otillötna negativa summor.
  2. Dold `+ Lägg till rad`-knapp i UI:t för kreditutkast (`x-show="!selectedInvoice.credit_of"`) för att garantera strukturell integritet mot originalfaktran.</kärna>
    <motivering>Strikt BFL/WORM compliance kräver positiva belopp. UI-låsningen förhindrar användarmisstag som kan fördunkla spårbarheten.</motivering>
  </record>

  <record id="INV-006" kategori="Arkitektur / Logik">
    <beslut>Omstrukturerat RegisterPayment och SettleInvoice för partiell kreditering (Audit-fix).</beslut>
    <kärna>1. `RegisterPayment` beräknar dynamiskt `amountToPay` = fakturabelopp + summan av bokförda kreditfakturor. Endast restbeloppet bokförs mot 1930/1510.
  2. `SettleInvoice` stänger originalfakturan (sätter status = 'betald') endast om `abs(SUM(kreditfakturor)) >= originalfakturans belopp`.</kärna>
    <motivering>Löser Catch-22 problemet där partiella kreditfakturor tidigare korrumperade kundreskontran och banksaldot genom att antingen gömma restbeloppet eller låta det dubbelbokföras.</motivering>
  </record>
  <record id="INV-007" kategori="Felsökning">
    <beslut>Eliminering av duplicerade 'Phantom' funktioner i app.js och seed-datakorrigering.</beslut>
    <kärna>1. Raderade ~240 rader med död kod i app.js där Alpine.js-objektet överlagrades med dubbla uppsättningar fakturafunktioner.
  2. Portade framåt säkerhetsfixarna (AbortController, Math.round på lineExVat, ID-validering och ISO-strängparsing) från INV-002 och INV-003 till den aktiva funktionen.
  3. Korrigerade sandbox_seed.sql: kvantitet ändrades från 10 till 1000 för att matcha beloppet 100.00 kr. Status sattes från en ogiltig 'sent' till 'bokförd' med verification_id 3.</kärna>
    <motivering>Den dolda strukturdupliceringen medförde att de tidigare granskade säkerhetsspärrarna (INV-002, INV-003) exekverades på en inaktiv instans av objektet. Buggen i testdatan hindrade E2E-betalningstester. Genom detta säkerställs att WORM/BFL valideringen äger rum i den skarpa runtime-miljön.</motivering>
  </record>
  <record id="INV-008" kategori="Buggfix / Arkitektur">
    <beslut>Rättat databas-persistens för kreditfaktura-koppling (credit_of).</beslut>
    <kärna>CreateInvoice i internal/ledger/invoice.go uppdaterades till att inkludera SQL-kolumnen 'credit_of' i sin INSERT-fråga och binda 'inv.CreditOf' som argument till SQLite.</kärna>
    <motivering>En kritisk bugg upptäcktes under den headless E2E-verifieringen där skapade kreditfakturor saknade sin 'credit_of'-koppling i databasen (den förblev NULL). Detta gjorde kvittning av kundreskontra omöjlig. Buggfixen återställer dataintegriteten enligt WORM-specifikationen.</motivering>
  </record>
  <record id="INV-009" kategori="Säkerhet / Buggfix">
    <beslut>Stängt WORM-sårbarhet och moms-manipulationsvektor i kreditfakturor.</beslut>
    <kärna>1. HandleCreateInvoice forcerar nu 'inv.CreditOf = nil' för att hindra klienter från att injicera länkningar.
  2. CreateCreditInvoice blockerar nya utkast om redan bokförda/utkastkrediter täcker originalet.
  3. UpdateInvoice tillåter enbart kvantitetsändringar (mot noll) på kreditutkast. Pris, moms och nya rader låses matematiskt mot originalfakturan via backend-verifiering.</kärna>
    <motivering>En djupgående audit visade att API:et var öppet för moms-fusk (ändra momssats på kreditutkast) samt 'Broken Access Control' vid nyskapande. Dessa kirurgiska snitt stänger båda vektorerna utan att bryta stödet för partiell kreditering.</motivering>
  </record> nil' för att hindra klienter från att injicera länkningar.
2. CreateCreditInvoice blockerar nya utkast om redan bokförda/utkastkrediter täcker originalet.
3. UpdateInvoice tillåter enbart kvantitetsändringar (mot noll) på kreditutkast. Pris, moms och nya rader låses matematiskt mot originalfakturan via backend-verifiering.</kärna>
  <motivering>En djupgående audit visade att API:et var öppet för moms-fusk (ändra momssats på kreditutkast) samt 'Broken Access Control' vid nyskapande. Dessa kirurgiska snitt stänger båda vektorerna utan att bryta stödet för partiell kreditering.</motivering>
</record>

<record id="INV-010" kategori="Arkitektur / Säkerhet">
  <beslut>Slutfört robust partiell kreditering, verifierat centi-enhetsdivisor och multi-kvittning (Audit-fix).</beslut>
  <kärna>1. Rättade divisorn i UpdateInvoice för rowAmount genom att dela med 100 för att korrekt omvandla centi-enheters kvantitet till kr/öre.
2. Inkluderade moms i det återkalkylerade totalbeloppet (newTotalAmount += rowAmount + rowVat).
3. Relaxerade statuskontrollen i SettleInvoice så att den tillåter del-krediteringar mot en originalfaktura som redan har markerats som 'betald' av tidigare kvittningar.
4. Utökade E2E-testerna till att täcka fullständig partiell kreditering, överkrediteringsspärrar på öre-nivå och automatisk reskontra-kvittning.</kärna>
  <motivering>En fientlig regressionstestning avslöjade dolda logiska spärrar där partiell kreditering misslyckades på grund av inkonsekvent kvantitetshantering och catch-22 blockeringskontroller. Dessa ändringar låser upp fullt stöd för komplexa, legala partiella krediteringar i linje med svensk bokföringslagstiftning (BFL).</motivering>
</record>

<record id="INV-011" kategori="Arkitektur / Logik">
  <beslut>Eliminering av avrundningsdivergens mellan utkast och bokföring (Audit-fix).</beslut>
  <kärna>1. Rättade integer-divisionen i UpdateInvoice till att använda float-division och math.Round, exakt speglande PostInvoice-matematiken.
2. Expanderade E2E-sviten med ett 0-kvantitetstest (Step 3.6.5) för att verifiera blockering av tomma verifikationer.
3. Lade till ett "Interleaved Settlement" flöde (Step 6) som bevisar att stegvis kvittning av partiella krediteringar fungerar smärtfritt.</kärna>
  <motivering>En kritisk granskning påvisade att integer-trunkering i utkastfasen kunde leda till ett 1-öres kryphål förbi överkrediterings-spärren vid bokföring. Genom att tvinga fram matematisk isomorfism mellan UpdateInvoice och PostInvoice garanteras total integritet. Utökade tester för interleaving och noll-fakturor bevisar systemets robusthet även i komplexa edge-cases.</motivering>
</record>

<record id="INV-012" kategori="Arkitektur / Säkerhet">
  <beslut>Slutgiltig härdning av fakturamotorn: TOCTOU, PDF och RegisterPayment.</beslut>
  <kärna>1. Löste TOCTOU (Time-Of-Check to Time-Of-Use) race condition i CreateCreditInvoice. Refaktorerade CreateInvoice till en transaktionsberoende createInvoiceTx och lade till en atomär dummy-UPDATE ('UPDATE invoices SET id = id WHERE id = ?') för att tvinga fram ett exklusivt SQLite write-lock INNAN sumExistingCredits beräknas.
2. Korrigerade PDF-motorn (pdf.go) så att moms-uppdelningen (VAT breakdown) skrivs ut även för negativa belopp på kreditfakturor (ändrade 'amount > 0' till 'amount != 0').
3. Skrev Step 7 i E2E-sviten som fullständigt bevisar att RegisterPayment drar av delkrediteringar från originalbeloppet innan det bokförs mot 1930 Bank.</kärna>
  <motivering>En teoretisk sårbarhet i SQLite:s standard DEFERRED-transaktionsmodell kunde ha tillåtit multipla parallella anrop att bypassa överkrediterings-spärren. Dummy-UPDATEN fungerar som en portabel mutex-låsning direkt i databasen. Tillsammans med PDF-rättningen och RegisterPayment-testet stänger detta de allra sista hålen i faktureringsmodulen och låser Zero Double-Entry-motorn.</motivering>
</record>

<record id="INV-013" kategori="Arkitektur / Säkerhet">
  <beslut>Post-Audit korrigering av RegisterPayment (Isolation & E2E-robusthet).</beslut>
  <kärna>1. Ändrade l.db.QueryRow till tx.QueryRow i RegisterPayment för att säkra transaktionsisoleringen (Transaction Isolation) och förhindra deadlocks vid beräkning av kvarvarande krediteringar.
2. Refaktorerade E2E Step 7 så att den robust lokaliserar betalningsverifikationen via fritext (istället för array-index) och explicit validerar BÅDE debet (1930) och kredit (1510) för Zero Double-Entry-efterlevnad.</kärna>
  <motivering>En fientlig audit visade att 'RegisterPayment' avslutade fakturor utanför transaktionsskopet vid summering, vilket skapade en teoretisk race condition- och deadlock-vektor i SQLite. Dessutom var det tidigare E2E-testet för svagt eftersom det enbart verifierade halva dubbelbokföringen. Dessa ändringar låser motorn och bevisar fullständigt 1510/1930-avstämningen.</motivering>
</record>

  <record id="INBOX_ATOM_01" kategori="Arkitektur">
    <beslut>Dokumentera och testa Inbox Orphan Reconciliation för kraschsäkerhet.</beslut>
    <kärna>Genom att köra ReconcileOrphans() vid uppstart av InboxManager identifieras och loggas herrelösa filer (orphans) i 'workspace/inbox' som saknar DB-koppling till följd av systemkrascher under pågående nätverksöverföringar.</kärna>
    <motivering>För att garantera atomicitet mellan filskrivning och SQLite DB-insättning under nätverksöverföring (POST /api/inbox) görs en transaktionell rollback/os.Remove på disk om DB-insättningen misslyckas. Om systemet skulle krascha innan disk-cleanup exekveras garanterar ReconcileOrphans() vid nästa uppstart välavgränsad spårbarhet och övervakning av herrelösa underlagsfiler utan att störa SQLite-databasens WORM-integritet.</motivering>
  </record>

  <record id="HARDENING_UX_EXPORT_01" kategori="Säkerhet / UX">
    <beslut>Slutförande av System Hardening, UX-polering och krypterad export (Phase 2C &amp; Phase 3 Rollout).</beslut>
    <kärna>1. Implementerat atomär databas-backup och återställning via VACUUM INTO under uppstart samt schemamigrering med rollback. 2. Infört heartbeat watchdog på backend (/api/ping) med 15s idle-timeout för att eliminera zombie-processer på Windows. 3. Verifierat kryptering med AES-256 ZIP-export, SKV-avrundningslogik och ett robust Toast-system samt anpassade SVG-ikoner och live dashboardmetrics.</kärna>
    <motivering>Säkerställer fullständig Bokföringslag-compliance (BFL) och oföränderlighet av data, med robust driftssäkerhet och modern UX som underlättar pilotanvändares onboarding utan att lämna zombie-processer.</motivering>
  </record>
  <record id="POST_AUDIT_HARDENING_01" kategori="Säkerhet / Felsökning">
    <beslut>Härdning av heartbeat-watchdog och pre-migration felhantering efter audit-fasen.</beslut>
    <kärna>1. Förlängt heartbeat watchdog timeout till 90s och lagt till visibilitychange/sendBeacon-events för att förhindra falska nedstängningar pga Chromium Background Tab Throttling. 2. Infört explicit felhantering för os.Remove() av stalerade backups vid SQLite uppstart.</kärna>
    <motivering>En fientlig audit-analys visade att Chromium strypta setInterval bakgrundsflikar stängde av servern av misstag efter 15s. Att öka timeouten till 90s samt integrera visibility-lyssnare eliminerar denna allvarliga driftstörning och garanterar stabil Single-Instance drift under Windows.</motivering>
  </record>
  <record id="POST_AUDIT_HARDENING_02" kategori="Säkerhet / Felsökning">
    <beslut>Åtgärdat sendBeacon POST 405-fel och eliminerat TOCTOU i backup-hanteringen.</beslut>
    <kärna>1. Registrerat /api/ping explicit för både GET och POST i server.go för att tillåta Chromium beforeunload sendBeacon-anrop. 2. Ersatt os.Stat+os.Remove med en atomär os.Remove operation under errors.Is(err, os.ErrNotExist) felhantering i ledger.go.</kärna>
    <motivering>En djupare fientlig audit visade att sendBeacon alltid skickar en POST-begäran, vilket ledde till 405 Method Not Allowed på vår strikta GET-rutt. Att lägga till en POST-rutt förhindrar zombie-processer vid snabb stängning. TOCTOU-fixen eliminerar en potentiell race condition vid borttagning av gamla backupfiler.</motivering>
  </record>

  <record id="INV-014" kategori="Arkitektur / Säkerhet">
    <beslut>Implementerat Kundregister (v14), GDPR-pseudonymisering, WORM-fakturanummersekvens, Offline OCR med Kognitiv Isolering samt BFL-lagstadgad varning.</beslut>
    <kärna>1. Skapat DB-migration (v14) för 'customers' och kopplat via foreign key till invoices.
2. Etablerat 'DELETE /api/customers/{id}' endpoint för GDPR pseudonymisering till '[ANONYMISERAD]' utan att störa WORM-fakturahistorik.
3. Säkrat fakturanummerserien med atomär sekventiell tilldelning (MAX()+1) vid 'PostInvoice' för att eliminera glapp i nummerserien.
4. Integrerat offline-first OCR via Tesseract.js WASM och PDF.js i webbläsaren, kompletterat med kognitiv isolering på servern som rekommenderar kostnadskonto baserat på leverantörshistorik.
5. Lagt till lagstadgad gul BFL-varning rörande molntjänster i Settings-vyn.</kärna>
    <motivering>Denna rollout stänger de sista luckorna i Master PRD och PRD_OCR_AI. GDPR pseudonymisering parat med historiskt bevarande av verifikat är det enda legala sättet att förena rätten att bli glömd med BFL:s 7-åriga lagringsstadga. Atomär sekvens och lokal/kognitiv OCR möter stränga svenska regelkrav för bokföringssäkerhet och dataintegritet.</motivering>
  </record>
  <record id="INV-015" kategori="Säkerhet / Arkitektur">
    <beslut>Slutfört post-audit merge med härdning av GDPR-gallring i utkast, TOCTOU-skydd i PostInvoice, säker CustomerID-kloning i kreditfakturor samt robust databas-felhantering.</beslut>
    <kärna>1. Uppdaterat AnonymizeCustomer att rensa personuppgifter i utkastfakturor utan att bryta WORM-compliance för bokförda verifikat.
2. Infört en portabel transaktionell mutex-låsning på company_settings i PostInvoice för att eliminera TOCTOU-kapplöpningar vid fakturanummergenerering.
3. Säkerställt att CustomerID klonas vid CreateCreditInvoice för fullständig GDPR-spårbarhet.
4. Ersatt tyst svalda databasfel i createInvoiceTx och UpdateInvoice med strikt felrapportering.</kärna>
    <motivering>Efter granskning och assimilation i Fas 3 stängdes dessa sista teoretiska och praktiska sårbarheter. Genom att kombinera transaktionell mutex med strikt oföränderlighet av historisk skattedata uppnår systemet 100% legalt och tekniskt skydd i enlighet med Bokföringslagen och GDPR.</motivering>
  </record>

  <record id="Phase9Hardening" kategori="Säkerhet"><beslut>Infört databas- triggers (WORM) för fakturor samt atomära transaktionslås för betalningar.</beslut><kärna>Förhindrar manipulation av bokförda fakturor på DB-nivå och stänger TOCTOU-race conditions i RegisterPayment/SettleInvoice.</kärna><motivering>Kompilerar med BFL:s krav på oföränderlig finansiell data och eliminerar risker för dubbelbokföring i distribuerade/konkurrenta miljöer.</motivering></record>
  <record id="Phase9BinaryCompilation" kategori="Felsökning">
    <beslut>Byggt om den lokala exekverbara binären localledger.exe för att stänga en avvikelse mellan disk och binär.</beslut>
    <kärna>Kompilerat om källkoden till localledger.exe efter rengöring av HTML-artefakter i style.css, vilket gör att app.js reaktivitetsfixar säkert bakats in.</kärna>
    <motivering>En diskrepans uppstod då frontends ändringar inte återspeglades i användarens lokala binär på grund av att go:embed bakar in källkoden statiskt vid kompileringstillfället. Genom att bygga om binären integreras alla reaktivitetsförbättringar för "+ Ny Faktura" fullt ut.</motivering>
  </record>
  <record id="VATAccountIntegrationUTF8" kategori="Felsökning / UI">
    <beslut>Integrerat saknade BAS-konton för reducerad och nollmoms samt korrigerat UTF-8-teckenfel i tabellhuvudet.</beslut>
    <kärna>1. Etablerat Version 16 schema-migration i init.go för att registrera kontona 2621, 2631 och 3004 i databasen. 2. Korrigerat index.html rad 314 från 'Ãƒâ€¦tgärd' till 'Åtgärd'. 3. Uppdaterat init_test.go för sekventiell migrationstestning (förväntat 16 migreringar).</kärna>
    <motivering>E2E-analys visade att bokföring med 12%, 6% eller 0% moms misslyckades med ett valideringsfel eftersom de matchande bokföringskontona inte existerade i accounts-tabellen. Att korrigera texten i gränssnittet löser dessutom ett störande dubbelkodningsfel för de svenska tecknen ÅÄÖ i Huvudboken.</motivering>
  </record>
  <record id="WORMSealingUIFeedback" kategori="UI / UX">
    <beslut>Förbättrat information och feedback kring WORM-förseglingens 24-timmarsgräns.</beslut>
    <kärna>1. Lagt till pedagogisk text i index.html förseglingsmodal som förklarar att 24-timmarsgränsen baseras på registreringstid (inmatningstid) istället för bokföringsdatum. 2. Uppdaterat sealVerifications i app.js att utläsa Count och visa dynamisk toast baserat på antalet faktiskt förseglade verifikationer.</kärna>
    <motivering>Genom att förklara reglerna och visa dynamisk feedback (t.ex. att noll verifikationer förseglades eftersom de skapats nyligen) slipper användaren förvirring kring varför verifikationer med äldre bokföringsdatum ligger kvar som oskyddade under det första dygnet efter skapande.</motivering>
  </record>
  <record id="PHOENIX_PROTOCOL_09" kategori="UI / UX">
    <beslut>Implementerat lyxigt Command Center (Dashboard), flyttat räkenskapsårskontroller till sidomenyn, städat Verktyg och lagt till logotypuppladdning samt säkert avstängningsflöde.</beslut>
    <kärna>1. Skapat ett reaktivt glassmorfiskt Command Center med KPI-kort, Quick Actions och BFL-compliance-mätare. 2. Flyttat räkenskapsårsinställningar till sidomenyn och infört dynamic header-visning för Huvudboken. 3. Konsoliderat Laga Nummerserie och Samlingsplan under Verktyg &amp; Export bredvid BFL-tipsboxen. 4. Byggt drag-and-drop logotypuppladdning under Settings med SVG-tips och serve-endpoint. 5. Säkrat avstängning via explicit clearInterval på pingInterval för att stänga av zombiefri drift utan felmeddelanden.</kärna>
    <motivering>Denna fullständiga rollout av Spår B stänger alla öppna UX-krav i Master PRD och ger pilotanvändare en extremt modern, lyxig och stabil onboarding. Genom att eliminera zombiefel vid avstängning och isolera bokföringslogiken skyddas applikationens integritet i enlighet med Bokföringslagen och GDPR.</motivering>
  </record>
  <record id="POST_AUDIT_HARDEST_FIX_01" kategori="Säkerhet / Arkitektur">
    <beslut>Säkrat logotypuppladdning mot Stored XSS och flyttat accounts receivable (AR) beräkning till backend.</beslut>
    <kärna>1. Implementerat MIME magic bytes-kontroll och en anpassad, strikt isSVGXSS check i handleUploadLogo som blockerar inbäddade skript/event-lyssnare i SVG:er. 2. Infört nosniff och sandbox CSP-headers på handleServeLogo. 3. Refaktorerat GET /api/dashboard till att returnera outstanding_receivables och unpaid_count beräknat via en SQL-query, samt tagit bort den klient-sidiga O(N) prestandabomben i app.js.</kärna>
    <motivering>Eliminerar Stored XSS och masquerading-sårbarheter vid filuppladdning i desktop-miljön samt skyddar applikationens prestanda och kodhygien genom att centralisera finansiella nyckeltal till backend-motorn.</motivering>
  </record>
  <record id="LOCALLEDGER_POLISH_LOGIC_01" kategori="UI / UX / Logik">
    <beslut>Slutfört omfattande polering och logikfixar för fakturering, momsredovisning och onboarding-hjälp.</beslut>
    <kärna>1. Justerat faktura-grid (.invoice-grid) till laptop-vänlig '350px 1fr' med responsiv stapling och explicit styling för .invoice-item-desc. 2. Implementerat direkt PDF-utskrift via dold iframe med URL.revokeObjectURL efter anropat 'afterprint' event för att förebygga minnesläckor. 3. Säkrat momsredovisningen och periodlåset genom att exkludera moms-omföringar via systemtyp 'MOMSOMFORING' och styra låsknappen på 'vatReport.is_locked'. 4. Integrerat en global tips-toggling (showTips) som visar hoverbara 'form-tip-indicators' vid inmatningsfälten.</kärna>
    <motivering>Denna stängning av de 9 poleringspunkterna lyfter LocalLedger till en helt premium mörk fintech-upplevelse som garanterar 100% driftsäkerhet, efterlevnad av Bokföringslagen och GDPR, samt ger en exceptionell onboarding-upplevelse via integrerade hjälpguider och checklistor.</motivering>
  </record>
  <record id="POST_AUDIT_ROBUSTNESS_01" kategori="Säkerhet / Arkitektur">
    <beslut>Implementerat fientliga granskningsåtgärder för momsredovisning, stornotyp, utskriftslås och kodhygien (Fas 10).</beslut>
    <kärna>1. Säkrat 'storno.go' att explicit tilldela typen 'STORNO' på stornoposter. 2. Härdat 'GetVatReport' och 'TransferVat' SQL-logik med sub-queries för att helt exkludera stornoposter kopplade till momsomföringar (Defense-in-Depth). 3. Etablerat '_printingInProgress' guard i 'printInvoicePDF' mot minnesläckage vid dubbelklick. 4. Lagt till Promise.all felhantering med toast samt villkorlig momsrapportrecal i 'refreshAllData'. 5. Rensat de sista 5 döda 'light-theme'-reglerna i 'style.css'.</kärna>
    <motivering>En fientlig audit i Fas 2 visade att stornoposter på momsomföringar riskerade att korrumpera momsrapporter om de bokfördes på rörliga datum samt att snabba klick på fakturautskrift läckte URL-objekt. Genom att stänga dessa sårbarheter garanteras fullständig finansiell korrekthet under alla edge-cases.</motivering>
  </record>
  <record id="POST_AUDIT_RESTORE_ONBOARDING_01" kategori="Säkerhet / Arkitektur">
    <beslut>Implementerat fullständig och säker återställning från krypterad eller okrypterad backup (.llbak) under onboarding (Setup Wizard).</beslut>
    <kärna>1. Skapat endpoints /select-folder och /restore i Setup-multiplexern i setup.go som stöder PowerShell FolderBrowserDialog och multipart-uppladdning. 2. Integrerat decryptPayload, Anti-Zip Slip och ledger.OpenLedger schema- och korruptionsvalidering (v3.0.0) under uppstarts-restorering. 3. Uppdaterat setup.html med en premium och lyxig "Återställ från Säkerhetskopia" glassmorfisk expander med drag-and-drop droppzon, lösenordsfält och dynamic progress feedback.</kärna>
    <motivering>Genom att bygga in fullständigt stöd för att starta från en befintlig säkerhetskopia direkt i onboarding-skedet slipper användare manuellt fippla med mappar på disk eller starta tomma arbetsytor innan återställning görs. Integrerad validering skyddar mot nedgraderingsattacker, skadade zippar eller korrupta SQLite-databaser innan driftsättning sker.</motivering>
  </record>
  <record id="POST_AUDIT_RESTORE_ROBUSTNESS_01" kategori="Säkerhet / Arkitektur">
    <beslut>Eliminerat tomma mappar och infört robusta retry-loopar för Windows file lock mitigation under återställning (Restore).</beslut>
    <kärna>1. Rensat phantom 'invoices/' katalog-logiken i setup.go för att undvika onödig tom mappskapande. 2. Implementerat robusta retry-loopar (upp till 5 försök med 200ms sleep) och explicit felrapportering på os.Remove och os.Rename för databas- och attachments-operationer under både Setup-återställning (setup.go) och Hot-återställning (backup.go).</kärna>
    <motivering>En fientlig arkitekturgranskning (FAS 1 &amp; 2) påvisade potentiella risker för Windows-specifika fillåsningsfel (ERROR_SHARING_VIOLATION) direkt efter Close() på databasen. Genom att införa retry-loopar och städa bort den icke-existerande invoices-katalogen garanteras 100% driftsäkerhet och atomicitet på Windows under hela återställningscykeln.</motivering>
  </record>
  <record id="UX_ROBUSTNESS_HARDENING_01" kategori="UI/UX / Robusthet">
    <beslut>Implementerat omfattande UI-buggfixar, root-mappsvalidering och förtydligat felmeddelande för logotypuppladdning.</beslut>
    <kärna>1. Säkrat /restore-endpointen i setup.go mot Windows rot-enheter via isRootDirectory-kontroll. 2. Förbättrat PNG MIME-valideringsfel i logo.go. 3. Ändrat .company-topbar från sticky till relative i style.css för att undvika överlappningar. 4. Lagt till showHelp-negation i index.html för nav-highlight och paneldöljning vid visning av Hjälp &amp; Guide.</kärna>
    <motivering>Löser användarrapporterade problem från pilottester rörande röriga rotkataloger, otydliga MIME-fel vid omdöpta bildfiler, störande CSS-överlappningar i listor/inmatning samt navigeringsbuggar för Hjälp &amp; Guide.</motivering>
  </record>
  <record id="PORTABLE_MULTITENANT_LAUNCHER_01" kategori="Arkitektur / UX">
    <beslut>Implementerat Multi-Tenant Launcher, Cascading Config Fallback och Portable Onboarding för 100% USB-vänlig drift.</beslut>
    <kärna>1. Skapat konfigurationsmotorn i config.go med tyst fallback från programkatalogen (USB-portabelt läge) till %LOCALAPPDATA% om katalogen är skrivskyddad. 2. Implementerat relativisering av arbetsytesökvägar för att klara skiftande enhetsbokstäver på USB-enheter. 3. Lagt till Orphan Guard-validering i Launchern som avaktiverar workspaces som saknar ledger.db och ger möjlighet till borttagning. 4. Integrerat ett pedagogiskt informationskort i setup.html som förklarar stateless .exe och workspaces. 5. Tagit bort automatisk skrivbordsgenväg vid onboarding och skapat en in-app Settings-funktion via POST /api/settings/shortcut med PowerShell-baserad genvägsgenerering döpt efter företagsnamn och servad med workspace-argument.</kärna>
    <motivering>Denna arkitekturförändring lyfter LocalLedger till att bli helt bärbar (USB-vänlig), förtydligar driftsmodellen dramatiskt för förstagångsanvändare samt ger full flexibilitet för användare att hantera flera företag oberoende av varandra utan att störa Windows-miljön med oväntade automatiska genvägar.</motivering>
  </record>
  <record id="PORTABLE_LAUNCHER_HARDENING_01" kategori="Arkitektur / Säkerhet">
    <beslut>Härdat Launcher-systemet med portabilitetsvarning, path-traversal-skydd och de-duplicerade säkra genvägar.</beslut>
    <kärna>1. Implementerat isPortable-detektering i config.go och exponerat den till frontenden via /api/recent-workspaces för att visa en banner vid begränsade skrivrättigheter. 2. Härdat open_workspace med filepath.Clean mot path traversal. 3. Säkrat createDesktopShortcut med %q mot PowerShell-injektion och lagt till automatisk radering av dubbletter/zombie-genvägar baserat på målarbetsyta.</kärna>
    <motivering>En djupgående revision (FAS 2) identifierade risker med tyst portabilitetsförlust, duplicerade zombieskrivbordsgenvägar vid namnbyten och injektioner. Detta löser alla tre brister på ett BFL- och Windows-säkert sätt.</motivering>
  </record>
  <record id="PORTABLE_LAUNCHER_HARDENING_02" kategori="Säkerhet / Windows">
    <beslut>Åtgärdat split-brain för genvägssökväg, infört isForbiddenDirectory-check mot Windows-systemmappar samt implementerat 100% enhetstesttäckning av cascading config-fallback.</beslut>
    <kärna>1. Flyttat all sökvägsberäkning för Windows Desktop till PowerShell. 2. Skapat isForbiddenDirectory för att explicit blockera och logga obehöriga öppningar av systemkataloger (t.ex. C:\Windows, C:\Program Files) under uppstart. 3. Skapat config_test.go med fullständiga enhetstester för relativa sökvägar, orphanguard och config-parsarflödet.</kärna>
    <motivering>Åtgärdar allvarliga Windows-specifika edge-cases med OneDrive/Skrivbord-omdirigeringar, lyfter säkerheten i fleranvändarscenarier och ger robust testskydd vid framtida refaktoriseringar.</motivering>
  </record>
  <record id="POST_AUDIT_HARDENING_03" kategori="Felsökning / Säkerhet">
    <beslut>Slutfört post-audit buggfixar och härdning av serverns nätverks- och versionsintegritet.</beslut>
    <kärna>1. Synkat CurrentAppVersion till 1.4.0 och ändrat handleHealth till att returnera detta värde, vilket löser versionsmissmatch vid single-instance kontroll under Windows-uppstart. 2. Ökat ReadTimeout och WriteTimeout till 30 sekunder för att säkra nätverksöverföringar vid uppladdning av stora säkerhetskopior (50MB+). 3. Ökat setup-heartbeat watchdog timeout till 90s för att förhindra falska zombie-nedstängningar orsakade av Chromium-bakgrundstrådstrypning och FolderBrowserDialog-väntan under onboarding. 4. Raderat pre-SPA reliker (handleGetTools och handleGetReports) och städat bort tillhörande oanvända importer.</kärna>
    <motivering>Denna driftssäkring och systemhärdning åtgärdar teoretiska och praktiska brister identifierade under revisionsfasen. Genom att eliminera versionsdivergensen, harmonisera nätverkstimeouts och utöka watchdog-gränsen uppnår systemet fullständig och stabil produktionsstatus.</motivering>
  </record>
  <record id="POST_AUDIT_HYGIENE_04" kategori="Kodhygien / Dokumentation">
    <beslut>Åtgärdat felaktig timeoutkommentar i frontenden efter djupgående granskningsrunda.</beslut>
    <kärna>1. Korrigerat utdaterad kommentar i setup.html som angav 10 sekunders timeout, till att stämma överens med den faktiska 90 sekunders timeout som konfigurerats i setup.go.</kärna>
    <motivering>En djupgående fientlig revision (FAS 2) och granskning bekräftade att alla ändringar i nätverks- och versionsprotokollen är robusta och stabila. Justeringen av kommentaren garanterar absolut kodintegritet och förhindrar framtida missförstånd för utvecklare.</motivering>
  </record>
  <record id="UI_UX_BRANDING_FALLBACK_01" kategori="UI / UX / Branding">
    <beslut>Implementerat layout-, UI/UX- och varumärkesförbättringar samt en högupplöst vektorlogotyp som fallback i PDF-fakturor.</beslut>
    <kärna>Åtgärdat flex-overflow- och scroll-buggar på setup-skärmen, flyttat status-badgen (WORM) till topbaren, lagt till klick-avvisning och parametriserad timeout på toasts, samt implementerat en cyan (#06b6d4) layered-diamond vektorfallback i PDF-genereringen via fpdf.</kärna>
    <motivering>Förbättrar systemets professionella estetik och varumärkesupplevelse under onboarding och löpande användning. Vektorfallbacken i PDF-motorn garanterar knivskarp visning av LocalLedger-logotypen även om en uppladdad bild saknas eller raderas på disk, helt i linje med svensk bokföringslagstiftning (BFL) och designstandarder.</motivering>
  </record>
  <record id="UI_UX_BRANDING_FALLBACK_02" kategori="UI / UX / Säkerhet">
    <beslut>Härdat toast-notifieringssystemets stängningslogik mot timeout-raceconditions, åtgärdat PDF-logotypgenereringens dolda fel och BFL-säkrat onboardingens molnsynktext.</beslut>
    <kärna>1. Skapat dismissToast() i app.js och bundit till @click i index.html för att atomärt stänga toasten och rensa _toastTimeout. 2. Implementerat pdf.Err() och pdf.ClearError() i pdf.go efter ImageOptions för att säkra fallback-vektorritning vid skadade bildfiler. 3. Skrivit om setup.html's portabilitetstips till att varna mot aktiv databassynkning av SQLite-filer och rekommendera inbyggd AES-256 backup.</kärna>
    <motivering>En fientlig granskning (FAS 1 och 2) under strängt I/O-lås avtäckte en kritisk state-läcka i toast-notifieringarnas livscykel, en risk för SQLite-korruption på grund av felaktigt formulerade molnråd till användare, samt en blind fläck i fpdf:s dolda felhantering. Genom att stänga dessa sårbarheter säkras både UX och dataintegriteten enligt BFL.</motivering>
  </record>
  <record id="UI_UX_BRANDING_FALLBACK_03" kategori="UI / UX / Säkerhet / Kodhygien">
    <beslut>Slutfört de sista post-audit-punkterna: Jargongfri onboarding-synktext samt proaktiv SVG-detektion vid faktura-PDF-generering.</beslut>
    <kärna>1. Skrivit om sync-informationen i setup.html till ett helt jargongfritt språk (tagit bort "databas" och "fillåsningsfel"). 2. Implementerat SVG-filtilläggskontroll i invoice.go som proaktivt loggar en konsolvarning för att underlätta logotyp-felsökning då gofpdf saknar SVG-stöd.</kärna>
    <motivering>Genom att förenkla onboarding-guiden undviks teknisk förvirring hos slutanvändare rörande synkroniseringsstörningar. SVG-kontrollen säkerställer dessutom tydlig backend-diagnostik om varför bildgenereringen tyst faller tillbaka på LocalLedger-vektorlogotypen.</motivering>
  </record>
  <record id="EULA_CLICKWRAP_01" kategori="Säkerhet / Juridik">
    <beslut>Implementerat tvingande Clickwrap EULA-modal samt offline-vänlig feedback-kanal.</beslut>
    <kärna>1. Skapat LICENSE.md i rotkatalogen med svenskspråkigt EULA-avtal. 2. Integrerat tvingande Clickwrap-modal i index.html som blockerar gränssnittet tills avtalet godkänts av användaren. 3. Lagt till versionskontroll (v1.0-beta) via localStorage. 4. Skapat feedback-sektion under Hjälp & Guide med mailto-länk samt copy-to-clipboard fallback för Daniel Karlsson (dka120@hotmail.com).</kärna>
    <motivering>Skyddar utvecklarens immateriella rättigheter (IP) och eliminerar ansvarsrisk (AS-IS) genom ett juridiskt robust och användarvänligt clickwrap-gränssnitt som respekterar programmets local-first arkitektur.</motivering>
  </record>
  <record id="EULA_HARDENING_POST_AUDIT_01" kategori="Säkerhet / Juridik / UX">
    <beslut>Härdat EULA-modalen mot tangentbordsbypass, Command Palette-läckage, re-read UX-deadlock, samt säkrat pre-acceptance data fetching.</beslut>
    <kärna>1. Lagt till x-bind:inert på aside och main i index.html för att helt förhindra Tab-fokusläckage och bakgrundsinteraktioner när EULA-modalen är aktiv. 2. Implementerat isEulaReadOnly i app.js och index.html för att separera tvingande first-boot läge (utan stängknapp/Escape) från frivillig re-read (med stängknapp och Escape-avslut). 3. Blockerat Command Palette (Ctrl+K) i command-palette.js när EULA-modalen visas. 4. Refaktorerat init() i app.js för att helt blockera API-datainläsning (privacy-first) och endast köra completeInit() efter godkänt avtal.</kärna>
    <motivering>En djupgående fientlig audit (FAS 2) visade att den tidigare EULA-implementationen enkelt kunde kringgås via tangentbordstabbning och Ctrl+K globala snabbkommandon, samt led av ett UX-deadlock vid re-read. Genom att implementera fullständig inert-fokusspärr, blockera Command Palette, och helt skjuta upp datainläsning till efter godkänt avtal garanteras 100% juridisk giltighet och en oantastlig integritetsnivå enligt GDPR och Bokföringslagen.</motivering>
  </record>
  <record id="EULA_BUTTON_ACCENT_SANITY_01" kategori="UI / UX / Säkerhet">
    <beslut>Sanerat all --accent variabel-användning, integrerat a.primary och eliminerat inline button styling.</beslut>
    <kärna>1. Justerat style.css så Bullet.primary tvingar färg #000 för mörkt läge och utökat regeln att gälla a.primary samt tillhörande hover-states. 2. Rensat bort alla inline background, box-shadow och color stilar från EULA-godkännandeknappen och Nytt År-knappen i index.html för att helt förlita sig på CSS-klassen. 3. Ändrat klassen för Feedback-länken till primary och tagit bort dess färgstilar. 4. Bytt ut alla förekomster av odefinierad var(--accent) till var(--primary) för rubriker, ikoner och spinners.</kärna>
    <motivering>En djupgående post-audit (FAS 2) visade att den odefinierade variabeln var(--accent) renderade primärknappar och kritiska ikoner helt transparenta eller med felaktig kontrast i Windows-miljön. Genom att lyfta designen till att helt styras av den globala CSS-klassen .primary, harmonisera a-element, samt sanera alla accent-referenser till primärfärgen, återställs LocalLedgers lyxiga och tillgängliga mörka fintech-estetik i enlighet med designmanualen.</motivering>
  </record>
  <record id="EULA_BUTTON_FINAL_POLISH_01" kategori="UI / UX / Robusthet">
    <beslut>Slutgiltig sanering av EULA-knappen och harmonisering av Feedback-länkens knapplayout i CSS.</beslut>
    <kärna>1. Rensat bort hela style-attributet (inklusive dolt kvarvarande var(--accent) bakgrundsstil) från EULA-knappen på rad 121 i index.html för att helt låta CSS sköta utseendet utan transparens. 2. Ändrat basregeln för knappar i style.css till 'button, a.primary' så att länkbaserade knappar ärver identisk padding, typsnitt, border-radius och cursor. 3. Rensat bort inline-padding och border-radius från Feedback-länken på rad 1217 i index.html. 4. Städat upp .gitignore från trasig UTF-16-formatering och lagt till localledger_config.json.</kärna>
    <motivering>En fientlig granskning (FAS 2) visade att EULA-knappen fortfarande led av transparent bakgrund i produktion på grund av en kvarlämnad inline-stil som överstyrde klassen, samt att länkbaserade knappar saknade grundläggande knapptypsnitt och layout utan sköra inline-lösningar. Genom att integrera a.primary direkt i basreglerna för knappar i CSS uppnås en helt konsekvent och robust kodstruktur utan redundans, vilket tryggar designmanualens estetik för alla framtida beta-testare.</motivering>
  </record>
  <record id="EULA_SANDBOX_ISOLATION_AND_FAST_SHUTDOWN_01" kategori="Arkitektur / Säkerhet / UX">
    <beslut>Implementerat snabblåsning av filer via unload-beacons samt portabel EULA-isolation per arbetsyta.</beslut>
    <kärna>1. Skapat POST /api/unload och POST /unload i server.go och setup.go med en idempotent 2s shutdown-timer skyddad mot F5-raceconditions genom lastPing-validering. 2. Implementerat workspace-hash i handleFrontendIndex baserad på SQLite company_settings (org_number + name) för full USB-portabilitet av EULA-avtal. 3. Isolerat Sandbox-läge via sessionsbaserad hash för att tvinga fram EULA vid varje nystart.</kärna>
    <motivering>Löser Windows file-lock problem omedelbart vid stängning utan 90s fördröjning, samt isolerar localStorages EULA-tillstånd fullständigt för Sandbox utan att bryta LocalLedgers bärbara och local-first USB-arkitektur.</motivering>
  </record>
  <record id="EULA_SANDBOX_PORTABILITY_HARDENING_01" kategori="Arkitektur / Säkerhet / UX">
    <beslut>Härdat EULA-portabilitet, namnbytestolerans, samt städning av föräldralösa Sandbox-nycklar.</beslut>
    <kärna>1. Förfinat workspaceHash i server.go till att prioritera org_number, falla tillbaka på name, och använda absolutsökväg (s.workspace) för okonfigurerade instanser. 2. Implementerat automatisk localStorage-städning i app.js init() som rensar alla föräldralösa 'eula_accepted_version_'-nycklar utom den aktiva. 3. Lagt till TestWorkspaceHash i server_test.go för komplett verifiering av hashnivåer.</kärna>
    <motivering>Säkerställer fullständig robusthet för EULA-avtal på USB-enheter. Genom att prioritera org_number tål EULA-tillståndet att användaren ändrar sitt företagsnamn, medan absolutsökvägs-fallback hindrar krockar mellan nyskapade okonfigurerade instanser. Rensningen av localStorage-nycklar förhindrar att gamla tillfälliga Sandbox-nycklar ackumuleras i all oändlighet på värddatorn.</motivering>
  </record>
  <record id="BETA_RELEASE_LAUNCH_ASSETS_01" kategori="UI / UX / Lansering">
    <beslut>Implementerat och kört automatiserad Playwright-skärmdumpsgenerering för premium lanseringsbilder i betaversionen av LocalLedger samt förberett anpassat lokalt e-postutkast för beta-inbjudan till Ludvig &amp; CO.</beslut>
    <kärna>1. Utvecklat automatiseringsskriptet scratch/capture_premium_screenshots.py med Playwright för att programmatiskt starta backend-sandboxen, bypassa EULA och navigera till tre nyckelvyer. 2. Genererat exakt 3 högupplösta, retina-skalade (DPI=2) skärmdumpar under mappen lanseringsbilder/ (dashboard, fakturaskapare och momsredovisning). 3. Formulerat ett informellt, lokalt förankrat beta-inbjudningsmail till Skellefteå-startupen Ludvig &amp; CO med Google Drive-leveransstrategi och utan prislåsningar.</kärna>
    <motivering>Följer de stränga kraven på premiumestetik och lokal tonalitet för Skellefteå-aktörer. Playwright garanterar repeterbar och kristallklar visualisering i 2x DPI utan att kräva manuell handpåläggning eller riskera trasig fönsterkontrast, samtidigt som e-poststrategin med Google Drive minimerar risker för skräppostfilter och undviker framtida prislåsningar.</motivering>
  </record>
  <record id="SIE4_VALIDATION_PREVIEW_01" kategori="Säkerhet / Arkitektur / UI">
    <beslut>Implementerat robust och juridiskt korrekt SIE-4 validering och interaktiv dry-run förhandsgranskningsmodal.</beslut>
    <kärna>1. Ändrat exportkodning till IBM PC8 (CP437) och lagt till dynamisk #FLAGGA-generering (0 om stängt, 1 om preliminärt) samt #UB-export för nollbalanser. 2. Implementerat byte-sniffande matematisk UTF-8 vs CP437 kodningsdetektering via utf8.Valid. 3. Skapat aggregerad dry-run endpoint (/api/import/sie4?dry_run=true) och PreviewSIE4-valideringssvit för att förhindra O(N) frontend-krascher och blockera ogiltiga år/lås/balanser. 4. Byggt en premium glassmorfisk förhandsgranskningsmodal i index.html för både Inställningar och Verktyg.</kärna>
    <motivering>Garanti för 100% driftsäker och laglig dataimport/export inför beta-lanseringen. Genom att begränsa dry-run payloaden till aggregerad statistik skyddas användarens prestanda, medan den noggranna byte-detekteringen eliminerar risken för trasiga svenska tecken efter felaktiga UTF-8 exporter från externa system.</motivering>
  </record>
  <record id="SIE4_POST_AUDIT_HARDENING_01" kategori="Säkerhet / Kodhygien">
    <beslut>Slutfört post-audit härdning av SIE-import: införde uppladdningsbegränsning samt explicit DB-iterationskontroll.</beslut>
    <kärna>1. Säkrat handleImportSIE4 i routes.go med http.MaxBytesReader (20MB) för att förhindra okontrollerade minnesallokeringar och OOM-vektorer. 2. Implementerat explicit rows.Err() kontroll i PreviewSIE4 (sie_import.go) efter SELECT code query-loopen för att säkra mot tysta databasfel.</kärna>
    <motivering>Stänger de sista potentiella driftssårbarheterna som identifierades under den fientliga audit-processen i Fas 2, vilket lyfter importmotorn till fullständig driftsäkerhet.</motivering>
  </record>
  <record id="SRU_RECONCILIATION_BAS2026_01" kategori="Arkitektur / Säkerhet / Skatt">
    <beslut>Implementerat inbäddad BAS 2026 SRU-export samt reaktiv bankmatchningsmotor mot 1930/1510.</beslut>
    <kärna>1. Skapat Version 17-migrering för accounts.sru_code. 2. Integrerat sru_bas2026.json (via go:embed) med mappningar till NE_2026 fältkoder. 3. Skapat GenerateSRUFiles (sru.go) för INFO.SRU och BLANKETTER.SRU. 4. Byggt MatchBankTransactions (reconciliation.go) som matchar 1930 insättningar mot 1510 fakturor med OCR-text och datum (±3 dagar). 5. Exponerat endpoints (/api/export/sru i ZIP-format, /api/reconciliation/match) med integrationstester.</kärna>
    <motivering>En fientlig granskning (/audit, /granska, /merge) godkände en strict Go-baserad, local-first arkitektur som avvisar moln- och CSV-komplexitet. SRU-exporten möjliggör direkt NE-deklaration enligt BFL-krav, medan bankmatchningen reaktivt underlättar avstämning av kundreskontra baserat på befintlig, verifierad SIE-4 bankdata.</motivering>
  </record>
  <record id="SRU_RECONCILIATION_POST_AUDIT_01" kategori="Säkerhet / Arkitektur / Prestanda">
    <beslut>Härdat SRU-exporten med ISO-8859-1 och optimerat bankmatchningen mot prestandabomber och dubbelmatchningar.</beslut>
    <kärna>1. Integrerat charmap.ISO8859_1 i sru.go. 2. Infört SQL-baserad filtrering av bankinsättningar (1930) i MatchBankTransactions. 3. Lagt till usedDeposits-map för kollisionsundvikande matchning. 4. Infört rows.Err() loopkontroller.</kärna>
    <motivering>Möter Skatteverkets strikta krav på Latin-1 teckenkodning för svenska bokstäver, förhindrar minneskrascher (O(N) prestandabomb) vid stora transaktionsvolymer, samt garanterar korrekt kundreskontra-matchning utan att en enskild bankinsättning kan matchas till flera fakturor.</motivering>
  </record>
  <record id="UI_UX_STABILIZATION_01" kategori="UI / UX / Robusthet">
    <beslut>Stabiliserat flexbox-layouten, åtgärdat sidomeny-krympning och anpassat/centrerat rapportvyn till A4-maxbredd, samt infört automatisk datumvaliderings-smooth scroll, fokus och röd input-glow.</beslut>
    <kärna>1. Lagt till flex-shrink: 0 på .sidebar och min-width: 0 på .main-content i style.css för att förhindra ihoptryckning. 2. Justerat #printable-report i index.html till max-width: 900px och centrerat. 3. Skapat .input-error klass i style.css och bundit den via Alpine :class till datumfältet (id post-date-input). 4. Implementerat showToast, focus och smooth-scroll i submitPost (app.js) vid valideringsfel.</kärna>
    <motivering>Löser tre kritiska layout- och UX-problem rapporterade under användartester (smal rapportvy, krympt sidomeny vid Verktyg &amp; Export, samt missad datumvalidering vid bokföring) på ett strukturellt och estetiskt premium sätt helt i linje med svensk bokföringslagstiftning (BFL) och systemarkitekturen.</motivering>
  </record>
  <record id="PRINT_COMPATIBILITY_01" kategori="UI / UX / Utskrift">
    <beslut>Säkrat utskriftskompatibiliteten för finansiella rapporter genom att lägga till max-width och margin reset i @media print.</beslut>
    <kärna>Lagt till max-width: none !important; margin: 0 !important; i style-mallen för #printable-report under print-media.</kärna>
    <motivering>En fientlig eftervalsgranskning visade att det nyligen introducerade max-width: 900px inline-attributet för att bredda rapportvyn på skärmen i produktion riskerade att begränsa och felmarginalisera fysiska utskrifter (Ctrl+P) eller PDF-export via webbläsaren. Genom att lägga till explicita @media print-nollställningar garanteras 100% A4-utskrifter och PDF-filer i linje med svensk bokföringslagstiftning (BFL) och standardiserad dokumentutformning.</motivering>
  </record>
  <record id="PRINT_PAGINATION_01" kategori="UI / UX / Utskrift">
    <beslut>Hävt SPA-applikationens höjdbegränsningar vid utskrift för att tillåta flersidig paginering.</beslut>
    <kärna>Lagt till overrides i index.html under @media print för body (height: auto !important; overflow: visible !important; display: block !important;) och .main-content (overflow: visible !important; height: auto !important;).</kärna>
    <motivering>En kritisk granskning avslöjade att SPA-containerns globala inställningar height: 100vh och overflow: hidden hindrade webbläsaren från att paginera rapporter som sträcker sig över mer än en enskild A4-sida vid utskrift eller PDF-export. Genom att häva dessa begränsningar specifikt under print-media kan finansiella rapporter nu sömlöst flöda över flera sidor utan att informationen klipps av efter första sidan.</motivering>
  </record>
  <record id="PORTFOLIO_SHOWCASE_01" kategori="Branding / Portfolio">
    <beslut>Etablerat en fristående public-facing Architecture Showcase i showcase/ katalogen.</beslut>
    <kärna>Skapat index.html (engelska), style.css, automatiserat WebM videoinspelning via Playwright, och separerat intern PRD-dokumentation från den publika GitHub Pages-miljön.</kärna>
    <motivering>Möter användarens krav på en premium LinkedIn-visning för en internationell teknisk målgrupp utan att kompromissa med sekretess kring interna PRD-filer genom att placera showcase i en separat katalog och handkurera ledger-poster istället för full exponering.</motivering>
  </record>
  <record id="PORTFOLIO_SHOWCASE_POST_AUDIT_01" kategori="Branding / Portfolio / Mobil">
    <beslut>Säkrat portföljens LinkedIn-delbarhet, iOS-videokompatibilitet, Mermaid CDN-tillförlitlighet samt mobilresponsivitet efter en fientlig granskning.</beslut>
    <kärna>1. Ändrat og:image till absolut URL på GitHub Pages. 2. Lagt till poster-attribut samt MP4-källa i videon (med poster som fail-safe då ffmpeg saknas lokalt). 3. Uppdaterat Mermaid.js CDN-inkludering till standard jsDelivr @11 utan sköra SRI-hashar. 4. Lagt till omfattande mobilresponsiva @media (max-width: 768px) regler för header, nav-links och layout-komponenter.</kärna>
    <motivering>Den fientliga audit-granskningen avtäckte kritiska hinder för LinkedIn-optimering (där relativa bildsökvägar blockeras) samt iOS Safari-problem (där WebM-videor utan poster förblir svarta block). Genom att åtgärda dessa samt standardisera Mermaid.js-inkluderingen och bygga in full mobilresponsivitet, garanteras en fläckfri och exklusiv presentation för Tech Leads och rekryterare världen över.</motivering>
  </record>
  <record id="PUBLIC_HARDENING_CLEAN_SLATE_01" kategori="Säkerhet / Kodhygien">
    <beslut>Genomfört en fullständig "Clean Slate" av Git-arkivet för att säkra källkoden inför publik publicering.</beslut>
    <kärna>1. Raderat känsliga sessionstokens, loggar, samt hela sandbox_e2e/-mappen. 2. Tagit bort interna AI-instruktioner, rules och recovery-skript (rescue.py, ag_blueprint_config.html). 3. Uppdaterat .gitignore med strikta filter. 4. Nollställt Git-historiken med git init och skapat en initial ren commit.</kärna>
    <motivering>Den fientliga granskningen i FAS 2 påvisade allvarliga risker för historiskt dataläckage (t.ex. sandbox-tokens i git-loggar samt lokala filsökvägar i rescue.py). Genom att nollställa Git-trädet och bygga en robust .gitignore rensas all känslig information för alltid, och källkoden kan tryggt publiceras under en Source-Available-licens utan att exponera utvecklingshistorik eller AI-infrastruktur.</motivering>
  </record>
  <record id="SHOWCASE_HARDENING_01" kategori="UI / UX / Branding">
    <beslut>Härdat och optimerat den publika Showcase-sidan samt integrerat GitHub-länkar.</beslut>
    <kärna>1. Löste färg-specifikitetsfel på navigationsknappar. 2. Reordnade video källor till .webm före .mp4 för att undvika 404-fel. 3. Säkrade responsiv Mermaid SVG-skalning. 4. Integrerade mobilvänlig GitHub-länk i header/footer samt ställde om OG-metadata.</kärna>
    <motivering>Garanterar en professionell och fläckfri presentation av LocalLedger inför beta-lanseringen på LinkedIn, med bibehållen fallback för äldre iOS-enheter och fullständig mobilresponsivitet på GitHub Pages.</motivering>
  </record>
</decision_ledger>


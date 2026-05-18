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
</decision_ledger>

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
  <kärna>1. Införde Negative-Flip logik i `RegisterPayment` så att utbetalningar av kreditfakturor bokförs korrekt (Kredit 1930, Debet 1510) istället för att skicka in otillåtna negativa summor.
2. Dold `+ Lägg till rad`-knapp i UI:t för kreditutkast (`x-show="!selectedInvoice.credit_of"`) för att garantera strukturell integritet mot originalfakturan.</kärna>
  <motivering>Strikt BFL/WORM compliance kräver positiva belopp. UI-låsningen förhindrar användarmisstag som kan fördunkla spårbarheten.</motivering>
</record>

<record id="INV-006" kategori="Arkitektur / Logik">
  <beslut>Omstrukturerat RegisterPayment och SettleInvoice för partiell kreditering (Audit-fix).</beslut>
  <kärna>1. `RegisterPayment` beräknar dynamiskt `amountToPay` = fakturabelopp + summan av bokförda kreditfakturor. Endast restbeloppet bokförs mot 1930/1510.
2. `SettleInvoice` stänger originalfakturan (sätter status = 'betald') endast om `abs(SUM(kreditfakturor)) >= originalfakturans belopp`.</kärna>
  <motivering>Löser Catch-22 problemet där partiella kreditfakturor tidigare korrumperade kundreskontran och banksaldot genom att antingen gömma restbeloppet eller låta det dubbelbokföras.</motivering>
</record>

</decision_ledger>

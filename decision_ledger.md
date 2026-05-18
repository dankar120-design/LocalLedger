<decision_ledger>
  <record id="PHOENIX_PROTOCOL_01" kategori="Arkitektur">
    <beslut>Återskapa Setup Wizard och Sandbox-motor med Vanilla JS och sync port binding.</beslut>
    <kärna>Övergång från CDN-beroende AlpineJS till Vanilla JS för on-boarding, infört go:embed för SQL/HTML, och implementerat heartbeat för att förhindra Zombie-processer.</kärna>
    <motivering>En kritisk förlust av kod (`git clean -fd`) krävde ett återskapande. Den nya arkitekturen eliminerar helt frontend-byggsteg och säkerställer Single-Instance säkerhet i Windows.</motivering>
  </record>
  <record id="WORM_COMPLIANCE_01" kategori="Säkerhet">
    <beslut>Implementera WORM (Write Once Read Many) via SQLite Triggers.</beslut>
    <kärna>Inga verifikationer med en kryptografisk hash får raderas eller uppdateras. Skyddet är inbakat direkt i SQLite-schemat via BEFORE UPDATE/DELETE triggers.</kärna>
    <motivering>För att uppfylla svensk Bokföringslag (BFL) krävs oföränderlighet. Genom att lägga skyddet i databasen skyddar vi datan även om applikationslogiken (Go) skulle manipuleras.</motivering>
  </record>
  <record id="SINGLE_INSTANCE_01" kategori="Arkitektur">
    <beslut>Nätverksport-mutex med Health-Ping för Single-Instance på Windows.</beslut>
    <kärna>Istället för komplexa Windows Mutex-anrop binder appen till TCP 8080. Vid kollision (EADDRINUSE) görs ett HTTP GET-anrop. Om appen svarar (rätt version), öppnas enbart webbläsaren.</kärna>
    <motivering>Garanterar att användaren inte av misstag kör två databas-instanser parallellt, vilket kunde orsakat SQLite-låsningar, samt eliminerar beroenden till OS-specifika C-bibliotek för Mutex.</motivering>
  </record>
  <record id="BROWSER_FALLBACK_01" kategori="Edge-case">
    <beslut>Kaskad-start av webbläsare för Desktop-känsla.</beslut>
    <kärna>Appen försöker starta msedge.exe --app=, sedan chrome.exe --app=, och faller till sist tillbaka på rundll32 (standardwebbläsare).</kärna>
    <motivering>Skapar illusionen av en infödd desktop-applikation (utan URL-bar och flikar) på Windows-system, utan att behöva inkludera en hel Electron/WebView2-runtime i binären.</motivering>
  </record>
  <record id="SETUP_HTML_EMBED" kategori="Arkitektur">
    <beslut>Strikt användning av go:embed för Setup-vyerna.</beslut>
    <kärna>setup.go läser nu setup.html från frontend.FS istället för os.ReadFile.</kärna>
    <motivering>Garanterar att applikationen förblir en enskild portabel binärfil utan risk för att sakna filer om den flyttas.</motivering>
  </record>
  <record id="ZOMBIE_HEARTBEAT_CONTEXT" kategori="Felsökning">
    <beslut>Eliminering av race condition i heartbeat via context.WithCancel.</beslut>
    <kärna>Setup-servern stängs ner via en dedikerad context-signal från main-tråden, istället för att tävla om data på resultChan.</kärna>
    <motivering>Förhindrar att heartbeat-tråden oavsiktligt konsumerar resultatet och skapar oändliga hängningar (Zombie-processer).</motivering>
  </record>
  <record id="SQLITE_DRIVER_NAME" kategori="Felsökning">
    <beslut>Korrigering av SQLite drivrutinsnamn till "sqlite".</beslut>
    <kärna>sandbox.go anropar nu sql.Open("sqlite", ...) för att matcha modernc.org/sqlite.</kärna>
    <motivering>Förhindrar runtime-krascher vid Sandbox-start eftersom mattn/go-sqlite3 ("sqlite3") inte ingår i detta projekt.</motivering>
  </record>
  <record id="SYNC_PORT_BIND" kategori="Felsökning">
    <beslut>Synkron port-bindning för Setup-servern innan webbläsaren startas.</beslut>
    <kärna>Användning av net.Listen innan srv.Serve anropas, med fallback-hantering i main.go.</kärna>
    <motivering>Förhindrar uppkomsten av dubbla process-krascher och falska webbläsarfönster om användaren dubbelklickar på .exe-filen flera gånger.</motivering>
  <record id="CACHE_CONTROL_SPA" kategori="Felsökning">
    <beslut>Införa strikta cache-busting-huvuden i Go-servern för frontend-tillgångar.</beslut>
    <kärna>Frontend-servern sätter nu Cache-Control till "no-store, no-cache, must-revalidate, max-age=0" för index.html och alla statiska JS/CSS-filer.</kärna>
    <motivering>Förhindrar att webbläsare cachar trasiga, avbrutna eller trunkerade JS-tillgångar efter misslyckade byggförsök, vilket säkerställer omedelbar synkronisering av frontend-ändringar.</motivering>
  </record>
  <record id="ALPINE_REACTIVITY_STABILIZATION" kategori="Felsökning">
    <beslut>Deklarera alla reaktiva tillstånd på föräldranivå och ersätt nästlade getters med watchers.</beslut>
    <kärna>Initierat showTools, showReports och rapporttillstånd i ledgerApp (app.js), deklarerat ibTargetYearName som standardfält i tools.js och bundit uppdateringar till en reaktiv watcher i init().</kärna>
    <motivering>Eliminerar Alpine.js kompilerings- och ReferenceError krascher som uppstår när nestade komponenter refererar till odefinierade fält under uppstartsfasen.</motivering>
  </record>
  <record id="DESKTOP_SHORTCUT_ONBOARDING" kategori="Arkitektur">
    <beslut>Automatiskt skapande av skrivbordsgenväg (.lnk) på Windows.</beslut>
    <kärna>Setup-servern kör nu ett powershell-skript via exec.Command för att generera en genväg på skrivbordet till den nuvarande körande exe-filen vid slutförd onboarding.</kärna>
    <motivering>Ersätter manuell länkhantering och förbättrar användarupplevelsen genom att ge en sömlös integration på Windows-skrivbordet.</motivering>
  </record>
</decision_ledger>
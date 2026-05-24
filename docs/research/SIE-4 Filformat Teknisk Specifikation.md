# **Djupgående Teknisk Analys och Arkitektonisk Sammanställning av Filformatet SIE-4 (Typ 4\)**

Denna forskningsrapport utgör en fullständig, tekniskt uttömmande och arkitektonisk djupdykning i det svenska filformatet SIE-4, med ett primärt och detaljerat fokus på Typ 4 (Transaktionsfil) och Typ 4I (Import av transaktioner). Filformatet, som kontinuerligt förvaltas av Föreningen SIE-Gruppen, utgör den absoluta de facto-standarden för överföring av redovisningsdata mellan olika programvaror på den svenska marknaden.1 Trots dess ålder och dess tekniska rötter i tidig PC- och DOS-historia, förblir SIE-4 centralt och fundamentalt för svensk finansiell infrastruktur, revision och systemintegration. Dess dominans beror till stor del på dess enkelhet, kompakta storlek och en närmast hundraprocentiga marknadstäckning bland bokförings-, skatte- och löneprogram.1  
Denna sammanställning är specifikt utformad för systemarkitekter, mjukvaruutvecklare och integrationsspecialister som har till uppgift att bygga moderna system, såsom molnbaserade affärssystem, försystem för lön och fakturering, eller AI-drivna analysverktyg, vilka kräver strikt, feltolerant och bakåtkompatibel integration mot SIE-standarden.1 Analysen syntetiserar den officiella tekniska specifikationen och konfronterar den med de verkliga utmaningar som uppstår när äldre standarder möter moderna, distribuerade mjukvaruarkitekturer.

## **1\. Filformatets Historiska Kontext och det Ekosystematiska Landskapet**

SIE (Standard Import Export) introducerades ursprungligen den 1 juli 1992\.4 I sin allra första inkarnation (SIE Typ 1\) var formatet uteslutande inriktat på överföring av årssaldon från redovisningsprogram till skatteprogram för att underlätta deklarationsarbetet.4 Formatet rönte snabbt en enorm popularitet tack vare dess omedelbara nytta och låga tröskel för implementation. I takt med att integrationsbehoven mellan olika verksamhetssystem växte, utökades standarden successivt. Den kompletterades med strukturer för periodsaldon (Typ 2), objekt- och flerdimensionell redovisning (Typ 3\) och slutligen med det format som kommit att bli branschstandarden för fullständig dataportabilitet: transaktionsformatet Typ 4\.4  
I det svenska redovisningslandskapet samverkar SIE-formatet exceptionellt tätt med BAS-kontoplanen. Föreningen SIE-Gruppen utgör tillsammans med BAS (som ansvarar för den svenska standardkontoplanen) och XBRL Sweden (som ansvarar för modern finansiell rapportering) de tre bärande pelarna för standardisering av bokföring, finansiella rapporter och myndighetsrapportering i landet.1 Den tekniska och pragmatiska styrkan i SIE-4 ligger i att det är ett rent, taggat textfilformat, vilket står i skarp kontrast till de mer omständliga XML-baserade formaten som XBRL GL eller det nyare försöket SIE-5.2 Avsaknaden av en tungrodd taxonomi och det faktum att en genomsnittlig SIE-4 fil är ungefär 20 gånger mer kompakt än motsvarande XML-format, gör det extremt resurseffektivt vid överföring av miljontals transaktionsrader från massiva affärssystem.2 Denna kompakthet är direkt kritisk i system där minnesbegränsningar och nätverkslatens påverkar prestandan.

### **Varianter av SIE-formatet och deras Specifika Tillämpningsområden**

För att förstå SIE-4 till fullo måste man placera det i kontexten av de andra formattyperna. Den officiella specifikationen definierar flera olika typer, var och en avsedd för en specifik nivå av informationsdjup 1:

* **SIE Typ 1 (Årssaldon):** Innehåller uteslutande ingående och utgående saldon för alla konton i kontoplanen.1 Denna typ används primärt för historiska syften och förklaringar till skatteprogram, och rekommenderas generellt inte för nyutveckling där högre granularitet förväntas.5  
* **SIE Typ 2 (Periodsaldon):** Innehåller all information från Typ 1, men adderar därtill saldoförändringar per månad för varje enskilt konto, samt periodbudgetar.1 Denna typ är ofta tillräcklig för enklare analysprogram som inte behöver borra ner till verifikationsnivå.  
* **SIE Typ 3 (Objektsaldon):** Utökar Typ 2 genom att introducera dimensioner och objektredovisning, vilket innebär att saldon kan redovisas för specifika kostnadsställen eller projekt.1  
* **SIE Typ 4 (Transaktioner / 4E):** Den kompletta exporten. Innehåller allt från Typ 1 till Typ 3, med det fundamentala tillägget att alla enskilda verifikationer under räkenskapsåret inkluderas.1 Detta är formatet som krävs vid datormässiga revisioner (Computer Assisted Audit Techniques) och vid migrationer mellan olika kompletta affärssystem.5  
* **SIE Typ 4I (Import av transaktioner):** En avskalad variant av Typ 4\. Syftet med detta format är uteslutande att göra det möjligt för försystem (t.ex. ett separat lönesystem, ett faktureringssystem eller ett kassaregister) att generera verifikationer som sedan importeras till huvudbokföringssystemet.1 I en Typ 4I-fil utelämnas ofta kompletta saldon, och fokus ligger helt på de nyskapade \#VER- och \#TRANS-posterna.5

Föreningen SIE-Gruppen rekommenderar starkt att alla nyskrivna system som ska exportera bokföringsdata som ett minimum stöder SIE Typ 2, men i praktiken är Typ 4 den odiskutabla normen för att systemet ska anses vara fullvärdigt på den svenska marknaden.5

## **2\. Teckenkodningens Arkitektur: Arvet från DOS och Moderna Paradigmskiften**

Den tveklöst största och överlägset vanligaste fallgropen vid modern mjukvaruimplementering av SIE-4 rör textrepresentation och teckenkodning. Eftersom formatet standardiserades långt innan Unicode och i synnerhet UTF-8 (vilket idag utgör över 99 % av all elektronisk textkommunikation på webben) blev normen, bär SIE med sig strikta, tvingande restriktioner kopplade till tidiga IBM PC-system.6 Mjukvaruutvecklare som inte noga studerar denna detalj kommer obönhörligen att orsaka datakorruption när information exporteras till äldre, strikta system.

### **Den Strikta Standarden: Codepage 437 (IBM PC8)**

Den officiella SIE-standarden medger absolut noll flexibilitet på denna punkt: formatet tillåter formellt och strikt endast en teckenkodning, nämligen **IBM PC 8-bits extended ASCII**, mer känt inom utvecklarkretsar som **Codepage 437 (CP437)**.5  
Denna teckenkodning måste deklareras i filen via den obligatoriska taggen \#FORMAT PC8.5 Codepage 437 är en 8-bitars teckenkodning som maximalt kan representera 256 olika tecken.8 De första 128 tecknen (0x00 till 0x7F) är helt identiska med standard-ASCII, vilket innebär att engelska bokstäver, siffror och vanliga interpunktionstecken kodas precis som i nästan alla andra moderna teckenset.8 Det är i den övre halvan (0x80 till 0xFF) som avvikelserna uppstår. Här placerade IBM i början av 1980-talet västeuropeiska tecken, grekiska bokstäver och en rad numera förlegade grafiska ritsymboler som användes för att rita fönster i DOS-gränssnitt.8

### **Den Tekniska Utmaningen med Svenska Tecken (Å, Ä, Ö) i CP437**

I moderna system kodas text nästan uteslutande i UTF-8, en variabel-längdskodning som stöder samtliga över en miljon definierade Unicode-tecken.7 I UTF-8 lagras de svenska tecknen å, ä och ö med två bytes vardera (exempelvis lagras 'ä' som bytesekvensen 0xC3 0xA4).7  
Om ett modernt webbaserat ERP-system exporterar en byteström innehållande UTF-8 direkt till en textfil och deklarerar den som \#FORMAT PC8, kommer det mottagande, strikt överensstämmande redovisningssystemet att tolka varje enskild byte som ett separat tecken enligt CP437-tabellen.11 Sekvensen 0xC3 0xA4 kommer då att översättas till två grafiska ritsymboler eller främmande tecken (så kallad "mojibake") istället för ett 'ä'.9 Detta leder inte bara till estetiska problem; om kontonamn (\#KONTO) eller objektsnamn (\#OBJEKT) korrumperas, försvåras både revision och sökbarhet avsevärt, och maskinell analys omöjliggörs.11  
För att ett nyskrivet system ska hantera svenska tecken korrekt vid export, **måste strängarna i arbetsminnet konverteras explicit från UTF-8 till CP437** innan byteströmmen skrivs till disk eller nätverkssocket.11 Den exakta mappningen för de kritiska svenska tecknen i CP437 är fastställd enligt följande tabell, vilken måste implementeras i systemets serialiseringslager:

| Tecken (Grafem) | Hexadecimal representation (CP437) | Decimalt värde (CP437) | Motsvarande Unicode-kodpunkt | Exempel på praktisk kontext i SIE |
| :---- | :---- | :---- | :---- | :---- |
| å | 0x86 | 134 | U+00E5 | Förekommer extremt frekvent i fältet \#FNAMN och i allmänna beskrivningar. |
| ä | 0x84 | 132 | U+00E4 | Standard i verifikationstexter, exempelvis under taggen \#VER (t.ex. "Lön", "Försäljning"). |
| ö | 0x94 | 148 | U+00F6 | Mycket vanligt i kontobenämningar under taggen \#KONTO. |
| Å | 0x8F | 143 | U+00C5 | Versal representation. |
| Ä | 0x8E | 142 | U+00C4 | Versal representation. |
| Ö | 0x99 | 153 | U+00D6 | Versal representation. |

Ett konkret implementeringsexempel illustrerar detta väl. Utvecklare som använder moderna programmeringsspråk (som Python, C\#, eller Node.js) måste utnyttja inbyggda kodningsbibliotek. I Python åstadkoms detta genom den inbyggda codecs-modulen.11 När en modern UTF-8-sträng såsom "Lön och försäljning" (vilken internt hanteras som en Unicode-sträng) ska exporteras till en SIE-fil, bör följande metodik tillämpas:

Python

\# Modern UTF-8 sträng internt i det nyskrivna systemet  
verifikationstext \= "Lön och försäljning"

\# Vid SIE-export: Koda till CP437.   
\# En robust applikation måste specificera errors='replace' eller 'ignore'.  
\# Detta förhindrar att applikationen kraschar med en UnicodeEncodeError   
\# om användaren matat in tecken som saknar representation i CP437 (t.ex. Emojis, €, ‰).  
sie\_bytes \= verifikationstext.encode('cp437', errors='replace') 

\# Byteströmmen kan nu säkert skrivas till.se eller.si filen.

### **Praktisk Accepterans i Moderna System: UTF-8 i Verkligheten**

Frågan om vilka teckenkodningar som accepteras i praktiken av moderna system är tudelad och utgör ett betydande interoperabilitetsproblem inom branschen. Den strikta specifikationen säger en sak, medan den pragmatiska verkligheten i molnsystem säger något annat.7

1. **Strikta och Äldre System:** Traditionella, on-premise bokföringssystem (ofta skrivna i äldre versioner av C++, Delphi eller Cobol) validerar filen exakt enligt specifikationen. De läser byteströmmen by-for-byte och utför tabelluppslag mot systemets interna CP437-kodblad.14 Om dessa system matas med en UTF-8-fil kommer all text med tecken utanför det vanliga ASCII-spannet att bli oåterkalleligt korrupt.11  
2. **Förlåtande Moderna Molnsystem:** Många nyskrivna, webbaserade affärssystem och AI-analysverktyg (som exempelvis vissa implementationer av Visma, Fortnox, eller tilläggsapplikationer) kämpar med det motsatta problemet. Eftersom de är byggda i plattformar som natively enbart talar UTF-8, implementerar de ibland heuristik för att hantera felaktiga SIE-filer.3 Om deras parser stöter på en fil som visserligen deklarerar \#FORMAT PC8, men där filens inledande bytes innehåller en UTF-8 Byte Order Mark (BOM), eller om byteströmmen i sin helhet validerar som perfekt UTF-8 utan ogiltiga sekvenser, kan systemet dynamiskt välja att "gissa" rätt och dölja felet för slutanvändaren.19

Detta pragmatiska beteende skapar dock en falsk trygghet och en fragmenterad standard. Utvecklare av försystem (exempelvis ett modernt molnbaserat HR-system) kan luras att tro att deras UTF-8 kodade SIE-filer är korrekta, eftersom de importeras felfritt i ett förlåtande system, bara för att mötas av systemkrascher när kunden byter till ett striktare bokföringsprogram.  
**Arkitektonisk Rekommendation för Nyskrivna System:**

* **Vid EXPORT:** Ett nyskrivet system ska **alltid och undantagslöst** respektera standarden och exportera SIE-4-filer strikt kodade i CP437.5 Att exportera UTF-8, oavsett dess globala dominans 7, bryter mot specifikationen. Om systemets interna databas innehåller tecken utanför CP437-repertoaren (vilket är högst sannolikt med emojis, japanska tecken eller specifika moderna valutasymboler som €), måste systemets serialiseringsmotor implementera en sanitetsrutin.12 Denna rutin måste translitterera eller ersätta dessa tecken med ASCII-ekvivalenter (t.ex. byta ut '€' mot "EUR") *före* export, för att förhindra datakorruption eller undantagsfel under kodningsprocessen.11  
* **Vid IMPORT:** Ett nyskrivet system bör bygga sin parser så att den som utgångspunkt alltid antar CP437, oavsett vad operativsystemet försöker tvinga fram.11 En robust och kommersiellt gångbar parser bör emellertid inkludera en "fallback"-logik: om filen inleds med en UTF-8 signatur (BOM) 19, eller om byteströmmen vid avkodning genererar ett orimligt högt antal meningslösa kontrolltecken varpå en alternativ läsning som UTF-8 plötsligt resulterar i felfri svensk text, kan parsern automatiskt byta läge. Detta ökar toleransen för inkommande filer från inkorrekt implementerade försystem, vilket minskar mängden supportärenden.

## **3\. Låg-nivå Formateringsregler och Lexikalisk Syntax för Datatyper**

Eftersom SIE-4 är ett taggat textformat som utvecklades före XML och JSON, ställer standarden extremt specifika krav på hur formatets inre mekanik – fältavgränsare, strängrepresentation och numeriska primitiva datatyper – ska serialiseras och tolkas av den lexikaliska analysatorn.2 Bristande efterlevnad av dessa regler resulterar oundvikligen i parsing-fel (fatal errors).

### **3.1 Syntax för Textsträngar, Avgränsare och Citattecken**

* **Lexikalisk avgränsare:** Fält inom en och samma post (item/rad) måste alltid separeras av minst ett, men gärna flera, mellanslag (space, ASCII 32). Specifikationen tillåter även uttryckligen att tabulatorer (ASCII 9\) tolkas och accepteras som fullgoda mellanslag.5 Detta möjliggör visuell kolumninriktning för mänskliga läsare, även om parsern ignorerar antalet på varandra följande avgränsare.  
* **Hantering av citattecken:** För textfält där värdet internt innehåller mellanslag (vilket är regel snarare än undantag för fält som företagsnamn, kontobenämningar och verifikationstexter), **måste** hela strängen ovillkorligen omslutas av dubbla citattecken (ASCII 34, ").5 För textfält som saknar mellanslag är citattecken tekniskt sett valfria enligt specifikationen.5 I modern praxis och för att minimera risken för tvetydigheter, rekommenderas dock starkt att exporters alltid omsluter alla textsträngar i citattecken.  
* **Sekvenser för Escaping:** Om ett citattecken förekommer inuti det faktiska textvärdet för ett fält, måste det maskeras (escapas) för att inte bryta strängen i parsern. Detta görs genom att föregå tecknet med ett omvänt snedstreck (backslash, ASCII 92, \\). Om systemet exempelvis ska exportera företagsnamnet Företaget "Solen" AB, måste det formateras i SIE-filen som "Företaget \\"Solen\\" AB".5  
* **Förbjudna och Otillåtna Tecken:** Standarden är rigorös när det gäller kontrolltecken. Samtliga styrtecken från ASCII 0 upp till och med ASCII 31, samt DEL-tecknet (ASCII 127), är strängt förbjudna att placera inuti textsträngar.5 Detta innebär i praktiken att en radbrytning (ASCII 10 eller 13\) inuti ett beskrivande fält – exempelvis inuti en verifikationstext – omedelbart invaliderar hela filens struktur, eftersom parsern använder just radbrytning för att markera slutet på ett SIE-kommando.5

### **3.2 Formatering av Numeriska Belopp (Decimaler och Tecken)**

Till skillnad från många databasstrukturer sätter SIE-formatet formellt inga övre, hårda gränser för storleken på de belopp som kan anges; filformatet stödjer i teorin obegränsat stora summor.5 Begränsningarna dikteras istället uteslutande av det mottagande programmets interna datatyper och minneskapacitet (exempelvis om systemet lagrar belopp som 32-bitars eller 64-bitars flyttal). Trots avsaknaden av maxbelopp är reglerna för *formateringen* av dessa siffror mycket rigida:

* **Decimaltecknets form:** Formateringen kräver anglosaxisk punkt (ASCII 46, .) som decimaltecken. Svenskt kommatecken (,) är under inga omständigheter tillåtet, trots filformatets svenska ursprung.5 Om kommatecken används kommer parsern antingen krascha eller tolka allt efter kommat som ett nytt, separerat fält.  
* **Antal decimaler:** Belopp får anges med maximalt två decimaler (vilket speglar det svenska systemet med ören). Om beloppet är exakt (d.v.s. inga ören, noll decimaler), ska det numeriska värdet anges som ett heltal utan varken decimaltecken eller efterföljande nollor (exempelvis ska värdet skrivas som 1000, och inte som 1000.00).5  
* **Matematiska Tecken (Sign-hantering):** Ett av de mest karaktäristiska dragen för formatet är dess teckenhantering. Negativa belopp (som inom dubbel bokföring alltid representerar Kredit) *måste* föregås av ett minustecken (-). **Det är strikt förbjudet att använda ett plustecken (+) för att uttrycka positiva belopp** (Debet).5 Värdet skrivs enbart som siffran i sig.  
* **Tusentalsavgränsare:** Användning av tusentalsavgränsare (såsom mellanslag, punkt eller kommatecken inuti ett större heltal) är strikt förbjuden. Värdet en miljon kronor och femtio öre måste formatteras som 1000000.50, och får aldrig skrivas som 1 000 000.50.5

### **3.3 Representation av Datum och Valfria Kvantiteter**

* **Datumformatering:** Alla datum, vare sig de utgör räkenskapsårets gränser eller tidpunkten för en specifik verifikation, måste ovillkorligen följa ett ISO 8601-liknande format, men sakna all form av interpunktion eller avgränsare. Formatet är fixerat till åtta tecken: **YYYYMMDD** (exempelvis 20260507).5  
* **Kvantitetsangivelser:** Även om SIE är ett finansiellt format, stöder specifikationen att man, utöver de finansiella beloppen, även bokför specifika kvantiteter. Detta är särskilt användbart för system som hanterar projektredovisning eller löner där timmar, antal enheter eller vikt (t.ex. inom lantbruk) måste föras över.5 Om en kvantitet exporteras placeras den omedelbart efter beloppet i de flesta tillämpliga taggar (såsom i slutet av en \#TRANS-rad). Kvantiteter följer exakt samma formateringsregler för tecken (utan plus, minus för kredit/minskning) som beloppen gör.5

## **4\. Den Tvingande Sekventiella Grammatiken och Övergripande Filstruktur**

En giltig SIE-fil kan inte förstås som en slumpmässig samling rader där taggarna kan kastas om hur som helst. Datamodellen ställer krav på en rigorös och sekventiell blockstruktur som måste parceras i rätt ordning för att variablerna ska hinna initieras. Filens komponenter måste följa en strikt sekventiell ordning. Avvikelser från denna övergripande struktur leder ofta till kritiska importfel i äldre redovisningssystem.5 Posterna (items) i en SIE-fil grupperas och *måste* förekomma i följande strikta ordningsföljd 5:

1. **Flaggpost (Flag item):** Den allra första posten, som initierar filen och kontrollerar status för import och export.  
2. **Identifikationsposter (Metadata):** Information som sätter kontexten för datan. Detta inkluderar identifiering av företaget, generatorn av filen, datum för exporten och, fundamentalt viktigt, definitionen av det räkenskapsår som datan avser.  
3. **Kontoplan och Dimensionsstruktur:** Det lexikon som transaktionerna refererar till. Alla konton, kontoklasser, samt alla dimensioner (kostnadsställen, projekt) och objekt måste deklareras i denna zon. Ett konto *måste* vara deklarerat här (eller redan existera i mottagande system) innan det kan anropas i ett senare block.5  
4. **Saldon och Verifikationer:** Den massiva kärndatan som ofta utgör 99 % av filens volym. Först definieras ingående saldon, sedan utgående saldon och slutligen listas alla transaktioner i kronologisk ordning via blocken \#VER och dess underliggande \#TRANS.5

### **Hantering av Radbrytningar och Tomrader**

Rent syntaktiskt måste varje enskild post (kommandorad) avslutas med ett strikt line feed-tecken (LF, ASCII 10). En kombination av carriage return och line feed (CRLF, ASCII 13 följt av ASCII 10), vilket är standard i Windows- och DOS-miljöer, är också fullt tillåtet och förväntat av parsern, men CR-tecknet är inte ett absolut tekniskt krav för formatets integritet.5 Specifikationen klargör även att tomrader (rader som enbart består av radbrytning) är tillåtna exakt var som helst i filstrukturen. En korrekt implementerad parser ska helt ignorera tomrader utan att kasta några varningar eller fel.5

## **5\. Tvingande och Rekommenderade Strukturer: Filhuvud och Metadata**

I tolkningen av SIE-4 uppstår ofta förvirring kring vad som är obligatoriskt. Skillnaden mellan "obligatoriska" (compulsory) poster och valfria (optional) poster är kontextberoende och varierar dessutom mellan SIE Typ 4E och Typ 4I. En grundregel är att en posttyp i sig själv kan vara valfri (till exempel företagsadress), men *om* det exporterande systemet väljer att inkludera posten, måste dess underliggande fält uppfylla strikta syntaktiska krav.5  
Följande markdown-tabell sammanställer de mest kritiska elementen i filhuvudet och definierar deras tvingande status och strukturella formatering för Typ 4E och 4I:

| Kommandotagg | Syntax och Exempel på Fältordning | Status i Typ 4E / 4I | Arkitektonisk och Teknisk Beskrivning |
| :---- | :---- | :---- | :---- |
| \#FLAGGA | \#FLAGGA flagg\_värde Ex: \#FLAGGA 0 | **Tvingande** | Måste vara det absolut första elementet i varje fil.5 Värdet sätts till 0 av det exporterande systemet. Vissa mottagande program ändrar denna till 1 i själva textfilen efter en lyckad import. Detta fungerar som ett arkitektoniskt lås för att förhindra dubbelimport av samma fysiska fil (vilket skyddar mot oavsiktligt duplicerade verifikationer i system som kontinuerligt bevakar en in-korg).5 |
| \#FORMAT | \#FORMAT teckenset Ex: \#FORMAT PC8 | **Tvingande** | Explicit deklaration av den teckenkodning som tillämpas på dataströmmen. Måste enligt specifikationen vara exakt \#FORMAT PC8.5 |
| \#SIETYP | \#SIETYP format\_typ Ex: \#SIETYP 4 | **Tvingande** | Definierar filens version och taxonomiska nivå. För transaktionsfiler deklareras värdet 4\.21 |
| \#PROGRAM | \#PROGRAM "program\_namn" "version" Ex: \#PROGRAM "Mitt MolnERP" "1.2.0" | Rekommenderad | Specificerar exakt vilken mjukvara som genererat filen. Oerhört viktigt för felsökning, support och vid identifikation av generiska exportfel.5 |
| \#GEN | \#GEN datum \[signatur\] Ex: \#GEN 20260507 "AutoSys" | Rekommenderad | Datostämpel för när filen genererades (YYYYMMDD). Den valfria signaturen anger ofta den inloggade användarens ID eller namnet på den batch-tjänst som skapade filen.3 |
| \#ORGNR | \#ORGNR "organisationsnummer" Ex: \#ORGNR "555555-5555" | Valfri / Starkt rek. | Företagets legala identifikationsnummer. Används ofta av redovisningsbyråer för att automatiskt matcha importfilen mot rätt klient i en multi-tenant klienthanterare.21 |
| \#FNAMN | \#FNAMN "företagsnamn" Ex: \#FNAMN "Acme AB" | Valfri / Starkt rek. | Företagets legala namn. Eftersom detta ofta innehåller mellanslag och svenska tecken, måste det omslutas av citattecken och CP437-kodas korrekt.5 |
| \#RAR | \#RAR årsnr startdatum slutdatum Ex: \#RAR 0 20260101 20261231 | **Tvingande** i 4E | Definierar det exakta räkenskapsår som saldon och verifikationer avser. Årsnummer är alltid 0 för det nuvarande året, \-1 för föregående år, och så vidare. Formateras YYYYMMDD. I SIE Typ 4E måste denna tagg finnas för både aktuellt och föregående år om data överhuvudtaget existerar i systemet.5 I importformatet Typ 4I kan taggen ibland utelämnas. |

## **6\. Den Logiska Modellen för Kontoplan, Dimensioner och Objektredovisning**

I SIE-4:s hierarkiska datamodell fungerar konton och dimensioner som det underliggande referensbiblioteket. För att en verifikation ska vara giltig när den importeras, måste de bokföringskonton som anropas antingen finnas statiskt upplagda i det mottagande systemet sedan tidigare, eller så måste de deklareras dynamiskt i SIE-filens huvud innan transaktionerna listas.

### **6.1 Kontodeklaration (\#KONTO)**

Kontoplanens element definieras genom taggen \#KONTO. Förekomsten av denna tagg är **tvingande** för kompletta exporter (SIE Typ 4E), eftersom syftet är att kunna återskapa hela företagets tillstånd i ett nytt system. I ett rent importscenario (Typ 4I) kan kontodeklarationer utelämnas om det exporterande försystemet kan garantera att det mottagande huvudbokföringssystemet redan har kontona konfigurerade.21  
**Standardiserad Syntax:** \#KONTO kontonummer "kontonamn" **Exempel från produktion:** \#KONTO 1930 "Företagskonto" 6

### **6.2 Arkitekturen bakom Dimensioner (\#DIM) och Objekt (\#OBJEKT)**

Företag behöver ofta bryta ned och redovisa kostnader och intäkter på en djupare nivå än bara konto, vilket åstadkoms genom "objektredovisning" (Cost centers, projekt, anställda, kampanjer).5 SIE Typ 3 och Typ 4 hanterar denna flerdimensionella komplexitet.5 Den fundamentala regeln är att innan ett objekt kan taggas på en transaktionsrad måste dess övergripande dimension, och därefter det specifika objektet i sig, finnas deklarerade.5  
För att förenkla integration mellan orelaterade mjukvarusystem reserverar specifikationen vissa dimensionsnummer för specifika, universella syften.5 Exempelvis är kod 1 ofta reserverad som "Kostnadsställe" och kod 6 som "Projekt", vilket tillåter system att smidigt mappa över data utan specialbyggda konfigurationsregler.24  
**Syntax för att initiera en ny dimension:** \#DIM dimensions\_nr "dimensions\_namn" Exempel: \#DIM 1 "Kostnadsställe" 5  
**Syntax för att populera dimensionen med ett specifikt objekt:** \#OBJEKT dimensions\_nr "objekt\_id" "objekt\_namn" Exempel: \#OBJEKT 1 "30" "Marknadsavdelningen" 24

## **7\. Verifikationer och Transaktionsrader: Den Finansiella Kärnan i SIE-4**

Hela syftet med SIE Typ 4 är att transportera händelsekedjor. Det absoluta hjärtat i formatet är därför det sekventiella block som hanterar de enskilda verifikationerna. Även om SIE absolut inte är XML, använder formatet en C-liknande syntax med klammerparenteser { } för att gruppera data och skapa tvingande relationer (sub-entries) mellan en verifikationshuvudpost (\#VER) och dess underliggande, detaljerade transaktionsrader (\#TRANS).5 Denna blockstruktur möjliggör snabb streaming och parsning av enorma filer.

### **7.1 Verifikationshuvudet och Kontexten (\#VER)**

Taggen \#VER deklarerar att en ny affärshändelse påbörjas. All övergripande metadata om händelsens tidpunkt, verifikationsserie och syfte aggregeras här.24  
**Obligatorisk Fältordning för \#VER:** \#VER serie "verifikationsnummer" verifikationsdatum "verifikationstext" 5

* **Serie:** Ett kort prefix, oftast en enda bokstav (exempelvis A för manuell huvudbok, L för leverantörsfakturor eller K för kundfakturor), som segmenterar händelserna.5  
* **Verifikationsnummer:** Ett alfanumeriskt strängvärde som entydigt identifierar händelsen i serien. Detta värde måste vara omslutet av citattecken. Vid exporter från försystem till huvudböcker skickar exportören ibland medvetet en tom sträng "". Detta signalerar till det mottagande systemet att det förväntas generera och tilldela egna sekventiella serienummer vid importen för att följa god redovisningssed.21  
* **Verifikationsdatum:** Fixerat till formatet YYYYMMDD.  
* **Verifikationstext:** En citattecken-omsluten textbeskrivning som förklarar händelsen, till exempel "Inköp kontorsmaterial". Det är här teckenkodning av 'ä' och 'ö' kritiskast prövas.

Omedelbart efter att \#VER-raden har deklarerats, måste en startklammer { placeras. Enligt specifikationen ska denna klammer stå helt ensam på sin egen rad.5 Därefter följer transaktionsraderna.

### **7.2 Anatomin i Transaktionsrader (\#TRANS)**

Strukturen inuti blocket utgörs av transaktionsraderna. Den överordnade redovisningsprincipen dikterar att summan av alla \#TRANS-belopp inom en och samma \#VER-gruppering (debet adderat med kredit) matematiskt måste balansera till exakt 0.00. Mjukvara som misslyckas med att balansera dessa på grund av flyttalsavrundning (ett vanligt problem vid momskalkylering) kommer att få sina filer förkastade vid import.26  
**Obligatorisk Fältordning för \#TRANS:** \#TRANS kontonummer {objektlista} belopp \[transaktionsdatum\]\[kvantitet\]\[signatur\] 5

* **Kontonummer:** Måste matcha ett konto (t.ex. 1930\) som deklarerats tidigare i filen eller som redan är känt.  
* **Objektlista:** Fältet hanterar den flerdimensionella konteringen (kostnadsställen/projekt). Den syntaktiska konstruktionen för objektlistan kräver att den i sig själv är omsluten av klammerparenteser: {dimensions\_nr "objekt\_id"}. Detta fält är tvingande; om ingen objektivkontering finns för raden, **måste** en tom klammer {} skickas. Syntaxen stöder aggregering av flera objekt på samma rad genom ett kontinuerligt flöde av par: {1 "30" 6 "101"} innebär att beloppet ska belasta både dimension 1 (objekt 30\) och dimension 6 (objekt 101).5  
  * *Arkitektonisk notis angående hierarkier:* Om dimensionerna importeras som hierarkiska träd, ska endast den understa sub-dimensionens kod specifieras i underobjektets position i listan, men det överliggande (parent) objektet måste också finnas med i samma objektlista.5  
* **Belopp:** Formateras med punkt som decimaltecken. Inget plustecken skrivs ut för debet, men minustecken är tvingande för kredit.5  
* *(Valfria parametrar)*  
* **Transaktionsdatum:** YYYYMMDD. Krävs enbart och appliceras endast om den unika transaktionsraden skiljer sig tidsmässigt från verifikationens övergripande huvuddatum.  
* **Kvantitet:** Hanterar icke-monetära flöden och anges med samma teckenregler som beloppen.5  
* **Signatur:** Ett valfritt användar-ID i citattecken som identifierar personen eller processen som initierade just denna rad.

### **7.3 Arkaiska Mekanismer för Historisk Bakåtkompatibilitet (\#RTRANS och \#BTRANS)**

Den avancerade specifikationen av SIE tillåter mer komplexa transaktionstyper än det grundläggande \#TRANS-kommandot. För hantering av transaktioner som tagits bort eller rensats ut finns \#RTRANS (raderad transaktion), och för att redovisa balanserade kopplingar används \#BTRANS.5  
Här introducerar specifikationen ett extremt specifikt och lättförbisedd krav på bakåtkompatibilitet. Om en \#RTRANS genereras i en modern systemintegration, dikterar standarden att denna rad **alltid och omedelbart måste följas av en identisk, falsk \#TRANS-rad**. Den enda anledningen till detta bisarra strukturella krav är att upprätthålla kompatibilitet med uråldriga, äldre SIE-parsers från 90-talet som helt saknar inbyggt lexikaliskt stöd för de nyare R- och B-taggarna; de läser då bara in \#TRANS-kopian och ignorerar den för dem okända taggen.5

## **8\. Minimalt men Komplett Kodexempel på SIE-4**

Följande syntetiska kodexempel demonstrerar en fullständigt giltig, balanserad och strukturellt komplett SIE Typ 4E-fil. Den illustrerar kronologin från filhuvudets konfiguration, via dimensioner och kontoplan, ned till klammerstrukturen för dubbel bokföring.  
*Notering till mjukvaruutvecklare: För att denna exempelfil ska vara tekniskt giltig måste de nedanstående strängarna vid skrivning till disk explicit omkodas till IBM PC8 (CP437) för att svenska tecken som i "Företagskonto" och "Konsultarvode" ska validera korrekt hos mottagaren.* 5  
\#FLAGGA 0  
\#FORMAT PC8  
\#SIETYP 4  
\#PROGRAM "Modern ERP Exporter" "1.0"  
\#GEN 20260507 "Systemanvändare"  
\#FNAMN "Företaget Exempel AB"  
\#ORGNR "555555-5555"  
\#RAR 0 20260101 20261231  
\#DIM 1 "Kostnadsställe"  
\#OBJEKT 1 "30" "Marknad"  
\#KONTO 1930 "Företagskonto"  
\#KONTO 4000 "Varuinköp"  
\#VER A "1" 20260507 "Inköp kontorsmaterial"  
{  
\#TRANS 4000 {1 "30"} 1500.00  
\#TRANS 1930 {} \-1500.00  
}  
\#VER A "2" 20260508 "Konsultarvode"  
{  
\#TRANS 4000 {} 25000.00  
\#TRANS 1930 {} \-25000.00  
}  
Ur ett parsingperspektiv är indenteringen av \#TRANS-raderna inuti klammerparenteserna { } inte tvingande; parsern struntar fullständigt i inledande blanksteg. Trots detta är indentering en absolut branschstandard, eftersom det radikalt ökar filens läsbarhet för tekniker vid manuell felsökning.5

## **9\. Kryptografisk Dataintegritet och Säkerhetmekanismer: CRC32 och \#KSUMMA**

När finansiell information exporteras som asynkrona textfiler uppstår omedelbart en säkerhets- och tillförlitlighetsfråga: Hur garanterar det mottagande systemet att filens innehåll inte har modifierats manuellt (ett säkerhetsbrott) eller korrumperats vid överföring via FTP eller e-post (ett tekniskt brott)? För att säkra dataintegriteten inkluderar SIE-formatet en avancerad algoritm för beräkning av kontrollsummor (checksum).5 Detta är en sofistikerad del av tekniken som ofta missförstås eller helt enkelt ignoreras vid slarviga implementationer.

### **Algoritmisk Implementering av Kontrollsumman**

Standardens matematiska grund för kontrollsumman är algoritmen **CRC32**, vilken specifikt använder det väletablerade polynomet 0xEDB88320L.5 Implementationen kräver förståelse för bit-hantering på låg nivå:

* **Initiering:** Hash-funktionen (pre-conditioning) ska alltid initieras med startvärdet 0xFFFFFFFFL innan beräkningen av byteströmmen börjar.5  
* **Finalisering:** När all relevant data är summerad, måste det resulterande värdet genomgå post-conditioning genom fullständig bit-invertering innan summan presenteras.5

Ett kritiskt systemarkitektoniskt faktum är att CRC32-beräkningen i SIE *inte* under några omständigheter appliceras på hela råfilen från början till slut som en blind byteström. Istället adderas och hashas summan uteslutande över det exakta data-innehållet i en strikt avgränsad uppsättning specifika taggar (däribland \#ADRESS, \#GEN, \#KONTO, \#IB, \#UB och det mest voluminösa, \#TRANS).5  
Ytterligare en nivå av parsing-komplexitet är att när kontrollsumman lexikalt räknas ut på textfält som inuti sig själva innehåller "escapade" (maskerade) citattecken (det vill säga sekvensen \\"), måste beräkningsrutinen förfina datan. Standarden dikterar att endast själva citattecknet ska inkluderas i hash-datan, medan backslash-tecknet ska exkluderas och förbises.5 Att implementera en parser som korrekt utför denna selektiva extraktion och CRC32-hashning i farten ställer höga krav på koden.

### **Konfigurering och Signalering via \#KSUMMA**

För att styra parserns beteende rörande kontrollsummor och förhindra avbrott om exportören inte implementerat stödet, använder filen kommandot \#KSUMMA för handskakning på två separata ställen i flödet 5:

1. **I filens inledning (Signalering):** Taggen \#KSUMMA 0 placeras i filhuvudet. Detta fungerar som en direkt flagga till det mottagande programmets import-engine. Den säger: "En valid kontrollsumma existerar för denna fil, påbörja ackumulering av CRC32-värden under inläsningen av raderna." 5  
2. **I filens absoluta slut (Verifiering):** Som den sista meningsfulla raden i hela filen placeras \#KSUMMA 1234567890 (där siffrorna utgör det faktiska framräknade hashvärdet från exporterande system).

Importprocessen i det mottagande systemet har därefter ett strikt ansvar: Om den inledande \#KSUMMA 0 identifierades i filhuvudet, men den avslutande \#KSUMMA-taggen saknas vid filens slut (vilket indikerar att filen har kapats av i förtid), eller om det internt beräknade CRC32-värdet under inläsningen avviker från det deklarerade slutvärdet, **måste importprogrammet omedelbart och obönhörligen avbryta importen och utfärda ett dataintegritetsfel till slutanvändaren**.5 Om det exporterande försystemet är byggt på ett sätt som gör det orimligt att utföra denna komplicerade fält-specifika CRC32-beräkning (exempelvis på grund av en strömmande resursbegränsad arkitektur), bör \#KSUMMA-taggarna helt och hållet utelämnas. Äldre system faller då tillbaka på blind acceptans.

## **10\. Moderna Applikationsutmaningar i Molnet: Skalbarhet, Minne och Tid**

Även om SIE-4 tekniskt sett bara är en asynkron och passiv textfil, ställer moderna informationsvolymer enormt höga krav på hanteringen av dessa filer vid både import och export. System som exporterar data från stora databaser, exempelvis globala e-handelsplattformar eller transaktionsintensiva kassasystem (POS), producerar regelmässigt SIE-filer med flera miljoner transaktionsrader.21 Försök att hantera sådana volymer med aningslösa programmeringsmönster resulterar i totala systemhaverier.

### **Paginering och Förhindrande av Minneskrascher**

I molnmiljöer (såsom hos stora affärssystem som Deltek Maconomy) har arkitekter upprepade gånger konfronterats med allvarliga "Java heap space"-undantag och direkta minnesrelaterade serverkraschar när användare begärt tunga SIE-4-exporter för helårsrevisioner.21 En naiv tillvägagångssätt hos en mjukvaruutvecklare är att skriva en databasfråga som laddar *all* verifikationsdata till en enorm matris (Array/List) i applikationens arbetsminne (RAM), formaterar matrisen som en massiv CP437-sträng och därefter försöker skriva resultatet till disken. Detta är ett utpräglat anti-pattern för SIE-4.21  
Dokumenterad prestandaanalys påvisar att parsning eller export av cirka 1,2 miljoner transaktionsrader i ett storskaligt ERP-system kräver i storleksordningen 10 GB allokerat, ledigt arbetsminne (RAM) om hela datamängden hålls statiskt i minnet under operationen.21 För att mitigera dessa katastrofala minnesläckor måste systemen utrustas med strömmande och paginerande logik.

* **Paginering vid serialisering:** Moderna exporterande system tvingas införa paginering (pagination) av resultatmängden vid själva databasuttaget. Lämpliga storlekar på dataseten ligger mellan 20 000 och upp till 100 000 verifikationer per "sida" (page), beroende på serverns kapacitet.21  
* **Temporära Diska-Filer (Streaming):** Istället för att bygga ett DOM-träd av text i minnet, strömmas data. Systemet hämtar batch 1, kodar datan till CP437, och skriver den linjärt (append) i slutet av en temporär fil på disken, varpå det Garbage Collectar (rensar) minnet innan batch 2 hämtas.28 Detta håller RAM-förbrukningen på en konstant låg nivå oavsett filens enorma slutstorlek.  
* **Justering av Serverns Tidsgränser:** Eftersom linjär diskläsning och beräkning av kontrollsummor på miljoner rader är en CPU-intensiv flaskhals, upplever många webbservrar (exempelvis IIS eller Apache/Tomcat) att processen överstiger deras standard-timeouts (som ofta är ställda på några få minuter). Erfarenheten från industriella implementationer visar att systemadministratörer ibland behöver konfigurera processernas maximala exekveringstid ("Max. Duration") till extrema nivåer, upp emot 24 eller till och med 48 timmar, för att garantera att gigantiska retroaktiva revisionsexporter färdigställs.28

### **Balansering och Flyttalsproblematik**

Ett annat återkommande problem vid generering av storskaliga SIE-4-filer är förlust av numerisk precision och avrundningsfel. Transaktionerna lagras vanligen i systemens SQL-databaser som typerna DECIMAL eller NUMERIC, men i SIE-filen representeras de enbart som avrundade ASCII-tecken (10.50).5  
Om ett system ackumulerar saldon för den Ingående Balansen (\#IB) och den Utgående Balansen (\#UB) med en underliggande logik för flyttal (exempelvis en Java Double eller Python Float), kan aggregeringen drabbas av binära avrundningsfel. När rapporten sedan summerar dessa värden och formaterar dem med två decimaler, kan det uppstå mikroskopiska obalanser där verifikationens summa inte är exakt noll.26 Mjukvaruarkitekter måste till fullo garantera att de aggregerade beloppen (summan av alla \#TRANS under en och samma \#VER) formateras i SIE-filen med absolut identisk avrundningslogik som den inre kalkylmotorn, annars riskerar filen att permanent förkastas av revisorns importhantering.

## **11\. Slutsatser och Arkitektoniska Best Practices för Nyutveckling**

Ett nytt affärssystem, oavsett om det är en mikrotjänst i molnet eller en on-premise-mjukvara, som siktar på att integrera och fungera sömlöst på den svenska redovisningsmarknaden, har inga genvägar förbi SIE-4. Utifrån den tekniska specifikationen, kombinerat med de bittra lärdomarna från de verkliga tekniska fallgropar som finns dokumenterade i ekosystemets praxis, kan följande tvingande arkitektoniska riktlinjer dras för framtidssäker implementering:

1. **Omfamna CP437-kravet med stränghet:** Försök under inga omständigheter att tvinga fram eller ignorera UTF-8-kompatibilitet i exportfiler, oavsett hur antikt eller ologiskt Codepage 437 uppfattas ur ett modernt utvecklingsperspektiv. Att serialisera datan strikt som CP437 är det *enda* deterministiska sättet att garantera att informationen överlever import utan korrupta tecken i Skatteverkets analyssystem, hos konservativa revisorer och i de otaliga äldre klient-ERP-system som fortfarande opererar i landets kärnverksamheter.5 Den initiala tekniska smärtan med att bygga robust encoding/decoding sparar enorma mängder supporttid.  
2. **Förlita er uteslutande på \#SIETYP 4 för all dataöverföring:** Formaten Typ 1, 2 och 3 betraktas i många seriösa implementationssammanhang som antingen förlegade eller otillräckliga för djup, spårbar finansiell analys. Den arkitektoniska standardrekommendationen är att konsekvent och alltid designa sina datautdrag för att inkludera verifikationer som fullvärdiga Typ 4E- eller Typ 4I-filer, oavsett kundens påstådda, minimerade behov.5 Då täcker man alla framtida revisionskrav automatiskt.  
3. **Streama vid serialisering och skippa DOM:** Undvik instinkten att ladda och bygga upp enorma datastrukturer i arbetsminnet innan serialiseringen till text sker. Bygg istället omedelbart asynkrona streamers. Dessa streamers utläser data från lagringslagret genom små sidor (pages), formaterar det som \#VER- och \#TRANS-block i farten, och skriver direkt till filresursen.21 Detta upprätthåller algoritmens tids- och rumskomplexitet och förhindrar oväntade systemnedstängningar vid årsslutsexporter.  
4. **Objekthantering i en nymodig kontext:** Moderna SaaS-plattformar lagrar ofta dimensioner (projekt, anställda, avdelningar) som hierarkiska noder i en graf- eller NoSQL-databas. Detta moderna mönster måste försiktigt tvättas och mappas ned korrekt till SIE-formatets platta, länkade struktur som nyttjar \#DIM och den strikt omslutna syntaxen {dimension "objekt"} inuti transaktionsraden. Saknas objekt på en transaktionsrad måste arkitekturen säkerställa att en tom klammer {} skickas ut; den får aldrig tyst ignoreras.5

Den enorma, historiska och framtida styrkan i det svenska filformatet SIE-4 vilar paradoxalt nog i dess avskalade, djupt omoderna och extremt strikt typade textrepresentation. Eftersom reglerna för balansering, formatering och teckenkodning är så omutbara, lämnar det extremt minimalt utrymme för tolkningsfel mellan parterna. Så länge integratörer och mjukvaruarkitekter med rigorös precision lyder de semantiska reglerna kring matematisk nolltolerans för fel (balansering), CP437-kodningens historiska arv, strömmad minneshantering och de grammatiska lagarna för blockavslut ({ }), kommer formatet obehindrat fortsätta fungera felfritt som den robusta digitala ryggraden för svensk finansiell dataportabilitet under överskådlig framtid.

#### **Citerade verk**

1. The SIE file format \- Föreningen SIE-Gruppen, hämtad maj 7, 2026, [https://sie.se/in-english/](https://sie.se/in-english/)  
2. SIE (file format) \- Wikipedia, hämtad maj 7, 2026, [https://en.wikipedia.org/wiki/SIE\_(file\_format)](https://en.wikipedia.org/wiki/SIE_\(file_format\))  
3. GitHub \- magnusfroste/sie-parser: Join me in co-developing an innovative parser designed to transform intricate Swedish bookkeeping data (SIE 4 files) into a structured, LLM-friendly JSON format. This crucial tool will enable seamless integration with advanced AI models, unlocking enhanced user interactions and sophisticated financial analysis capabilities., hämtad maj 7, 2026, [https://github.com/magnusfroste/sie-parser](https://github.com/magnusfroste/sie-parser)  
4. E av tt ny v ge ytt fi neri ilform ska mat redo för ö ovisn över ning rföri gsdat ng ta \- Srf konsulterna, hämtad maj 7, 2026, [https://www.srfkonsult.se/app/uploads/2015/10/sie-5-remissversion.pdf](https://www.srfkonsult.se/app/uploads/2015/10/sie-5-remissversion.pdf)  
5. SIE file format, hämtad maj 7, 2026, [https://sie.se/wp-content/uploads/2020/05/SIE\_filformat\_ver\_4B\_ENGLISH.pdf](https://sie.se/wp-content/uploads/2020/05/SIE_filformat_ver_4B_ENGLISH.pdf)  
6. The SIE File format \- UNECE, hämtad maj 7, 2026, [https://unece.org/fileadmin/DAM/cefact/cf\_forums/2019\_Geneva/Conf\_AccountAudit/PPT\_2\_1\_SIE.pdf](https://unece.org/fileadmin/DAM/cefact/cf_forums/2019_Geneva/Conf_AccountAudit/PPT_2_1_SIE.pdf)  
7. UTF-8 \- Wikipedia, hämtad maj 7, 2026, [https://en.wikipedia.org/wiki/UTF-8](https://en.wikipedia.org/wiki/UTF-8)  
8. Character encoding: CP 437 \- Localizely, hämtad maj 7, 2026, [https://localizely.com/character-encodings/cp437/](https://localizely.com/character-encodings/cp437/)  
9. CP437 \- Just Solve the File Format Problem, hämtad maj 7, 2026, [http://fileformats.archiveteam.org/wiki/CP437](http://fileformats.archiveteam.org/wiki/CP437)  
10. CP437 Encoding : R | Encoding Solutions Across Programming Languages \- MojoAuth, hämtad maj 7, 2026, [https://mojoauth.com/character-encoding-decoding/cp437-encoding--r](https://mojoauth.com/character-encoding-decoding/cp437-encoding--r)  
11. CP437 in Python | Encoding Standards for Programming Languages \- SSOJet, hämtad maj 7, 2026, [https://ssojet.com/character-encoding-decoding/cp437-in-python](https://ssojet.com/character-encoding-decoding/cp437-in-python)  
12. CP437 vs UTF-8 | Compare Popular Character Encoding Standards \- MojoAuth, hämtad maj 7, 2026, [https://mojoauth.com/compare-character-encoding/cp437-vs-utf-8](https://mojoauth.com/compare-character-encoding/cp437-vs-utf-8)  
13. Internationalization Best Practices for Spec Developers \- W3C, hämtad maj 7, 2026, [https://www.w3.org/TR/international-specs/](https://www.w3.org/TR/international-specs/)  
14. Character encoding units \- IBM, hämtad maj 7, 2026, [https://www.ibm.com/docs/en/cobol-zos/6.3.0?topic=pages-character-encoding-units](https://www.ibm.com/docs/en/cobol-zos/6.3.0?topic=pages-character-encoding-units)  
15. File.encoding values and IBM i CCSID, hämtad maj 7, 2026, [https://www.ibm.com/docs/en/i/7.4.0?topic=encodings-fileencoding-values-i-ccsid](https://www.ibm.com/docs/en/i/7.4.0?topic=encodings-fileencoding-values-i-ccsid)  
16. Export transactions in SIE format \- Cledara, hämtad maj 7, 2026, [https://help.cledara.com/hc/en-gb/articles/25278779765138-Export-transactions-in-SIE-format](https://help.cledara.com/hc/en-gb/articles/25278779765138-Export-transactions-in-SIE-format)  
17. SIE export (GL40100S) | Visma Net ERP, hämtad maj 7, 2026, [https://docs.vismasoftware.no/visma-net-erp/help/general-ledger/general-ledger-windows/sie-export-gl40100s/](https://docs.vismasoftware.no/visma-net-erp/help/general-ledger/general-ledger-windows/sie-export-gl40100s/)  
18. Handling Swedish SIE files for Fortnox \- invantive, hämtad maj 7, 2026, [https://forums.invantive.com/t/handling-swedish-sie-files-for-fortnox/5611](https://forums.invantive.com/t/handling-swedish-sie-files-for-fortnox/5611)  
19. FAQ \- UTF-8, UTF-16, UTF-32 & BOM \- Unicode, hämtad maj 7, 2026, [https://unicode.org/faq/utf\_bom.html](https://unicode.org/faq/utf_bom.html)  
20. Selecting an encoding \- IBM, hämtad maj 7, 2026, [https://www.ibm.com/docs/en/icos/22.1.2?topic=output-selecting-encoding](https://www.ibm.com/docs/en/icos/22.1.2?topic=output-selecting-encoding)  
21. Standard Import/Export (Data Export) \- Deltek Software Manager, hämtad maj 7, 2026, [https://help.deltek.com/product/maconomy/documentation/BPMReporting/CountryReports/Country\_Sweden\_Standard\_Import\_Export.html](https://help.deltek.com/product/maconomy/documentation/BPMReporting/CountryReports/Country_Sweden_Standard_Import_Export.html)  
22. SIE4 file \- Centra Support, hämtad maj 7, 2026, [https://support.centra.com/centra-sections/modules/sie4](https://support.centra.com/centra-sections/modules/sie4)  
23. Alphabet Rosetta accounting file format and method, hämtad maj 7, 2026, [https://www.alphabet.se/Rosetta/Rosetta\_Specification.pdf](https://www.alphabet.se/Rosetta/Rosetta_Specification.pdf)  
24. SIE 4 \- The Moment Knowledge Base, hämtad maj 7, 2026, [https://docs.milientsoftware.com/help/moment-by-topic/integrations/file-exports/file-export-accounting-system/sie-4](https://docs.milientsoftware.com/help/moment-by-topic/integrations/file-exports/file-export-accounting-system/sie-4)  
25. Import and Export Data in SIE \[SE\] \- Business Central \- Microsoft Learn, hämtad maj 7, 2026, [https://learn.microsoft.com/en-us/dynamics365/business-central/localfunctionality/sweden/how-to-import-and-export-data-in-standard-import-export-format](https://learn.microsoft.com/en-us/dynamics365/business-central/localfunctionality/sweden/how-to-import-and-export-data-in-standard-import-export-format)  
26. Deltek Maconomy Essentials 2.6.1 Release Notes, hämtad maj 7, 2026, [https://dsm.deltek.com/DeltekSoftwareManagerWebServices/downloadFile.ashx?documentid=8170F95E-02C0-47D3-9D9F-709C5DBA09FB](https://dsm.deltek.com/DeltekSoftwareManagerWebServices/downloadFile.ashx?documentid=8170F95E-02C0-47D3-9D9F-709C5DBA09FB)  
27. Swebase \- Programekonomi, hämtad maj 7, 2026, [http://www.programekonomi.se/Manuals/SWE-SweBase-OnCloud.pdf](http://www.programekonomi.se/Manuals/SWE-SweBase-OnCloud.pdf)  
28. Deltek Maconomy Essentials 2.5.2 Release Notes, hämtad maj 7, 2026, [https://dsm.deltek.com/DeltekSoftwareManagerWebServices/downloadFile.ashx?documentid=359223C9-7942-4107-AEC1-11DA9BD69E4D](https://dsm.deltek.com/DeltekSoftwareManagerWebServices/downloadFile.ashx?documentid=359223C9-7942-4107-AEC1-11DA9BD69E4D)  
29. Deltek Maconomy Essentials 2.5.2 Release Notes, hämtad maj 7, 2026, [https://dsm.deltek.com/DeltekSoftwareManagerWebServices/downloadFile.ashx?documentid=FC3B5A1B-A654-45F2-AE06-21FFF9B80D99](https://dsm.deltek.com/DeltekSoftwareManagerWebServices/downloadFile.ashx?documentid=FC3B5A1B-A654-45F2-AE06-21FFF9B80D99)
# PRD: Tunn OCR (Kognitiv Heuristik)

## 1. Bakgrund & Vision
För att minimera manuell inmatning vid kvittohantering ska LocalLedger utrustas med automatisk inläsning av underlag. Efter flera arkitektoniska iterationer har vi övergivit tunga AI-modeller (LLM/VLM) till förmån för en ultralätt, 100% portabel och blixtsnabb lösning baserad på deterministisk heuristik.

## 2. Arkitektoniska Grundpelare
1. **Dumb OCR (Klienten gör grovjobbet):** För att hålla Go-backend helt fri från tunga beroenden och CGO-krav, sköter användarens webbläsare all text-extraktion lokalt.
   - **PDF:** Text plockas ut omedelbart via `pdf.js`.
   - **Bilder:** Text extraheras via `Tesseract.js` (WebAssembly) utan att filerna lämnar datorn.
2. **Go-Heuristik (Regular Expressions):** Go-backend tar emot den råa (ofta stökiga) textsträngen och använder `regexp` för att vaska fram nyckeldata baserat på förutsägbara svenska kvittoformat (t.ex. "ATT BETALA", "MOMS 25%", "Datum"). Noll inferenstarif, noll RAM-krav.
3. **Kognitiv Isolering (Historik-matchning):** När Go-heuristiken identifierat leverantörsnamnet (ofta i början av kvittot eller vid ett org.nr), görs ett uppslag i användarens lokala `ledger.db`. Systemet hittar tidigare verifikationer från samma leverantör och föreslår exakt samma BAS-konto.
4. **Human-in-the-Loop:** Systemet är en "föreslående" assistent. Misslyckas heuristiken lämnas fälten tomma. Användaren måste alltid granska och klicka "Bokför".

## 3. Dataflöde (End-to-End)
1. **Input:** Användare droppar kvitto i LocalLedger UI.
2. **Frontend-extraktion:** Alpine.js aktiverar Tesseract.js/PDF.js som bearbetar bilden/dokumentet och returnerar en lång, rå textsträng.
3. **API-anrop:** Textsträngen skickas via `POST /api/ocr/parse`.
4. **Backend-parsning:** Go kör sina fördefinierade regex-filter.
   - Söker efter `Datum` (YYYY-MM-DD eller YYMMDD).
   - Söker efter `Totalbelopp` och `Momsbelopp`.
   - Extraherar `Leverantör` (t.ex. första textraden eller rad vid organisationsnummer).
5. **Databas-matchning:** Go frågar `ledger.db`: "Vilket konto bokfördes 'IKEA' på senast?". Svar: "5410".
6. **Respons:** Go skickar tillbaka strukturerad JSON till frontenden.
7. **Autofill:** UI:t pre-fyller verifikationsformuläret.

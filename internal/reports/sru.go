package reports

import (
	"embed"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"golang.org/x/text/encoding/charmap"
	"localledger/internal/ledger"
)

//go:embed sru_bas2026.json
var sruEmbedFS embed.FS

// GenerateSRUFiles genererar innehållet till INFO.SRU och BLANKETTER.SRU för Skatteverket.
func GenerateSRUFiles(l *ledger.Ledger, yearID int64) (infoSRU []byte, blanketterSRU []byte, err error) {
	db := l.DB()

	// 1. Hämta räkenskapsår
	var startDate, endDate string
	err = db.QueryRow("SELECT start_date, end_date FROM fiscal_years WHERE id = ?", yearID).Scan(&startDate, &endDate)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get fiscal year: %w", err)
	}

	// 2. Hämta inställningar
	settings, err := l.GetSettings()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get settings: %w", err)
	}

	// 3. Ladda mappningar från inbäddad JSON
	mappingData, err := sruEmbedFS.ReadFile("sru_bas2026.json")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read embedded mappings: %w", err)
	}
	var mappings map[string]string
	if err := json.Unmarshal(mappingData, &mappings); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal mappings: %w", err)
	}

	// 4. Hämta aggregerade saldon per konto för det valda året
	query := `
		SELECT 
			a.code, 
			a.type,
			COALESCE(SUM(r.debet), 0) as total_debet,
			COALESCE(SUM(r.kredit), 0) as total_kredit
		FROM verification_rows r
		JOIN verifications v ON r.verification_id = v.id
		JOIN accounts a ON r.account = a.code
		WHERE v.date >= ? AND v.date <= ?
		GROUP BY a.code, a.type
	`
	rows, err := db.Query(query, startDate, endDate)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query balances: %w", err)
	}
	defer rows.Close()

	// Aggregera per SRU-kod
	sruValues := make(map[string]int64)

	for rows.Next() {
		var code, accType string
		var debet, kredit int64
		if err := rows.Scan(&code, &accType, &debet, &kredit); err != nil {
			return nil, nil, fmt.Errorf("failed to scan row: %w", err)
		}

		sruCode, ok := mappings[code]
		if !ok {
			// Hoppa över konton som saknar SRU-mappning
			continue
		}

		var balance int64
		switch accType {
		case "Intäkt", "Skuld":
			balance = kredit - debet
		case "Tillgång", "Kostnad":
			balance = debet - kredit
		}

		sruValues[sruCode] += balance
	}

	if err = rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("rows iteration error: %w", err)
	}

	// 5. Skapa INFO.SRU
	timestampStr := time.Now().Format("20060102 150405")
	cleanOrgNr := strings.ReplaceAll(settings.OrgNumber, "-", "")
	if cleanOrgNr == "" {
		cleanOrgNr = "165560000100" // Fallback om orgnummer saknas (standardformat)
	}
	
	var infoBuilder strings.Builder
	infoBuilder.WriteString("#DATABESKRIVNING_START\r\n")
	infoBuilder.WriteString("#PRODUKT SRU\r\n")
	infoBuilder.WriteString(fmt.Sprintf("#SKAPAD %s\r\n", timestampStr))
	infoBuilder.WriteString("#PROGRAM LocalLedger 1.5.0\r\n")
	infoBuilder.WriteString("#FILNAMN BLANKETTER.SRU\r\n")
	infoBuilder.WriteString("#DATABESKRIVNING_SLUT\r\n")
	infoBuilder.WriteString("#MEDIELEV_START\r\n")
	infoBuilder.WriteString(fmt.Sprintf("#ORGNR %s\r\n", cleanOrgNr))
	infoBuilder.WriteString(fmt.Sprintf("#NAMN %s\r\n", settings.Name))
	infoBuilder.WriteString(fmt.Sprintf("#ADRESS %s\r\n", settings.Address))
	infoBuilder.WriteString("#POSTNR \r\n")
	infoBuilder.WriteString("#POSTORT \r\n")
	infoBuilder.WriteString(fmt.Sprintf("#KONTAKT %s\r\n", settings.Name))
	infoBuilder.WriteString(fmt.Sprintf("#TELEFON %s\r\n", settings.SwishNumber))
	infoBuilder.WriteString("#EMAIL \r\n")
	infoBuilder.WriteString("#MEDIELEV_SLUT\r\n")
	infoBuilder.WriteString("#FIL_SLUT\r\n")

	// 6. Skapa BLANKETTER.SRU (NE_2026-format för enskild firma)
	var blanketterBuilder strings.Builder
	blanketterBuilder.WriteString("#SRU 2.0\r\n")
	blanketterBuilder.WriteString("#BLANKETT NE_2026\r\n")
	blanketterBuilder.WriteString(fmt.Sprintf("#IDENTITET %s %s\r\n", cleanOrgNr, time.Now().Format("20060102 150405")))

	for sruCode, value := range sruValues {
		// Avrunda ören till närmaste krona
		valueInSek := int64(math.Round(float64(value) / 100.0))
		blanketterBuilder.WriteString(fmt.Sprintf("#UPPGIFT %s %d\r\n", sruCode, valueInSek))
	}

	blanketterBuilder.WriteString("#BLANKETTSLUT\r\n")
	blanketterBuilder.WriteString("#FIL_SLUT\r\n")

	// 7. Kodkodning till ISO-8859-1 för Skatteverket-kompabilitet med ÅÄÖ
	encoder := charmap.ISO8859_1.NewEncoder()
	
	infoBytes, err := encoder.Bytes([]byte(infoBuilder.String()))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode INFO.SRU to ISO-8859-1: %w", err)
	}

	blanketterBytes, err := encoder.Bytes([]byte(blanketterBuilder.String()))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode BLANKETTER.SRU to ISO-8859-1: %w", err)
	}

	return infoBytes, blanketterBytes, nil
}

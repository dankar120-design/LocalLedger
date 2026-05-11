package ledger

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"math"
	"strconv"
	"strings"

	"localledger/internal/models"

	"golang.org/x/text/encoding/charmap"
)

// splitSIE delar en SIE-rad vid mellanslag, men ignorerar mellanslag inuti citattecken och klamrar.
func splitSIE(line string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false
	inBraces := false

	for _, r := range line {
		switch r {
		case '"':
			inQuotes = !inQuotes
			current.WriteRune(r)
		case '{':
			if !inQuotes {
				inBraces = true
			}
			current.WriteRune(r)
		case '}':
			if !inQuotes {
				inBraces = false
			}
			current.WriteRune(r)
		case ' ', '\t':
			if inQuotes || inBraces {
				current.WriteRune(r)
			} else if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

// ImportSIE4 parsear och importerar en SIE-4 fil.
// Den utförs atomärt, och kräver att året är tomt.
func (l *Ledger) ImportSIE4(user string, yearID int64, fileData []byte) error {
	// 1. Verifiera att året är tomt
	var count int
	var startDate, endDate string
	if err := l.db.QueryRow("SELECT start_date, end_date FROM fiscal_years WHERE id = ?", yearID).Scan(&startDate, &endDate); err != nil {
		return fmt.Errorf("failed to get fiscal year: %w", err)
	}

	err := l.db.QueryRow("SELECT COUNT(*) FROM verifications WHERE date >= ? AND date <= ?", startDate, endDate).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check verification count: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("import är endast tillåtet på helt tomma räkenskapsår. Det finns redan %d verifikationer.", count)
	}

	// 1b. Kontrollera att inga period-lås blockerar importen
	var lockCount int
	startMonth := startDate[:7]
	endMonth := endDate[:7]
	err = l.db.QueryRow("SELECT COUNT(*) FROM period_locks WHERE year_month >= ? AND year_month <= ?", startMonth, endMonth).Scan(&lockCount)
	if err != nil {
		return fmt.Errorf("failed to check period locks: %w", err)
	}
	if lockCount > 0 {
		return fmt.Errorf("import blockerad: det finns %d låsta perioder i det valda räkenskapsåret. Lås upp perioderna först.", lockCount)
	}

	// 2. Decode CP850 to UTF-8
	decoder := charmap.CodePage850.NewDecoder()
	utf8Data, err := decoder.Bytes(fileData)
	if err != nil {
		// Fallback till UTF-8 (vissa SIE-filer bryter mot standarden)
		utf8Data = fileData
	}

	reader := bufio.NewReader(bytes.NewReader(utf8Data))

	var currentVerReq *models.VerificationRequest
	var verificationsToImport []models.VerificationRequest
	var accountsToInsert []models.Account
	var ibBalance int64
	var ibRows []models.RowRequest

	// Hjälpfunktion för att ta bort citattecken
	stripQuotes := func(s string) string {
		s = strings.TrimSpace(s)
		if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
			return s[1 : len(s)-1]
		}
		return s
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read error: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := splitSIE(line)
		if len(parts) == 0 {
			continue
		}

		cmd := strings.ToUpper(parts[0])

		switch cmd {
		case "#RES":
			// Ignorera resultat-taggar, vi räknar ut resultatet on-the-fly
			continue

		case "#KONTO":
			if len(parts) >= 3 {
				code := stripQuotes(parts[1])
				name := stripQuotes(strings.Join(parts[2:], " "))
				// Guess type based on first digit
				accType := "Okänd"
				if len(code) > 0 {
					switch code[0] {
					case '1': accType = "Tillgång"
					case '2': accType = "Skuld"
					case '3': accType = "Intäkt"
					case '4', '5', '6', '7', '8': accType = "Kostnad"
					}
				}
				accountsToInsert = append(accountsToInsert, models.Account{
					Code: code,
					Name: name,
					Type: accType,
				})
			}

		case "#IB":
			// #IB år konto saldo
			if len(parts) >= 4 {
				yearStr := parts[1]
				if yearStr != "0" { // Endast innevarande år
					continue
				}
				account := stripQuotes(parts[2])
				amountStr := parts[3]
				amountFloat, err := strconv.ParseFloat(amountStr, 64)
				if err == nil {
					amountOren := int64(math.Round(amountFloat * 100))
					
					row := models.RowRequest{
						Account: account,
					}
					
					// Positiva belopp är Debet i SIE, Negativa är Kredit
					if amountOren > 0 {
						row.Debet = amountOren
					} else {
						row.Kredit = -amountOren
					}
					
					ibBalance += amountOren
					ibRows = append(ibRows, row)
				}
			}

		case "#VER":
			// Spara den föregående om den existerar
			if currentVerReq != nil {
				verificationsToImport = append(verificationsToImport, *currentVerReq)
			}

			// Format: #VER serie vnr datum text
			if len(parts) >= 4 {
				// Datum format i SIE är YYYYMMDD
				dateStr := stripQuotes(parts[3])
				if len(dateStr) == 8 {
					dateStr = fmt.Sprintf("%s-%s-%s", dateStr[0:4], dateStr[4:6], dateStr[6:8])
				}
				
				text := ""
				if len(parts) > 4 {
					text = stripQuotes(strings.Join(parts[4:], " "))
				}

				currentVerReq = &models.VerificationRequest{
					Date: dateStr,
					Text: text,
					Type: "NORMAL",
					Rows: []models.RowRequest{},
				}
			}

		case "#TRANS":
			if currentVerReq != nil && len(parts) >= 3 {
				account := stripQuotes(parts[1])
				
				// Hitta beloppet. Det kan vara 3e eller 4e argumentet beroende på om det finns "objekt" {}
				var amountStr string
				if strings.HasPrefix(parts[2], "{") {
					if len(parts) >= 4 {
						amountStr = parts[3]
					}
				} else {
					amountStr = parts[2]
				}

				amountFloat, err := strconv.ParseFloat(amountStr, 64)
				if err == nil {
					amountOren := int64(math.Round(amountFloat * 100))
					
					row := models.RowRequest{
						Account: account,
					}
					
					if amountOren > 0 {
						row.Debet = amountOren
					} else {
						row.Kredit = -amountOren
					}
					
					currentVerReq.Rows = append(currentVerReq.Rows, row)
				}
			}
		}
	}

	// Spara den sista verifikationen
	if currentVerReq != nil {
		verificationsToImport = append(verificationsToImport, *currentVerReq)
	}

	if len(verificationsToImport) == 0 {
		return fmt.Errorf("hittade inga verifikationer att importera")
	}

	// ATOMÄR TRANSACTION
	tx, err := l.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}

	success := false
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()

	// 1. Sätt in alla saknade konton
	for _, acc := range accountsToInsert {
		_, err := tx.Exec("INSERT INTO accounts (code, name, type) VALUES (?, ?, ?) ON CONFLICT(code) DO NOTHING", acc.Code, acc.Name, acc.Type)
		if err != nil {
			return fmt.Errorf("failed to insert account %s: %w", acc.Code, err)
		}
	}

	// 1b. Generera IB-verifikation om det finns IB-rader
	if len(ibRows) > 0 {
		// Obalanserad IB? Skicka mellanskillnaden till 2098
		if ibBalance != 0 {
			// Om ibBalance är positiv, har vi ett överskott av Debet -> Kreditera 2098
			// Om ibBalance är negativ, har vi underskott -> Debetera 2098
			row2098 := models.RowRequest{Account: "2098"}
			if ibBalance > 0 {
				row2098.Kredit = ibBalance
			} else {
				row2098.Debet = -ibBalance
			}
			ibRows = append(ibRows, row2098)
			
			// Se till att 2098 finns
			_, err := tx.Exec("INSERT INTO accounts (code, name, type) VALUES (?, ?, ?) ON CONFLICT(code) DO NOTHING", "2098", "Vinst eller förlust från föregående år", "Skuld")
			if err != nil {
				return fmt.Errorf("failed to create balancing account 2098: %w", err)
			}
		}

		ibVer := models.VerificationRequest{
			Date: startDate,
			Text: "Ingående Balans (Automatgenererad från SIE-import)",
			Type: "IB",
			Rows: ibRows,
		}

		_, _, err := l.postVerificationTx(tx, user, ibVer)
		if err != nil {
			return fmt.Errorf("misslyckades skapa IB-verifikation: %w", err)
		}
	}

	// 2. Posta alla verifikationer
	for i, v := range verificationsToImport {
		// Ignorera tomma verifikationer
		if len(v.Rows) == 0 {
			continue
		}

		// Enkel balanskontroll
		var sumDebet, sumKredit int64
		for _, r := range v.Rows {
			sumDebet += r.Debet
			sumKredit += r.Kredit
		}
		if sumDebet != sumKredit {
			return fmt.Errorf("verifikation #%d från filen balanserar inte (Debet: %d, Kredit: %d)", i+1, sumDebet, sumKredit)
		}

		// Sätt in transaktionen (bypassa API-säkerhetskopior genom att anropa tx direkt)
		_, _, err := l.postVerificationTx(tx, user, v)
		if err != nil {
			return fmt.Errorf("misslyckades importera verifikation #%d (%s): %w", i+1, v.Text, err)
		}
	}

	if err := l.logAuditTx(tx, user, "SIE-4 Import", fmt.Sprintf("Importerade %d verifikationer till %d", len(verificationsToImport), yearID)); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit import: %w", err)
	}
	success = true

	// Kör en WORM-sealing på hela den nyimporterade datan
	_, err = l.SealVerifications("System Auto-Seal (SIE-Import)", false)
	if err != nil {
		log.Printf("Warning: Failed to seal after import: %v", err)
	}

	return nil
}

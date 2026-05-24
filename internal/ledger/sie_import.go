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
	"unicode/utf8"

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

// decodeSIE4 detekterar och avkodar filens teckenkodning.
// Genom att bevisa att en CP437-kodad fil med svenska tecken aldrig är giltig UTF-8
// kan vi med 100% precision använda utf8.Valid för att särskilja filerna.
func decodeSIE4(fileData []byte) ([]byte, string) {
	hasNonASCII := false
	for _, b := range fileData {
		if b > 127 {
			hasNonASCII = true
			break
		}
	}

	if hasNonASCII && utf8.Valid(fileData) {
		return fileData, "UTF-8"
	}

	// Fallback to CP437
	decoder := charmap.CodePage437.NewDecoder()
	decoded, err := decoder.Bytes(fileData)
	if err != nil {
		return fileData, "UTF-8 (Fallback)"
	}
	return decoded, "CP437"
}

type parsedSIE struct {
	Program           string
	GenDate           string
	OrgNumber         string
	CompanyName       string
	FileRarStart      string
	FileRarEnd        string
	EncodingDetected  string
	Accounts          []models.Account
	IBRows            []models.RowRequest
	IBBalance         int64
	Verifications     []models.VerificationRequest
}

func (l *Ledger) parseSIE4(fileData []byte) (*parsedSIE, error) {
	decodedData, encoding := decodeSIE4(fileData)

	res := &parsedSIE{
		EncodingDetected: encoding,
		Accounts:         []models.Account{},
		IBRows:           []models.RowRequest{},
		Verifications:    []models.VerificationRequest{},
	}

	reader := bufio.NewReader(bytes.NewReader(decodedData))
	var currentVerReq *models.VerificationRequest

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
			return nil, fmt.Errorf("read error: %w", err)
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
		case "#PROGRAM":
			if len(parts) >= 2 {
				res.Program = stripQuotes(parts[1])
				if len(parts) >= 3 {
					res.Program = fmt.Sprintf("%s %s", res.Program, stripQuotes(parts[2]))
				}
			}
		case "#GEN":
			if len(parts) >= 2 {
				res.GenDate = stripQuotes(parts[1])
			}
		case "#FNAMN":
			if len(parts) >= 2 {
				res.CompanyName = stripQuotes(strings.Join(parts[1:], " "))
			}
		case "#ORGNR":
			if len(parts) >= 2 {
				res.OrgNumber = stripQuotes(parts[1])
			}
		case "#RAR":
			if len(parts) >= 4 && parts[1] == "0" {
				start := stripQuotes(parts[2])
				end := stripQuotes(parts[3])
				if len(start) == 8 {
					start = fmt.Sprintf("%s-%s-%s", start[0:4], start[4:6], start[6:8])
				}
				if len(end) == 8 {
					end = fmt.Sprintf("%s-%s-%s", end[0:4], end[4:6], end[6:8])
				}
				res.FileRarStart = start
				res.FileRarEnd = end
			}

		case "#KONTO":
			if len(parts) >= 3 {
				code := stripQuotes(parts[1])
				name := stripQuotes(strings.Join(parts[2:], " "))
				accType := "Okänd"
				if len(code) > 0 {
					switch code[0] {
					case '1': accType = "Tillgång"
					case '2': accType = "Skuld"
					case '3': accType = "Intäkt"
					case '4', '5', '6', '7', '8': accType = "Kostnad"
					}
				}
				res.Accounts = append(res.Accounts, models.Account{
					Code: code,
					Name: name,
					Type: accType,
				})
			}

		case "#IB":
			if len(parts) >= 4 {
				if parts[1] != "0" { // Endast innevarande år
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
					if amountOren > 0 {
						row.Debet = amountOren
					} else {
						row.Kredit = -amountOren
					}
					res.IBBalance += amountOren
					res.IBRows = append(res.IBRows, row)
				}
			}

		case "#VER":
			if currentVerReq != nil {
				res.Verifications = append(res.Verifications, *currentVerReq)
			}

			if len(parts) >= 4 {
				dateStr := stripQuotes(parts[3])
				if len(dateStr) == 8 {
					dateStr = fmt.Sprintf("%s-%s-%s", dateStr[0:4], dateStr[4:6], dateStr[6:8])
				}
				
				text := ""
				if len(parts) > 4 {
					text = stripQuotes(parts[4]) // Kirurgisk fix: Endast parts[4], inkludera inte regdatum/signatur
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

	if currentVerReq != nil {
		res.Verifications = append(res.Verifications, *currentVerReq)
	}

	return res, nil
}

type SIEPreview struct {
	Program             string   `json:"program"`
	GenDate             string   `json:"gen_date"`
	OrgNumber           string   `json:"org_number"`
	CompanyName         string   `json:"company_name"`
	EncodingDetected    string   `json:"encoding_detected"`
	FiscalYearMatch     bool     `json:"fiscal_year_match"`
	FileRarStart        string   `json:"file_rar_start"`
	FileRarEnd          string   `json:"file_rar_end"`
	SystemRarStart      string   `json:"system_rar_start"`
	SystemRarEnd        string   `json:"system_rar_end"`
	TotalVerifications   int      `json:"total_verifications"`
	TotalAccounts       int      `json:"total_accounts"`
	NewAccounts         []string `json:"new_accounts"`
	TotalDebetOre       int64    `json:"total_debet_ore"`
	TotalKreditOre      int64    `json:"total_kredit_ore"`
	IBEntries           int      `json:"ib_entries"`
	IsEmptyYear         bool     `json:"is_empty_year"`
	NoPeriodLocks       bool     `json:"no_period_locks"`
	Errors              []string `json:"errors"`
	Warnings            []string `json:"warnings"`
	IsValid             bool     `json:"is_valid"`
}

// PreviewSIE4 kör en dry-run validering och returnerar en rik, aggregerad status-JSON.
func (l *Ledger) PreviewSIE4(yearID int64, fileData []byte) (*SIEPreview, error) {
	// 1. Hämta systemets aktiva räkenskapsår
	var fy models.FiscalYear
	err := l.db.QueryRow("SELECT id, start_date, end_date FROM fiscal_years WHERE id = ?", yearID).Scan(&fy.ID, &fy.StartDate, &fy.EndDate)
	if err != nil {
		return nil, fmt.Errorf("fiscal year not found: %w", err)
	}

	// 2. Kontrollera om räkenskapsåret är tomt
	var count int
	err = l.db.QueryRow("SELECT COUNT(*) FROM verifications WHERE date >= ? AND date <= ?", fy.StartDate, fy.EndDate).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to check verification count: %w", err)
	}

	// 3. Kontrollera periodlås
	var lockCount int
	startMonth := fy.StartDate[:7]
	endMonth := fy.EndDate[:7]
	err = l.db.QueryRow("SELECT COUNT(*) FROM period_locks WHERE year_month >= ? AND year_month <= ?", startMonth, endMonth).Scan(&lockCount)
	if err != nil {
		return nil, fmt.Errorf("failed to check period locks: %w", err)
	}

	// 4. Hämta existerande kontoplan för att detektera nya konton
	existingAccounts := make(map[string]bool)
	rows, err := l.db.Query("SELECT code FROM accounts")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var code string
			if err := rows.Scan(&code); err == nil {
				existingAccounts[code] = true
			}
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("account iteration error: %w", err)
		}
	}

	// 5. Parse filen
	parsed, err := l.parseSIE4(fileData)
	if err != nil {
		return nil, err
	}

	// 6. Skapa förhandsgransknings-metadata
	preview := &SIEPreview{
		Program:            parsed.Program,
		GenDate:            parsed.GenDate,
		OrgNumber:          parsed.OrgNumber,
		CompanyName:        parsed.CompanyName,
		EncodingDetected:   parsed.EncodingDetected,
		FileRarStart:       parsed.FileRarStart,
		FileRarEnd:         parsed.FileRarEnd,
		SystemRarStart:     fy.StartDate[:10],
		SystemRarEnd:       fy.EndDate[:10],
		TotalVerifications: len(parsed.Verifications),
		TotalAccounts:      len(parsed.Accounts),
		IBEntries:          len(parsed.IBRows),
		IsEmptyYear:        count == 0,
		NoPeriodLocks:      lockCount == 0,
		Errors:             []string{},
		Warnings:           []string{},
		NewAccounts:        []string{},
	}

	// 7. Validera matchande räkenskapsår
	preview.FiscalYearMatch = (parsed.FileRarStart == preview.SystemRarStart && parsed.FileRarEnd == preview.SystemRarEnd)
	if !preview.FiscalYearMatch {
		preview.Errors = append(preview.Errors, fmt.Sprintf("Räkenskapsåret i filen (%s till %s) stämmer inte överens med systemets valda år (%s till %s).", 
			parsed.FileRarStart, parsed.FileRarEnd, preview.SystemRarStart, preview.SystemRarEnd))
	}

	if !preview.IsEmptyYear {
		preview.Errors = append(preview.Errors, fmt.Sprintf("Import är endast tillåtet på helt tomma räkenskapsår. Det finns redan %d verifikationer.", count))
	}

	if !preview.NoPeriodLocks {
		preview.Errors = append(preview.Errors, fmt.Sprintf("Import blockerad: det finns %d låsta perioder i det valda räkenskapsåret. Lås upp perioderna först.", lockCount))
	}

	// 8. Validera verifikationer och ackumulera summor
	var totalDebet int64
	var totalKredit int64
	for i, v := range parsed.Verifications {
		if len(v.Rows) == 0 {
			continue
		}

		var sumDebet int64
		var sumKredit int64
		for _, r := range v.Rows {
			sumDebet += r.Debet
			sumKredit += r.Kredit
			totalDebet += r.Debet
			totalKredit += r.Kredit

			// Registrera nya konton som saknas i systemet
			if !existingAccounts[r.Account] {
				alreadyAdded := false
				for _, a := range preview.NewAccounts {
					if a == r.Account {
						alreadyAdded = true
						break
					}
				}
				if !alreadyAdded {
					preview.NewAccounts = append(preview.NewAccounts, r.Account)
				}
			}
		}

		// Balanskontroll för varje enskild verifikation
		if sumDebet != sumKredit {
			preview.Errors = append(preview.Errors, fmt.Sprintf("Verifikation #%d (%s) balanserar inte (Debet: %.2f, Kredit: %.2f).", 
				i+1, v.Text, float64(sumDebet)/100.0, float64(sumKredit)/100.0))
		}

		// Datumkontroll (måste ligga inom räkenskapsåret)
		if v.Date < preview.SystemRarStart || v.Date > preview.SystemRarEnd {
			preview.Errors = append(preview.Errors, fmt.Sprintf("Verifikation #%d (%s) har datum %s vilket ligger utanför det valda räkenskapsåret.", 
				i+1, v.Text, v.Date))
		}
	}

	preview.TotalDebetOre = totalDebet
	preview.TotalKreditOre = totalKredit

	if len(preview.NewAccounts) > 0 {
		preview.Warnings = append(preview.Warnings, fmt.Sprintf("%d nya konton kommer att skapas i din kontoplan: %s", 
			len(preview.NewAccounts), strings.Join(preview.NewAccounts, ", ")))
	}

	preview.IsValid = (len(preview.Errors) == 0)

	return preview, nil
}

// ImportSIE4 parsear och importerar en SIE-4 fil.
// Den utförs helt atomärt och skyddas av samma strikta validering som dry-run.
func (l *Ledger) ImportSIE4(user string, yearID int64, fileData []byte) error {
	// 1. Kör fullständig förhandsgranskningsvalidering först för att säkra mot felaktig data
	preview, err := l.PreviewSIE4(yearID, fileData)
	if err != nil {
		return err
	}

	if !preview.IsValid {
		return fmt.Errorf("importen avvisades på grund av valideringsfel: %s", strings.Join(preview.Errors, "; "))
	}

	// 2. Parse filen på nytt för insättning
	parsed, err := l.parseSIE4(fileData)
	if err != nil {
		return err
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

	// 3. Skapa konton som inte finns i kontoplanen
	for _, acc := range parsed.Accounts {
		_, err := tx.Exec("INSERT INTO accounts (code, name, type) VALUES (?, ?, ?) ON CONFLICT(code) DO NOTHING", acc.Code, acc.Name, acc.Type)
		if err != nil {
			return fmt.Errorf("failed to insert account %s: %w", acc.Code, err)
		}
	}

	// Dubbelkolla att konton i IB/TRANS också finns
	ensureAccount := func(code string) error {
		accType := "Okänd"
		if len(code) > 0 {
			switch code[0] {
			case '1': accType = "Tillgång"
			case '2': accType = "Skuld"
			case '3': accType = "Intäkt"
			case '4', '5', '6', '7', '8': accType = "Kostnad"
			}
		}
		_, err := tx.Exec("INSERT INTO accounts (code, name, type) VALUES (?, ?, ?) ON CONFLICT(code) DO NOTHING", code, "Importerat konto "+code, accType)
		return err
	}

	for _, row := range parsed.IBRows {
		if err := ensureAccount(row.Account); err != nil {
			return fmt.Errorf("failed to verify IB account %s: %w", row.Account, err)
		}
	}

	for _, v := range parsed.Verifications {
		for _, row := range v.Rows {
			if err := ensureAccount(row.Account); err != nil {
				return fmt.Errorf("failed to verify account %s: %w", row.Account, err)
			}
		}
	}

	// 4. Generera IB-verifikation om det finns IB-rader
	if len(parsed.IBRows) > 0 {
		// Obalanserad IB? Skicka mellanskillnaden till 2098
		if parsed.IBBalance != 0 {
			row2098 := models.RowRequest{Account: "2098"}
			if parsed.IBBalance > 0 {
				row2098.Kredit = parsed.IBBalance
			} else {
				row2098.Debet = -parsed.IBBalance
			}
			parsed.IBRows = append(parsed.IBRows, row2098)
			
			// Se till att 2098 finns
			_, err := tx.Exec("INSERT INTO accounts (code, name, type) VALUES (?, ?, ?) ON CONFLICT(code) DO NOTHING", "2098", "Vinst eller förlust från föregående år", "Skuld")
			if err != nil {
				return fmt.Errorf("failed to create balancing account 2098: %w", err)
			}
		}

		ibVer := models.VerificationRequest{
			Date: preview.SystemRarStart,
			Text: "Ingående Balans (Automatgenererad från SIE-import)",
			Type: "IB",
			Rows: parsed.IBRows,
		}

		_, _, err := l.postVerificationTx(tx, user, ibVer)
		if err != nil {
			return fmt.Errorf("misslyckades skapa IB-verifikation: %w", err)
		}
	}

	// 5. Posta alla verifikationer
	for i, v := range parsed.Verifications {
		if len(v.Rows) == 0 {
			continue
		}

		_, _, err := l.postVerificationTx(tx, user, v)
		if err != nil {
			return fmt.Errorf("misslyckades importera verifikation #%d (%s): %w", i+1, v.Text, err)
		}
	}

	if err := l.logAuditTx(tx, user, "SIE-4 Import", fmt.Sprintf("Importerade %d verifikationer till %d", len(parsed.Verifications), yearID)); err != nil {
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
